package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

func (s *appsSystemScreen) Update(a *App, msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case refreshMsg:
		s.rebuild()
		return nil

	case textMsg:
		s.pager = ui.NewPager(t.title, t.content)
		s.pager.Height = clampI(a.height-8, 6, 40)
		return nil

	case tea.KeyMsg:
		key := t.String()

		if s.pager != nil {
			switch key {
			case "up", "k":
				s.pager.Up()
			case "down", "j":
				s.pager.Down()
			case "pgup":
				s.pager.Up()
			case "pgdn":
				s.pager.Down()
			case "esc", "q", "enter":
				s.pager = nil
			}
			return nil
		}

		if s.bootNameOpen {
			switch key {
			case "esc":
				s.bootNameOpen = false
			case "enter":
				name := strings.TrimSpace(s.bootCreateName.String())
				if name == "" {
					s.bootCreateName.Err = "name required"
					return nil
				}
				if !strings.HasSuffix(name, ".sh") {
					name += ".sh"
				}
				if name == "blank.sh" && s.bootTemplate == "blank" {
				}
				path := filepath.Join(homePath(".termux/boot"), name)
				s.bootNameOpen = false
				if _, err := os.Stat(path); err == nil {
					a.ToastError("already exists: " + path)
					return nil
				}
				content := s.bootTemplateContent
				tpl := s.bootTemplate
				if tpl == "blank" {
					a.OpenInEditor(path)
					content = "#!/data/data/com.termux/files/usr/bin/sh\n# New boot script\n"
				}
				return func() tea.Msg {
					os.MkdirAll(filepath.Dir(path), 0o700)
					err := os.WriteFile(path, []byte(content), 0o700)
					return childDone{title: "create " + filepath.Base(path), err: err,
						after: func(m *App, e error) tea.Cmd { return m.current().Update(m, refreshMsg{}) }}
				}
			default:
				typeInto(&s.bootCreateName, key)
				s.bootCreateName.Err = ""
			}
			return nil
		}

		if s.passwdOpen {
			switch key {
			case "esc":
				s.passwdOpen = false
				return nil
			case "enter":
				pw := s.passwd.String()
				s.passwdOpen = false
				if len(pw) < 4 {
					a.ToastError("password too short")
					return nil
				}
				cmd := exec.Command("sh", "-c", `stty -echo; printf 'New password: '; read -r p1; printf '\nRetype: '; read -r p2; [ "$p1" = "$p2" ] && printf '%s\n%s\n' "$p1" "$p1" | passwd || { echo mismatch; exit 1; }`)
				cmd.Env = append(os.Environ(), "TUI_SUGGESTED_PW="+pw)
				return a.RunChild("passwd", cmd, func(m *App, err error) tea.Cmd { return m.current().Update(m, refreshMsg{}) })
			default:
				typeInto(&s.passwd, key)
			}
			return nil
		}

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
			if s.cursor > 0 {
				s.cursor--
			} else if len(s.rows) == 0 {
				s.rebuild()
			}
		case "down", "j":
			if s.cursor < len(s.rows)-1 {
				s.cursor++
			} else if len(s.rows) == 0 {
				s.rebuild()
			}
		case "enter", " ", "space":
			if s.cursor < len(s.rows) && s.rows[s.cursor].run != nil {
				return s.rows[s.cursor].run(a)
			}
		}
	}
	return nil
}

func (s *appsSystemScreen) View() string {
	w := clampW(s.app.width, 40)
	h := clampI(s.app.height-8, 6, 60)
	if len(s.rows) == 0 {
		s.rebuild()
	}

	start := clampI(s.cursor-h+1, 0, maxI(0, len(s.rows)-h))
	var b strings.Builder
	for i, row := range s.rows[start:] {
		idx := start + i
		line := row.label
		desc := row.desc
		if desc != "" {
			line += "   " + ui.SubtleStyle.Render(desc)
		}
		if idx == s.cursor {
			line = ui.SelStyle.Render(ui.Strip(line))
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + ui.SubtleStyle.Render("enter run · scripts and wizards stream output live"))

	out := b.String()
	if s.passwdOpen {
		box := "Set SSH password (masked):\n" + s.passwd.View(true) + "\n\nenter → passwd prompts · esc cancel"
		out = overlay(out, ui.BoxStyle.Width(clampI(w-8, 30, 90)).Render(box), w)
	}
	if s.bootNameOpen {
		box := "New boot script (" + s.bootTemplate + ") — file name:\n" + s.bootCreateName.View(true) +
			"\n\nenter create · esc cancel"
		out = overlay(out, ui.BoxStyle.Width(clampI(w-8, 30, 90)).Render(box), w)
	}
	if s.confirm != nil {
		out += "\n" + s.confirm.View(clampI(w-4, 30, 140)) + "\n"
	}
	if s.pager != nil {
		out += "\n" + s.pager.View(w)
	}
	return out
}
