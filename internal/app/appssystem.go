package app

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/catalog"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

// actionRow is one selectable line on the Apps & System screen.
type actionRow struct {
	label string
	desc  string
	run   func(a *App) tea.Cmd
}

type appsSystemScreen struct {
	app                 *App
	rows                []actionRow
	cursor              int
	pager               *ui.Pager
	confirm             *ui.Confirm
	pending             func(a *App) tea.Cmd
	passwdOpen          bool
	passwd              ui.TextInput
	bootNameOpen        bool
	bootCreateName      ui.TextInput
	bootTemplate        string
	bootTemplateContent string
}

func newAppsSystemScreen(a *App) *appsSystemScreen {
	s := &appsSystemScreen{app: a}
	return s
}

func (s *appsSystemScreen) Title() string { return "Apps & System" }
func (s *appsSystemScreen) WantsGlobalKeys() bool {
	return s.confirm == nil && s.pager == nil && !s.passwdOpen
}

func homePath(rel string) string { return filepath.Join(homeDir, rel) }

func (s *appsSystemScreen) rebuild() {
	rows := []actionRow{}

	rows = append(rows, actionRow{
		label: storageLabel(),
		desc:  "runs termux-setup-storage (accept the Android dialog, then press Enter)",
		run: func(m *App) tea.Cmd {
			cmd := exec.Command("termux-setup-storage")
			return m.RunChild("termux-setup-storage", cmd, func(m *App, err error) tea.Cmd {
				if err == nil && storageGranted() {
					m.ToastGood("storage granted")
				} else if err == nil {
					m.ToastInfo("if the dialog was declined, run again")
				}
				return m.current().Update(m, refreshMsg{})
			})
		},
	})

	rows = append(rows, s.companionRows()...)
	rows = append(rows, s.sshRows()...)
	rows = append(rows, s.bootRows()...)

	rows = append(rows,
		actionRow{
			label: "Change package mirrors (interactive)",
			desc:  "termux-change-repo",
			run: func(m *App) tea.Cmd {
				return m.RunChild("termux-change-repo", exec.Command("termux-change-repo"), nil)
			},
		},
		actionRow{
			label: "Upgrade all packages",
			desc:  "pkg upgrade -y (confirm first)",
			run: func(m *App) tea.Cmd {
				if sc, ok := m.current().(*appsSystemScreen); ok {
					sc.confirm = ui.NewConfirm("Run pkg upgrade -y?",
						"Upgrades every installed package. This can take a while.", "y confirm · n cancel")
					sc.pending = func(mm *App) tea.Cmd {
						return mm.RunChild("pkg upgrade -y", exec.Command("pkg", "upgrade", "-y"),
							func(m *App, err error) tea.Cmd { return nil })
					}
				}
				return nil
			},
		},
		actionRow{
			label: "System info",
			desc:  "termux-info in pager",
			run:   func(m *App) tea.Cmd { return captureCmd("termux-info", exec.Command("termux-info")) },
		},
	)
	s.rows = rows
}

func storageLabel() string {
	if storageGranted() {
		return "S1 Storage — granted (run setup again)"
	}
	return "S1 Storage — NOT granted (set it up)"
}

func pkgInstalled(name string) bool {
	err := exec.Command("dpkg", "-s", name).Run()
	return err == nil
}

func (s *appsSystemScreen) companionRows() []actionRow {
	var rows []actionRow
	for _, app := range catalog.CompanionApps {
		name := app.Name
		url := app.FDroidURL
		cli := app.CliPackage
		opt := ""
		if app.Optional {
			opt = " [optional]"
		}
		status := "CLI not installed"
		if pkgInstalled(cli) {
			status = "CLI installed"
		}
		rows = append(rows, actionRow{
			label: fmt.Sprintf("%s%s — %s", name, opt, status),
			desc:  "open F-Droid page",
			run: func(m *App) tea.Cmd {
				return m.RunChild("open "+url, exec.Command("termux-open-url", url), nil)
			},
		})
		switch name {
		case "Termux:API":
			rows = append(rows, actionRow{
				label: "Termux:API self-test",
				desc:  "termux-battery-status output shown here",
				run: func(m *App) tea.Cmd {
					return captureCmd("termux-battery-status",
						exec.Command("sh", "-c", "termux-api-start; termux-battery-status"))
				},
			})
		case "Termux:Boot":
			rows = append(rows, actionRow{
				label: bootStatus(),
				desc:  "open the Termux:Boot app once after install to enable it",
				run: func(m *App) tea.Cmd {
					os.MkdirAll(homePath(".termux/boot"), 0o700)
					m.ToastGood("~/.termux/boot ready")
					return m.current().Update(m, refreshMsg{})
				},
			})
		case "Termux:Widget":
			rows = append(rows, actionRow{
				label: shortcutsStatus(),
				desc:  "creates ~/.shortcuts if missing",
				run: func(m *App) tea.Cmd {
					os.MkdirAll(homePath(".shortcuts"), 0o700)
					m.ToastGood("~/.shortcuts ready")
					return m.current().Update(m, refreshMsg{})
				},
			})
		}
	}
	rows = append(rows, actionRow{
		label: "",
		desc:  "",
		run:   nil,
	})
	rows[len(rows)-1] = actionRow{
		label: ui.WarnStyle.Render("Note: companion apps must come from F-Droid (Play Store builds are incompatible)"),
		desc:  "",
		run:   nil,
	}
	return rows
}

func bootStatus() string {
	if st, err := os.Stat(homePath(".termux/boot")); err == nil && st.IsDir() {
		n := countScripts(homePath(".termux/boot"))
		return fmt.Sprintf("S4 Boot scripts — %d script(s)", n)
	}
	return "S4 Boot scripts — ~/.termux/boot missing (create it)"
}

func shortcutsStatus() string {
	if st, err := os.Stat(homePath(".shortcuts")); err == nil && st.IsDir() {
		return "Widget shortcuts — ~/.shortcuts present"
	}
	return "Widget shortcuts — ~/.shortcuts missing (create it)"
}

func countScripts(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sh") {
			n++
		}
	}
	return n
}

func sshdRunning() bool {
	err := exec.Command("pgrep", "-x", "sshd").Run()
	return err == nil
}

var ipRe = regexp.MustCompile(`inet (?:addr:)?(\d+\.\d+\.\d+\.\d+)`)

func phoneIP() string {
	for _, cmd := range [][]string{{"ip", "-4", "addr"}, {"ifconfig"}} {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			continue
		}
		for _, m := range ipRe.FindAllStringSubmatch(string(out), -1) {
			ip := m[1]
			if !strings.HasPrefix(ip, "127.") && !strings.HasPrefix(ip, "172.16") && !net.ParseIP(ip).IsLoopback() {
				return ip
			}
		}
	}
	return "<phone-ip>"
}

func sshPort() string {
	data, err := os.ReadFile(filepath.Join(os.Getenv("PREFIX"), "etc/ssh/sshd_config"))
	if err == nil {
		re := regexp.MustCompile(`(?m)^\s*Port\s+(\d+)`)
		if m := re.FindSubmatch(data); m != nil {
			return string(m[1])
		}
	}
	return "8022"
}

func (s *appsSystemScreen) sshRows() []actionRow {
	port := sshPort()
	ip := phoneIP()
	state := "stopped"
	styleTxt := ui.StatusText("not installed")
	styleTxt = ui.StatusText(pkgInstalledLabel())
	if sshdRunning() {
		state = ui.GoodStyle.Render("running")
	} else {
		state = ui.SubtleStyle.Render("stopped")
	}

	return []actionRow{
		{label: "S3 SSH wizard — " + styleTxt + ", server " + state,
			desc: "enter cycles through the steps below", run: nil},
		{label: "  3.1 Install openssh", desc: "pkg install openssh -y", run: func(m *App) tea.Cmd {
			return m.RunChild("pkg install openssh", exec.Command("pkg", "install", "openssh", "-y"),
				func(m *App, err error) tea.Cmd { return m.current().Update(m, refreshMsg{}) })
		}},
		{label: "  3.2 Set password", desc: "masked input → passwd", run: func(m *App) tea.Cmd {
			if sc, ok := m.current().(*appsSystemScreen); ok {
				sc.passwdOpen = true
				sc.passwd = ui.TextInput{Masked: true}
				m.ToastWarn("Type the password, then enter — retype it when passwd asks")
			}
			return nil
		}},
		{label: "  3.3 Start sshd", desc: "sshd", run: func(m *App) tea.Cmd {
			return m.RunChild("sshd", exec.Command("sshd"),
				func(m *App, err error) tea.Cmd { return m.current().Update(m, refreshMsg{}) })
		}},
		{label: "  3.4 Connect from a remote host", desc: "ssh -p " + port + " <user>@" + ip + " — sshd dies on reboot; add a boot script below", run: captureLoginInfo(port, ip)},
		{label: "  3.5 Stop sshd", desc: "pkill sshd", run: func(m *App) tea.Cmd {
			return m.RunChild("pkill sshd", exec.Command("pkill", "sshd"),
				func(m *App, err error) tea.Cmd { return m.current().Update(m, refreshMsg{}) })
		}},
	}
}

func pkgInstalledLabel() string {
	if pkgInstalled("openssh") {
		return "openssh ok"
	}
	return "openssh missing"
}

func (s *appsSystemScreen) bootRows() []actionRow {
	dir := homePath(".termux/boot")
	bootInstalled := pkgInstalled("termux-boot")
	hint := ""
	if !bootInstalled {
		hint = " (install Termux:Boot above)"
	}
	rows := []actionRow{{
		label: "S4 Boot scripts — " + bootStatus() + hint,
		desc:  "scripts in ~/.termux/boot/*.sh run at boot; keep them short",
		run:   nil,
	}}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		name := e.Name()
		full := filepath.Join(dir, name)
		execBit := false
		if info, err := e.Info(); err == nil && info.Mode()&0o111 != 0 {
			execBit = true
		}
		bit := ui.BadStyle.Render("no-exec")
		if execBit {
			bit = ui.GoodStyle.Render("exec")
		}
		rows = append(rows,
			actionRow{label: "      edit " + name + " (" + ui.Strip(bit) + ")", desc: "$EDITOR", run: func(m *App) tea.Cmd {
				return m.OpenInEditor(full)
			}},
			actionRow{label: "      delete " + name, desc: "confirm", run: func(m *App) tea.Cmd {
				if sc, ok := m.current().(*appsSystemScreen); ok {
					sc.confirm = ui.NewConfirm("Delete boot script "+name+"?",
						"The file is removed immediately.", "y confirm · n cancel")
					sc.pending = func(mm *App) tea.Cmd {
						return func() tea.Msg {
							err := os.Remove(full)
							return childDone{title: "delete " + name, err: err,
								after: func(m *App, e error) tea.Cmd { return m.current().Update(m, refreshMsg{}) }}
						}
					}
				}
				return nil
			}},
		)
	}

	for _, tpl := range catalog.BootTemplates {
		tname := tpl.Name
		content := tpl.Content
		rows = append(rows, actionRow{
			label: "  new boot script from template: " + tname,
			desc:  "prompts for a name, saves chmod +x",
			run: func(m *App) tea.Cmd {
				if sc, ok := m.current().(*appsSystemScreen); ok {
					sc.bootCreateName.Set("")
					sc.bootTemplate = tname
					sc.bootTemplateContent = content
					sc.bootNameOpen = true
				}
				return nil
			},
		})
	}
	return rows
}

func captureLoginInfo(port, ip string) func(*App) tea.Cmd {
	return func(m *App) tea.Cmd {
		login := fmt.Sprintf("ssh -p %s <user>@%s", port, ip)
		user := os.Getenv("USER")
		return captureCmd("SSH login",
			exec.Command("sh", "-c", fmt.Sprintf(
				`echo "From any device on the same network:"; echo; echo "    %s"; echo; echo "Default user: %s"; echo "Warning: sshd does not survive reboots — add the sshd boot template in S4."`,
				login, user)))
	}
}
