package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/termuxconf"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

// fieldState is the live editor state of one property row.
type fieldState struct {
	def       propDef
	toggle    ui.Toggle
	radio     ui.Radio
	text      ui.TextInput
	effective string // value when loaded
}

func (f *fieldState) current() string {
	switch f.def.Kind {
	case kToggle:
		return boolToProp(f.toggle.Value)
	case kRadio:
		return f.radio.Value()
	default:
		return f.text.String()
	}
}

func (f *fieldState) modified() bool { return f.current() != f.effective }

type settingsScreen struct {
	app    *App
	groups *ui.Menu
	gi     int
	fields [][]*fieldState
	focus  int // focused field inside the group
	path   string

	confirm     *ui.Confirm
	pendingSave func(a *App) tea.Cmd
	dangerInput *ui.TextInput
	dangerIdx   [2]int // group, field being danger-enabled
	diffPager   *ui.Pager
	diffCommit  func(a *App) tea.Cmd
}

func newSettingsScreen(a *App) *settingsScreen {
	s := &settingsScreen{app: a}
	home, _ := os.UserHomeDir()
	s.path = filepath.Join(home, ".termux", "termux.properties")
	s.load()
	items := make([]ui.MenuItem, len(propGroups))
	for i, g := range propGroups {
		items[i].Label = g.Title
	}
	s.groups = ui.NewMenu(items)
	return s
}

func (s *settingsScreen) load() {
	doc, err := termuxconf.Load(s.path)
	if err != nil {
		s.app.ToastError("reading properties: " + err.Error())
	}
	s.fields = s.fields[:0]
	for _, g := range propGroups {
		var row []*fieldState
		for _, d := range g.Props {
			fs := &fieldState{def: d, effective: effectiveValue(doc, d)}
			switch d.Kind {
			case kToggle:
				fs.toggle.Value = parseBoolValue(fs.effective)
				if fs.effective != "true" && fs.effective != "false" {
					fs.toggle.Value = false
					fs.effective = "false"
				}
			case kRadio:
				opts := d.Options
				fs.radio = ui.Radio{Options: opts}
				for i, o := range opts {
					if o == strings.TrimSpace(fs.effective) {
						fs.radio.Index = i
						break
					}
				}
			case kNumber:
				fs.text.Set(strings.TrimSpace(fs.effective))
			case kText:
				fs.text.Set(fs.effective)
			}
			row = append(row, fs)
		}
		s.fields = append(s.fields, row)
	}
}

func (s *settingsScreen) Title() string { return "Termux Settings" }
func (s *settingsScreen) WantsGlobalKeys() bool {
	return s.confirm == nil && s.dangerInput == nil && s.diffPager == nil
}

func (s *settingsScreen) modifiedCount() int {
	n := 0
	for _, grp := range s.fields {
		for _, f := range grp {
			if f.modified() {
				n++
			}
		}
	}
	return n
}

func (s *settingsScreen) focusedField() *fieldState {
	if s.gi >= len(s.fields) || len(s.fields[s.gi]) == 0 {
		return nil
	}
	return s.fields[s.gi][clampI(s.focus, 0, len(s.fields[s.gi])-1)]
}

func (s *settingsScreen) Update(a *App, msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case refreshMsg:
		s.load()
		return nil

	case tea.KeyMsg:
		key := t.String()

		if s.dangerInput != nil {
			switch key {
			case "esc":
				s.dangerInput = nil
				return nil
			case "enter":
				if strings.EqualFold(strings.TrimSpace(s.dangerInput.String()), "yes") {
					g, i := s.dangerIdx[0], s.dangerIdx[1]
					s.dangerInput = nil
					s.fields[g][i].toggle.Value = true
				} else {
					a.ToastError("type yes to confirm")
					s.dangerInput.Err = "type yes"
				}
				return nil
			default:
				typeInto(s.dangerInput, key)
				s.dangerInput.Err = ""
				return nil
			}
		}

		if s.diffPager != nil {
			switch key {
			case "up", "k":
				s.diffPager.Up()
			case "down", "j":
				s.diffPager.Down()
			case "y", "Y":
				fn := s.diffCommit
				s.diffPager, s.diffCommit = nil, nil
				return fn(a)
			case "n", "N", "esc", "q":
				s.diffPager, s.diffCommit = nil, nil
				a.ToastInfo("cancelled — nothing written")
			}
			return nil
		}

		if s.confirm != nil {
			s.confirm.Update(key)
			if s.confirm.Result != 0 {
				s.confirm, s.pendingSave = nil, nil
			}
			return nil
		}

		switch key {
		case "up", "k":
			if !s.atInfoGroup() && s.focus > 0 {
				s.focus--
			} else if s.atInfoGroup() || s.focus == 0 {
				if s.groups.Cursor > 0 {
					s.groups.Cursor--
					s.gi = s.groups.Cursor
					s.focus = 0
				}
			}
		case "down", "j":
			g := propGroups[s.gi]
			if !s.atInfoGroup() && s.focus < len(g.Props)-1 {
				s.focus++
			} else if s.groups.Cursor < len(propGroups)-1 {
				s.groups.Cursor++
				s.gi = s.groups.Cursor
				s.focus = 0
			}
		case "left", "h":
			f := s.focusedField()
			if f != nil && f.def.Kind == kRadio {
				f.radio.Left()
			} else if f != nil && f.def.Kind == kText {
				s.editText(f, key)
			}
		case "right", "l":
			f := s.focusedField()
			if f != nil && f.def.Kind == kRadio {
				f.radio.Right()
			} else if f != nil && f.def.Kind == kText {
				s.editText(f, key)
			}
		case "enter", " ", "space":
			f := s.focusedField()
			if f == nil {
				break
			}
			switch f.def.Kind {
			case kToggle:
				if f.def.Danger && !f.toggle.Value {
					s.dangerInput = &ui.TextInput{}
					s.dangerIdx = [2]int{s.gi, s.focus}
					a.ToastWarn("Type 'yes' to allow external apps (security risk); esc cancels")
					return nil
				}
				f.toggle.Flip()
			case kRadio:
				f.radio.Right()
			case kNumber, kText:
				s.editText(f, "")
			}
		case "backspace":
			f := s.focusedField()
			if f != nil && (f.def.Kind == kText || f.def.Kind == kNumber) {
				s.editText(f, key)
			}
		case "z":
			f := s.focusedField()
			if f != nil {
				switch f.def.Kind {
				case kToggle:
					f.toggle.Value = parseBoolValue(f.def.Def)
				case kRadio:
					f.radio.Set(f.def.Def)
				default:
					f.text.Set(f.def.Def)
					f.text.Err = ""
				}
				a.ToastInfo("reset to default — save to remove the key from the file")
			}
		case "S":
			return s.beginSave(a)
		case "E":
			return a.OpenInEditor(s.path)
		}
	}
	return nil
}

var _ = fmt.Sprintf

func (s *settingsScreen) atInfoGroup() bool {
	return len(propGroups[s.gi].Props) == 0
}

func (s *settingsScreen) editText(f *fieldState, key string) {
	switch key {
	case "":
	case "left":
		f.text.Left()
		return
	case "right":
		f.text.Right()
		return
	case "backspace":
		f.text.Backspace()
		return
	default:
		if len(key) == 1 {
			f.text.Insert([]rune(key)[0])
		}
	}
	f.text.Err = ""
	if f.def.Validate != nil {
		if err := f.def.Validate(f.text.String()); err != nil {
			f.text.Err = err.Error()
		}
	}
	if f.def.Kind == kNumber {
		if err := numberOK(f.def, f.text.String()); err != nil && f.text.String() != "" {
			f.text.Err = err.Error()
		}
	}
}

// buildDoc renders a candidate document with all modifications applied.
func (s *settingsScreen) buildDoc() (*termuxconf.File, error) {
	doc, err := termuxconf.Load(s.path)
	if err != nil {
		return nil, err
	}
	var firstErr error
	for _, grp := range s.fields {
		for _, f := range grp {
			v := f.current()
			if f.def.Validate != nil {
				if err := f.def.Validate(v); err != nil && v != f.effective {
					if firstErr == nil {
						firstErr = fmt.Errorf("%s: %v", f.def.Key, err)
					}
					continue
				}
			}
			if f.def.Kind == kNumber {
				if err := numberOK(f.def, v); err != nil {
					if firstErr == nil {
						firstErr = fmt.Errorf("%s: %v", f.def.Key, err)
					}
					continue
				}
			}
			applyFieldToDoc(doc, f.def, v)
		}
	}
	return doc, firstErr
}

func (s *settingsScreen) beginSave(a *App) tea.Cmd {
	doc, err := s.buildDoc()
	if err != nil {
		a.ToastError(err.Error() + " — fix the highlighted fields")
		return nil
	}
	oldData, _ := os.ReadFile(s.path)
	newContent := doc.Render()
	diff := termuxconf.UnifiedDiff(s.path+" (current)", s.path+" (new)", string(oldData), newContent)
	if diff == "" {
		a.ToastInfo("nothing to change")
		return nil
	}
	var restartProps []string
	for _, grp := range s.fields {
		for _, f := range grp {
			if f.modified() && f.def.Restart {
				restartProps = append(restartProps, f.def.Key)
			}
		}
	}
	s.diffPager = ui.NewPager("unified diff — exactly the lines that change", diff)
	s.diffPager.Height = clampI(a.height-8, 6, 40)
	s.diffCommit = func(m *App) tea.Cmd { return m.commitProperties(s.path, string(oldData), newContent, restartProps) }
	return nil
}

func (a *App) commitProperties(path, oldContent, newContent string, restartProps []string) tea.Cmd {
	return func() tea.Msg {
		backup, err := termuxconf.Save(path, newContent, timeNow())
		if err != nil {
			return childDone{title: "save settings", err: err}
		}
		msg := "saved"
		if backup != "" {
			msg += " · backup: " + filepath.Base(backup)
		}
		if err := termuxconf.ReloadSettings(); err != nil {
			return childDone{title: msg + " but reload failed: " + err.Error(), err: err,
				after: func(m *App, e error) tea.Cmd { return m.current().Update(m, refreshMsg{}) }}
		}
		return childDone{title: msg + " · reloaded", err: nil,
			after: func(m *App, e error) tea.Cmd {
				for _, prop := range restartProps {
					m.state.MarkRestart(prop)
				}
				if len(restartProps) > 0 {
					m.SaveState()
					m.ToastWarn("restart-required changed: " + strings.Join(restartProps, ", "))
				}
				return m.current().Update(m, refreshMsg{})
			}}
	}
}

func (s *settingsScreen) View() string {
	w := clampW(s.app.width, 40)
	h := clampI(s.app.height-9, 6, 40)

	leftW := clampI(w/4, 18, 28)
	s.groups.Height = h
	left := ui.TitleStyle.Render("Groups") + "\n" +
		ui.SubtleStyle.Render(strings.Repeat("─", leftW)) + "\n" + s.groups.View(leftW)

	g := propGroups[s.gi]
	right := ui.TitleStyle.Render(g.Title) + "\n" +
		ui.SubtleStyle.Render(strings.Repeat("─", clampI(w-leftW-3, 10, 200))) + "\n\n"

	if s.atInfoGroup() {
		for _, l := range g.Info {
			right += l + "\n"
		}
	} else {
		fields := s.fields[s.gi]
		start := clampI(s.focus-(h-1)+2, 0, maxI(0, len(fields)-(h-1)))
		for i, f := range fields[start:] {
			idx := start + i
			cursor := "  "
			if idx == s.focus {
				cursor = ui.KeyStyle.Render("> ")
			}
			label := f.def.Key
			if f.def.Restart {
				label += " ⟳"
			}
			line := cursor + label
			val := ""
			switch f.def.Kind {
			case kToggle:
				val = f.toggle.View()
			case kRadio:
				val = strings.ReplaceAll(f.radio.View(), "\n", "   ")
			default:
				focused := idx == s.focus
				val = f.text.View(focused)
			}
			right += line + "\n    " + val + "\n"
			hints := []string{}
			if f.def.Note != "" {
				hints = append(hints, ui.SubtleStyle.Render(f.def.Note))
			}
			if f.def.WarnOver > 0 {
				if n, ok := atoiSafe(f.text.String()); ok && n > f.def.WarnOver {
					hints = append(hints, ui.WarnStyle.Render(fmt.Sprintf("large buffers slow scrolling (> %d)", f.def.WarnOver)))
				}
			}
			if f.modified() {
				hints = append(hints, ui.GoodStyle.Render("modified"))
			}
			if len(hints) > 0 {
				right += "    " + strings.Join(hints, "  ") + "\n"
			}
		}
		right += "\n" + ui.SubtleStyle.Render("⟳ = restart-required · z reset field to default")
	}

	modified := s.modifiedCount()
	status := fmt.Sprintf("Modified: %d   S save (diff → confirm → backup → reload)   E open in $EDITOR", modified)
	out := lipJoin(fixedWidth(left, leftW), right, w) + "\n" + ui.SubtleStyle.Render(status) + "\n"
	if s.dangerInput != nil {
		box := "Confirm security risk — type 'yes' to enable:\n" + s.dangerInput.View(true)
		out = overlay(out, ui.BoxStyle.Width(clampI(w-8, 24, 120)).Render(box), w)
	}
	if s.confirm != nil {
		out += "\n" + s.confirm.View(clampI(w-4, 30, 140)) + "\n"
	}
	if s.diffPager != nil {
		return overlay(out, s.diffPager.View(w), w) + "\n y apply & reload · n cancel\n"
	}
	return out
}

func atoiSafe(s string) (int, bool) {
	n := 0
	ok := len(s) > 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, ok
}
