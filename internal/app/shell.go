package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/catalog"
	"github.com/DevCoreXOfficial/termux-ui/internal/core"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

type shellScreen struct {
	app       *App
	menu      *ui.Menu
	chshRadio ui.Radio
	stackRows []core.Tool
	stackErr  string
	p10kFound bool
	confirm   *ui.Confirm
	pending   func(a *App) tea.Cmd
}

func newShellScreen(a *App) *shellScreen {
	s := &shellScreen{app: a}
	s.chshRadio = ui.Radio{Options: []string{"bash", "zsh"}}
	cur := filepath.Base(os.Getenv("SHELL"))
	if cur == "zsh" {
		s.chshRadio.Index = 1
	}
	items := []ui.MenuItem{
		{Label: "Install ZSH stack", Desc: "core install shell"},
		{Label: "Update plugins", Desc: "core update shell"},
		{Label: "Reinstall stack", Desc: "core reinstall shell (confirm)"},
		{Label: "Switch default shell", Desc: "chsh -s …"},
		{Label: "Reconfigure prompt", Desc: "p10k configure (needs powerlevel10k)"},
	}
	s.menu = ui.NewMenu(items)
	return s
}

func (s *shellScreen) Title() string         { return "Shell Setup" }
func (s *shellScreen) WantsGlobalKeys() bool { return s.confirm == nil }

func (s *shellScreen) prefixBin(shell string) (string, bool) {
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		return "", false
	}
	path := filepath.Join(prefix, "bin", shell)
	st, err := os.Stat(path)
	return path, err == nil && !st.IsDir()
}

func (s *shellScreen) p10kStatus() (bool, string) {
	if _, err := exec.LookPath("p10k"); err == nil {
		return true, "command p10k found"
	}
	for _, hint := range []string{"~/.p10k.zsh", "${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/themes/powerlevel10k"} {
		p := strings.ReplaceAll(hint, "~", homeDir)
		if _, err := os.Stat(p); err == nil {
			return true, p
		}
	}
	return false, ""
}

func (s *shellScreen) Update(a *App, msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case refreshMsg:
		if a.core.Installed() {
			rows, err := a.core.List("shell")
			if err != nil {
				s.stackErr = err.Error()
				s.stackRows = nil
			} else {
				s.stackErr = ""
				s.stackRows = rows
			}
		} else {
			s.stackErr = "core not installed"
		}
		s.p10kFound, _ = s.p10kStatus()
		return nil

	case tea.KeyMsg:
		key := t.String()
		if s.confirm != nil {
			s.confirm.Update(key)
			if s.confirm.Result == 1 && s.pending != nil {
				fn := s.pending
				s.confirm, s.pending = nil, nil
				return fn(a)
			}
			if s.confirm.Result != 0 {
				s.confirm, s.pending = nil, nil
			}
			return nil
		}
		switch key {
		case "up", "k":
			s.menu.Up()
		case "down", "j":
			s.menu.Down()
		case "left":
			if s.menu.Cursor == 3 {
				s.chshRadio.Left()
			}
		case "right":
			if s.menu.Cursor == 3 {
				s.chshRadio.Right()
			}
		case "enter":
			switch s.menu.Cursor {
			case 0:
				return a.runCoreOp(core.VerbInstall, "shell", nil)
			case 1:
				return a.runCoreOp(core.VerbUpdate, "shell", nil)
			case 2:
				s.confirm = ui.NewConfirm("Reinstall the whole ZSH stack?",
					"Runs: core reinstall shell — uninstall + install.", "y confirm · n cancel")
				s.pending = func(m *App) tea.Cmd { return m.runCoreOp(core.VerbReinstall, "shell", nil) }
			case 3:
				target := s.chshRadio.Value()
				path, ok := s.prefixBin(target)
				if !ok {
					a.ToastError(target + " not found in $PREFIX/bin — install it first")
					return nil
				}
				target = filepath.Base(path)
				s.confirm = ui.NewConfirm(
					fmt.Sprintf("Make %s the default shell?", target),
					"Runs: chsh -s "+target+" (applies on next Termux restart).",
					"y confirm · n cancel")
				tgt := target
				s.pending = func(m *App) tea.Cmd {
					cmd := exec.Command("chsh", "-s", tgt)
					return m.RunChild("chsh -s "+tgt, cmd, func(m *App, err error) tea.Cmd {
						m.ToastInfo("restart Termux for the new shell to take effect")
						return nil
					})
				}
			case 4:
				found, where := s.p10kStatus()
				if !found {
					a.ToastWarn("powerlevel10k not detected — install the stack first")
					return nil
				}
				cmd := exec.Command("zsh", "-ic", "p10k configure")
				return a.RunChild("p10k configure ("+where+")", cmd, nil)
			}
		}
	}
	return nil
}

func (s *shellScreen) View() string {
	w := clampW(s.app.width, 40)

	current := filepath.Base(os.Getenv("SHELL"))
	if current == "" || current == "." {
		current = "unknown"
	}

	stack := ui.SubtleStyle.Render("…")
	if s.stackErr != "" {
		stack = ui.StatusText("unknown") + " " + ui.WarnStyle.Render("("+s.stackErr+")")
	} else if len(s.stackRows) > 0 {
		inst := 0
		p10kRow := core.StatusUnknown
		for _, r := range s.stackRows {
			if r.Status == core.StatusInstalled {
				inst++
			}
			if strings.Contains(strings.ToLower(r.Name), "powerlevel10k") {
				p10kRow = r.Status
			}
		}
		stack = fmt.Sprintf("%d/%d components installed; powerlevel10k: ", inst, len(s.stackRows)) + ui.StatusText(p10kRow.String())
	}

	var b strings.Builder
	b.WriteString("Current shell:    " + current + "\n")
	b.WriteString("ZSH stack:        " + stack + "\n")
	if found, where := s.p10kStatus(); found {
		b.WriteString("p10k detected:    " + ui.GoodStyle.Render(where) + "\n")
	}
	b.WriteString("\n" + ui.TitleStyle.Render("Plugin roster (from core's shell module)") + "\n")
	b.WriteString(ui.SubtleStyle.Render(strings.Join(catalog.ShellPlugins, " · ")) + "\n\n")

	s.menu.Items[3].Desc = "→ " + s.chshRadio.Value()
	b.WriteString(s.menu.View(w))
	b.WriteString("\n" + ui.SubtleStyle.Render("←/→ pick shell for chsh · installs record a restart-required note"))

	if s.confirm != nil {
		b.WriteString("\n" + s.confirm.View(clampI(w-4, 30, 140)) + "\n")
	}
	return b.String()
}
