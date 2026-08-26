package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/catalog"
	"github.com/DevCoreXOfficial/termux-ui/internal/core"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

const repoAPI = "https://api.github.com/repos/DevCoreXOfficial/termux-ui/releases/latest"

// updateMsg carries the result of a background update check.
type updateMsg struct {
	latest string
	err    error
}

func checkUpdateCmd(state *State) tea.Cmd {
	if time.Since(state.LastUpdateCheck) < 24*time.Hour && state.LatestVersion != "" {
		latest := state.LatestVersion
		return func() tea.Msg { return updateMsg{latest: latest} }
	}
	return func() tea.Msg {
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(repoAPI)
		if err != nil {
			return updateMsg{err: err}
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return updateMsg{err: err}
		}
		if resp.StatusCode != http.StatusOK {
			return updateMsg{err: fmt.Errorf("HTTP %d", resp.StatusCode)}
		}
		var payload struct {
			TagName string `json:"tag_name"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return updateMsg{err: err}
		}
		return updateMsg{latest: strings.TrimPrefix(payload.TagName, "v")}
	}
}

// selfUpdateCmd downloads the matching release binary and swaps it in.
func selfUpdateCmd(latest string) tea.Cmd {
	arch := runtime.GOARCH
	asset := fmt.Sprintf("https://github.com/DevCoreXOfficial/termux-ui/releases/latest/download/termux-ui_linux_%s", arch)
	return func() tea.Msg {
		prefix := os.Getenv("PREFIX")
		if prefix == "" {
			return childDone{title: "self-update", err: fmt.Errorf("$PREFIX not set")}
		}
		target := filepath.Join(prefix, "bin", "termux-ui")
		tmp := target + ".download"
		client := &http.Client{Timeout: 10 * time.Minute}
		resp, err := client.Get(asset)
		if err != nil {
			return childDone{title: "self-update", err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return childDone{title: "self-update", err: fmt.Errorf("download failed: HTTP %d", resp.StatusCode)}
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return childDone{title: "self-update", err: err}
		}
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			return childDone{title: "self-update", err: err}
		}
		if err := os.Rename(tmp, target); err != nil {
			_ = os.Remove(tmp) // ignore cleanup error on failure path
			return childDone{title: "self-update", err: err}
		}
		return childDone{title: "self-update → " + latest, err: nil, after: func(m *App, err error) tea.Cmd {
			m.ToastWarn("Restart termux-ui to run the new version")
			return nil
		}}
	}
}

// storageGranted reports whether ~/storage exists.
func storageGranted() bool {
	home, _ := os.UserHomeDir()
	st, err := os.Stat(filepath.Join(home, "storage"))
	return err == nil && st.IsDir()
}

type dashboard struct {
	app       *App
	menu      *ui.Menu
	update    updateMsg
	checking  bool
	confirm   *ui.Confirm
	pendingFn func(*App) tea.Cmd
}

func newDashboard(a *App) *dashboard {
	d := &dashboard{app: a}
	d.rebuildMenu()
	return d
}

func (d *dashboard) rebuildMenu() {
	items := []ui.MenuItem{
		{Label: "Modules & Packages", Desc: "core front-end: lang db ai editor dev npm shell ui auto"},
		{Label: "Termux Settings", Desc: "every termux.properties property"},
		{Label: "Extra Keys Builder", Desc: "rows, popups, macros"},
		{Label: "Themes & Fonts", Desc: "colorschemes + Nerd Fonts"},
		{Label: "Shell Setup", Desc: "ZSH stack, chsh, p10k configure"},
		{Label: "Apps & System", Desc: "storage, companions, SSH, boot, maintenance"},
	}
	d.menu = ui.NewMenu(items)
}

func (d *dashboard) Title() string         { return "Dashboard" }
func (d *dashboard) WantsGlobalKeys() bool { return d.confirm == nil }

func (d *dashboard) Update(a *App, msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case refreshMsg:
		d.checking = true
		return checkUpdateCmd(a.state)

	case updateMsg:
		d.checking = false
		d.update = t
		if t.err == nil {
			a.state.LastUpdateCheck = time.Now()
			a.state.LatestVersion = t.latest
			a.SaveState()
		}
		return nil

	case tea.KeyMsg:
		if d.confirm != nil {
			d.confirm.Update(t.String())
			if d.confirm.Result == 1 && d.pendingFn != nil {
				fn := d.pendingFn
				d.confirm, d.pendingFn = nil, nil
				return fn(a)
			}
			if d.confirm.Result != 0 {
				d.confirm, d.pendingFn = nil, nil
			}
			return nil
		}
		switch t.String() {
		case "up", "k":
			d.menu.Up()
		case "down", "j":
			d.menu.Down()
		case "enter":
			keys := []string{"2", "3", "4", "5", "6", "7"}
			if d.menu.Cursor < len(keys) {
				return a.jumpTo(keys[d.menu.Cursor])
			}
		case "i":
			if !a.core.Installed() {
				return a.RunChild("install core",
					exec.Command("sh", "-c", catalog.CoreInstallerOneLiner),
					func(m *App, err error) tea.Cmd {
						m.core = core.Detect()
						if m.core.Installed() {
							m.ToastGood("core " + m.core.Version + " detected")
						}
						return nil
					})
			}
			return a.RunChild("update core", a.core.Cmd(core.VerbUpdate, "core", nil),
				func(m *App, err error) tea.Cmd {
					m.core = core.Detect()
					return nil
				})
		case "s":
			if storageGranted() {
				a.ToastInfo("storage already granted")
				return nil
			}
			return a.RunChild("termux-setup-storage", exec.Command("termux-setup-storage"), nil)
		case "u":
			if d.update.err != nil || d.update.latest == "" {
				a.ToastInfo("no update information available yet")
				return nil
			}
			if core.ParseVersion(Version, d.update.latest) >= 0 {
				a.ToastGood("already up to date (" + Version + ")")
				return nil
			}
			d.confirm = ui.NewConfirm(
				fmt.Sprintf("Download %s and replace %s?", d.update.latest, Version),
				"The new binary is written to $PREFIX/bin/termux-ui. Restart termux-ui afterwards.",
				"y confirm · n cancel",
			)
			d.pendingFn = func(m *App) tea.Cmd { return selfUpdateCmd(d.update.latest) }
		}
	}
	return nil
}

func (d *dashboard) View() string {
	w := clampW(d.app.width, 40)

	coreStatus := ui.BadStyle.Render("not installed — press i to install")
	if d.app.core.Installed() {
		coreStatus = ui.GoodStyle.Render("installed "+d.app.core.Version) + ui.SubtleStyle.Render("  "+d.app.core.Path)
	}

	selfStatus := Version
	if d.update.err != nil {
		selfStatus += "   " + ui.SubtleStyle.Render("(update check failed: "+d.update.err.Error()+")")
	} else if d.checking {
		selfStatus += "   " + ui.SubtleStyle.Render("checking…")
	} else if d.update.latest != "" && core.ParseVersion(Version, d.update.latest) < 0 {
		selfStatus += "   " + ui.WarnStyle.Render(fmt.Sprintf("Update available: %s (current %s) — press u", d.update.latest, Version))
	}

	storeStatus := ui.BadStyle.Render("not granted — press s")
	if storageGranted() {
		storeStatus = ui.GoodStyle.Render("granted")
	}

	restart := ui.SubtleStyle.Render("none")
	if list := d.app.state.RestartList(); len(list) > 0 {
		restart = ui.WarnStyle.Render("Restart Termux (force-stop from Android settings) to apply: " + strings.Join(list, ", "))
	}

	line := func(k, v string) string {
		return fmt.Sprintf(" %-16s %s", ui.SubtleStyle.Render(k+":"), v)
	}

	var b strings.Builder
	b.WriteString(line("Termux version", os.Getenv("TERMUX_VERSION")) + "\n\n")
	b.WriteString(line("core", coreStatus) + "\n\n")
	b.WriteString(line("termux-ui", selfStatus) + "\n\n")
	b.WriteString(line("Storage permission", storeStatus) + "\n\n")
	b.WriteString(line("Pending restarts", restart) + "\n\n")
	if !d.app.core.Installed() {
		b.WriteString(" " + ui.SubtleStyle.Render("Install core first: "+catalog.CoreInstallerOneLiner) + "\n\n")
	}
	b.WriteString(" " + ui.TitleStyle.Render("Quick actions") + "\n")
	b.WriteString(d.menu.View(w))
	if d.confirm != nil {
		b.WriteString("\n" + d.confirm.View(w) + "\n")
	}
	return b.String()
}
