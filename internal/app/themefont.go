package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DevCoreXOfficial/termux-ui/internal/font"
	"github.com/DevCoreXOfficial/termux-ui/internal/termuxconf"
	"github.com/DevCoreXOfficial/termux-ui/internal/theme"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

type tfScreen struct {
	app    *App
	tab    int // 0 colors, 1 font
	colors *colorsTab
	fonts  *fontTab
}

func newThemeFontScreen(a *App) *tfScreen {
	s := &tfScreen{app: a, colors: newColorsTab(a), fonts: newFontTab(a)}
	return s
}

func (s *tfScreen) Title() string         { return "Themes & Fonts" }
func (s *tfScreen) WantsGlobalKeys() bool { return !s.colors.busy() && !s.fonts.busy() }

func (s *tfScreen) Update(a *App, msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case refreshMsg:
		s.colors.loadCurrent()
		return nil
	case fontDoneMsg:
		return s.fonts.done(a, t)
	case tea.KeyMsg:
		if t.String() == "tab" || t.String() == "shift+tab" {
			s.tab = 1 - s.tab
			return nil
		}
		if s.tab == 0 {
			return s.colors.Update(a, msg)
		}
		return s.fonts.Update(a, msg)
	}
	return nil
}

func (s *tfScreen) View() string {
	w := clampW(s.app.width, 40)
	tabs := []string{"Colors", "Font"}
	var rendered []string
	for i, name := range tabs {
		if i == s.tab {
			rendered = append(rendered, ui.SelStyle.Render(" "+name+" "))
		} else {
			rendered = append(rendered, ui.SubtleStyle.Render(" "+name+" "))
		}
	}
	head := strings.Join(rendered, "") + ui.SubtleStyle.Render("   (tab switches)")
	body := ""
	if s.tab == 0 {
		body = s.colors.View(w)
	} else {
		body = s.fonts.View(w)
	}
	return head + "\n\n" + body
}

// ---------------- Colors tab ----------------

type colorsTab struct {
	app        *App
	gallery    *ui.Menu
	customOpen bool
	fields     []hexField
	fieldIx    int
	importOpen bool
	importPath ui.TextInput
	confirm    *ui.Confirm
	pending    func(a *App) tea.Cmd
	pager      *ui.Pager
	current    *theme.Theme
}

type hexField struct {
	label string
	input ui.TextInput
}

func newColorsTab(a *App) *colorsTab {
	c := &colorsTab{app: a}
	bundled := theme.Bundled()
	items := make([]ui.MenuItem, len(bundled))
	for i, th := range bundled {
		items[i] = ui.MenuItem{Label: th.Name, Data: th}
	}
	c.gallery = ui.NewMenu(items)
	c.loadCurrent()
	return c
}

func (c *colorsTab) loadCurrent() {
	if th, err := theme.LoadColorsFile(); err == nil {
		c.current = th
	} else {
		c.current = nil
	}
}

func (c *colorsTab) busy() bool {
	return c.customOpen || c.importOpen || c.confirm != nil || c.pager != nil
}

func (c *colorsTab) selectedTheme() *theme.Theme {
	i := clampI(c.gallery.Cursor, 0, len(theme.Bundled())-1)
	bundled := theme.Bundled()
	if i < len(bundled) {
		return bundled[i]
	}
	return nil
}

func (c *colorsTab) Update(a *App, msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	key := k.String()

	if c.pager != nil {
		switch key {
		case "up":
			c.pager.Up()
		case "down":
			c.pager.Down()
		case "esc", "q":
			c.pager = nil
		}
		return nil
	}
	if c.importOpen {
		switch key {
		case "esc":
			c.importOpen = false
		case "enter":
			path := strings.TrimSpace(c.importPath.String())
			c.importOpen = false
			if path == "" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				a.ToastError("read: " + err.Error())
				return nil
			}
			th, err := theme.Parse(filepath.Base(path), string(data))
			if err != nil {
				a.ToastError("parse: " + err.Error())
				return nil
			}
			c.fields = themeFields(th)
			c.fieldIx = 0
			c.customOpen = true
			a.ToastGood("imported " + th.Name)
		default:
			typeInto(&c.importPath, key)
		}
		return nil
	}
	if c.customOpen {
		switch key {
		case "up":
			if c.fieldIx > 0 {
				c.fieldIx--
			}
		case "down":
			if c.fieldIx < len(c.fields)-1 {
				c.fieldIx++
			}
		case "tab":
			c.fieldIx = (c.fieldIx + 1) % (len(c.fields) + 1)
		case "enter":
			c.applyCustom(a)
		case "esc":
			if c.fieldIx >= len(c.fields) {
				c.customOpen = false
			} else if c.fieldIx > 0 {
				c.fieldIx--
			} else {
				c.customOpen = false
			}
		default:
			if c.fieldIx < len(c.fields) {
				typeInto(&c.fields[c.fieldIx].input, key)
				c.fields[c.fieldIx].input.Err = validateHexInput(c.fields[c.fieldIx].input.String())
			}
		}
		return nil
	}
	if c.confirm != nil {
		c.confirm.Update(key)
		if c.confirm.Result == 1 && c.pending != nil {
			fn := c.pending
			c.confirm, c.pending = nil, nil
			return fn(a)
		}
		if c.confirm.Result != 0 {
			c.confirm, c.pending = nil, nil
		}
		return nil
	}

	switch key {
	case "up", "k":
		c.gallery.Up()
	case "down", "j":
		c.gallery.Down()
	case "enter":
		th := c.selectedTheme()
		if th == nil {
			return nil
		}
		c.confirm = ui.NewConfirm(
			"Apply theme "+th.Name+"?",
			"Backs up ~/.termux/colors.properties and runs termux-reload-settings.",
			"y confirm · n cancel")
		c.pending = func(m *App) tea.Cmd { return applyThemeCmd(m, th.Name+" (bundled)", th) }
	case "c":
		base := c.current
		if base == nil {
			base = c.selectedTheme()
		}
		if base == nil {
			base = defaultTheme()
		}
		c.fields = themeFields(base)
		c.fieldIx = 0
		c.customOpen = true
	case "i":
		c.importOpen = true
		c.importPath = ui.TextInput{}
	case "R":
		if _, err := os.Stat(theme.ColorsPath()); err != nil {
			a.ToastInfo("no colors.properties to remove")
			return nil
		}
		c.confirm = ui.NewConfirm("Delete ~/.termux/colors.properties?",
			"Returns to the app default colors. A backup is kept.", "y confirm · n cancel")
		c.pending = func(m *App) tea.Cmd { return deleteThemeCmd(m) }
	}
	return nil
}

func validateHexInput(v string) string {
	if v == "" {
		return ""
	}
	if _, err := theme.NormalizeHex(v); err != nil {
		return "#RRGGBB"
	}
	return ""
}

func themeFields(th *theme.Theme) []hexField {
	fs := make([]hexField, 0, 19)
	for i := 0; i < 16; i++ {
		f := hexField{label: fmt.Sprintf("color%d", i)}
		f.input.Set(th.Colors[i])
		fs = append(fs, f)
	}
	for _, kv := range []struct {
		l string
		v string
	}{{"foreground", th.Foreground}, {"background", th.Background}, {"cursor", th.Cursor}} {
		f := hexField{label: kv.l}
		f.input.Set(kv.v)
		fs = append(fs, f)
	}
	return fs
}

func fieldsToTheme(fields []hexField, name string) (*theme.Theme, error) {
	th := &theme.Theme{Name: name}
	for i := 0; i < 16; i++ {
		h, err := theme.NormalizeHex(fields[i].input.String())
		if err != nil {
			return nil, fmt.Errorf("%s: %v", fields[i].label, err)
		}
		th.Colors[i] = h
	}
	hf, err := theme.NormalizeHex(fields[16].input.String())
	if err != nil {
		return nil, err
	}
	hb, err := theme.NormalizeHex(fields[17].input.String())
	if err != nil {
		return nil, err
	}
	hc, err := theme.NormalizeHex(fields[18].input.String())
	if err != nil {
		return nil, err
	}
	th.Foreground, th.Background, th.Cursor = hf, hb, hc
	return th, nil
}

func (c *colorsTab) applyCustom(a *App) tea.Cmd {
	th, err := fieldsToTheme(c.fields, "custom")
	if err != nil {
		a.ToastError(err.Error())
		return nil
	}
	return applyThemeCmd(a, "custom editor", th)
}

func defaultTheme() *theme.Theme {
	th := &theme.Theme{Name: "fallback"}
	for i := range th.Colors {
		th.Colors[i] = "#000000"
		if i == 7 || i == 15 {
			th.Colors[i] = "#FFFFFF"
		}
	}
	th.Foreground, th.Background, th.Cursor = "#FFFFFF", "#000000", "#FFFFFF"
	return th
}

func applyThemeCmd(a *App, source string, th *theme.Theme) tea.Cmd {
	return func() tea.Msg {
		path := theme.ColorsPath()
		oldData, _ := os.ReadFile(path)
		newContent := th.Render()
		diff := termuxDiff(string(oldData), newContent)
		_ = diff
		if _, err := saveWithBackup(path, newContent); err != nil {
			return childDone{title: "apply theme", err: err}
		}
		err := reloadSettingsSafe()
		return childDone{title: "applied " + source + " (" + th.Name + ") · reloaded", err: err,
			after: func(m *App, e error) tea.Cmd {
				m.current().Update(m, refreshMsg{})
				if e != nil {
					m.ToastWarn("if colors did not change, restart Termux")
				}
				return nil
			}}
	}
}

func deleteThemeCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		path := theme.ColorsPath()
		if _, err := backupFile(path); err != nil {
			return childDone{title: "backup before delete", err: err}
		}
		err := os.Remove(path)
		reloadErr := termuxconfReload()
		if err != nil {
			return childDone{title: "delete colors.properties", err: err,
				after: func(m *App, e error) tea.Cmd { m.current().Update(m, refreshMsg{}); return nil }}
		}
		return childDone{title: "deleted colors.properties · reloaded", err: reloadErr,
			after: func(m *App, e error) tea.Cmd { m.current().Update(m, refreshMsg{}); return nil }}
	}
}

func (c *colorsTab) View(w int) string {
	leftW := clampI(w/3, 22, 34)
	hh := clampI(c.app.height-12, 4, 30)
	c.gallery.Height = hh

	left := ui.TitleStyle.Render("Bundled themes") + "\n" +
		ui.SubtleStyle.Render(strings.Repeat("─", leftW)) + "\n" + c.gallery.View(leftW)

	right := c.previewView(clampI(w-leftW-2, 20, 200))
	out := lipJoin(fixedWidth(left, leftW), right, w)
	out += "\n" + ui.SubtleStyle.Render("enter apply · c custom editor · i import .properties · R reset to app defaults")

	if c.customOpen {
		out = overlay(out, c.customView(clampI(w-8, 34, 90)), w)
	}
	if c.importOpen {
		box := "Import .properties — full path:\n" + c.importPath.View(true) + "\n\nenter load · esc cancel"
		out = overlay(out, ui.BoxStyle.Width(clampI(w-8, 30, 100)).Render(box), w)
	}
	if c.confirm != nil {
		out += "\n" + c.confirm.View(clampI(w-4, 30, 140)) + "\n"
	}
	if c.pager != nil {
		out += "\n" + c.pager.View(w)
	}
	return out
}

func (c *colorsTab) previewView(width int) string {
	th := c.selectedTheme()
	title := ui.TitleStyle.Render("Preview: " + th.Name)
	if c.current != nil {
		title += "   " + ui.SubtleStyle.Render("(current file: "+c.current.Name+")")
	}
	rows := []struct{ label string }{
		{" normal "}, {" bold "}, {" italic "}, {" reverse "},
		{" $ ls -la "}, {" ERROR: exit 1 "}, {" warn: deprecated "}, {" debug: ok "},
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	pair := func(fg, bg int) string {
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Colors[fg])).Background(lipgloss.Color(th.Colors[bg]))
		return st.Render(rows[(fg)%len(rows)].label)
	}
	for row := 0; row < 8; row++ {
		line := pair(row, row) + " "
		bright := row + 8
		if bright < 16 {
			line += pair(bright, bright)
		}
		b.WriteString(line + "\n")
	}
	fgst := lipgloss.NewStyle().Foreground(lipgloss.Color(th.Foreground)).Background(lipgloss.Color(th.Background))
	b.WriteString("\n" + fgst.Render(" foreground on background ") + "\n")
	curSt := lipgloss.NewStyle().Background(lipgloss.Color(th.Cursor))
	b.WriteString(curSt.Render(" cursor ") + "\n")
	return b.String()
}

func (c *colorsTab) customView(w int) string {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Custom editor — 19 values (#RRGGBB)") + "\n\n")
	start := clampI(c.fieldIx-6, 0, maxI(0, len(c.fields)-13))
	end := clampI(start+13, start, len(c.fields)+1)
	for i := start; i < end && i < len(c.fields); i++ {
		f := c.fields[i]
		cursor := "  "
		if i == c.fieldIx {
			cursor = ui.KeyStyle.Render("> ")
		}
		sw := ""
		if v := f.input.String(); theme.ValidHex(v) {
			sw = lipgloss.NewStyle().Background(lipgloss.Color(v)).Render("      ")
		}
		b.WriteString(cursor + fmt.Sprintf("%-11s %s %s", f.label, f.input.View(i == c.fieldIx), sw) + "\n")
	}
	if c.fieldIx >= len(c.fields) {
		b.WriteString(ui.SelStyle.Render("> [ Save & apply ]"))
	} else {
		b.WriteString("  " + ui.SubtleStyle.Render("[ Save & apply ]"))
	}
	b.WriteString("\n\n" + ui.SubtleStyle.Render("tab jumps to Save · enter saves · esc back"))
	return ui.BoxStyle.Width(clampI(w-6, 32, 88)).Render(b.String())
}

// ---------------- Font tab ----------------

type fontDoneMsg struct {
	title string
	err   error
}

type fontTab struct {
	app       *App
	menu      *ui.Menu
	localMode bool
	localPath ui.TextInput
	confirm   *ui.Confirm
	pending   func(a *App) tea.Cmd
	working   bool
	lastErr   error
	lastOK    string
}

func newFontTab(a *App) *fontTab {
	f := &fontTab{app: a}
	f.rebuild()
	return f
}

func (f *fontTab) rebuild() {
	entries := font.Catalog()
	items := make([]ui.MenuItem, 0, len(entries)+2)
	for _, e := range entries {
		desc := "download + install"
		if e.Recommended {
			desc = "recommended for powerlevel10k"
		}
		items = append(items, ui.MenuItem{Label: e.Name, Desc: desc})
	}
	items = append(items, ui.MenuItem{Label: "Local .ttf file…", Desc: "install from device storage"})
	if ok, _ := font.Installed(); ok {
		items = append(items, ui.MenuItem{Label: "Remove font.ttf", Desc: "back to built-in font"})
	}
	f.menu = ui.NewMenu(items)
}

func (f *fontTab) busy() bool { return f.localMode || f.confirm != nil }

func (f *fontTab) done(a *App, msg fontDoneMsg) tea.Cmd {
	f.working = false
	if msg.err != nil {
		f.lastErr = msg.err
		a.ToastError(msg.title + ": " + msg.err.Error())
		return nil
	}
	f.lastErr = nil
	f.lastOK = msg.title
	a.ToastGood(msg.title)
	f.rebuild()
	return nil
}

func (f *fontTab) installEntry(e font.Entry) tea.Cmd {
	f.working = true
	name := e.Name
	return func() tea.Msg {
		data, err := downloadURL(e.URL)
		if err != nil {
			return fontDoneMsg{title: "install " + name, err: err}
		}
		if strings.HasSuffix(strings.ToLower(e.URL), ".zip") || !font.IsTTF(data) {
			data, err = font.ExtractFromZip(data, e.ZipMember)
			if err != nil {
				return fontDoneMsg{title: "install " + name, err: err}
			}
		}
		if err := font.Install(data); err != nil {
			return fontDoneMsg{title: "install " + name, err: err}
		}
		if err := termuxconf.ReloadSettings(); err != nil {
			return fontDoneMsg{title: "installed " + name + " (reload failed)", err: err}
		}
		return fontDoneMsg{title: "installed " + name + " — if not applied immediately, force-stop Termux", err: nil}
	}
}

func (f *fontTab) Update(a *App, msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok {
		key := k.String()
		if f.localMode {
			switch key {
			case "esc":
				f.localMode = false
			case "enter":
				path := strings.TrimSpace(f.localPath.String())
				f.localMode = false
				if path == "" {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					a.ToastError("read: " + err.Error())
					return nil
				}
				if !font.IsTTF(data) {
					a.ToastError("not a valid TTF file")
					return nil
				}
				name := filepath.Base(path)
				return func() tea.Msg {
					if err := font.InstallLocal(path); err != nil {
						return fontDoneMsg{title: "install " + name, err: err}
					}
					if err := termuxconf.ReloadSettings(); err != nil {
						return fontDoneMsg{title: "installed " + name, err: err}
					}
					return fontDoneMsg{title: "installed " + name + " — force-stop Termux if not applied", err: nil}
				}
			default:
				typeInto(&f.localPath, key)
			}
			return nil
		}
		if f.confirm != nil {
			f.confirm.Update(key)
			if f.confirm.Result == 1 && f.pending != nil {
				fn := f.pending
				f.confirm, f.pending = nil, nil
				return fn(a)
			}
			if f.confirm.Result != 0 {
				f.confirm, f.pending = nil, nil
			}
			return nil
		}
		entries := font.Catalog()
		hasRemove := false
		if ok, _ := font.Installed(); ok {
			hasRemove = true
		}
		switch key {
		case "up", "k":
			f.menu.Up()
		case "down", "j":
			f.menu.Down()
		case "enter":
			i := f.menu.Cursor
			switch {
			case i < len(entries):
				return f.installEntry(entries[i])
			case i == len(entries):
				f.localMode = true
				f.localPath = ui.TextInput{}
			case hasRemove && i == len(entries)+1:
				f.confirm = ui.NewConfirm("Delete ~/.termux/font.ttf?",
					"Returns to the built-in Termux font. A timestamped backup is kept first.", "y confirm · n cancel")
				f.pending = func(m *App) tea.Cmd {
					return func() tea.Msg {
						err := font.RemoveFont()
						reloadErr := termuxconf.ReloadSettings()
						if err != nil {
							return fontDoneMsg{title: "remove font", err: err}
						}
						return fontDoneMsg{title: "removed font.ttf — back to built-in font", err: reloadErr}
					}
				}
			}
		}
	}
	return nil
}

func (f *fontTab) View(w int) string {
	ok, size := font.Installed()
	status := ui.BadStyle.Render("no custom font (built-in)")
	if ok {
		status = ui.GoodStyle.Render(fmt.Sprintf("custom font installed (%d KB)", size/1024))
	}
	var b strings.Builder
	b.WriteString("Status: " + status + "\n")
	if f.lastOK != "" {
		b.WriteString(ui.SubtleStyle.Render("last action: "+f.lastOK) + "\n")
	}
	if f.lastErr != nil {
		b.WriteString(ui.WarnStyle.Render("last error: "+f.lastErr.Error()) + "\n")
	}
	b.WriteString("\n")
	f.menu.Height = clampI(f.app.height-10, 4, 24)
	b.WriteString(f.menu.View(clampI(w-2, 30, 200)))
	b.WriteString("\n" + ui.SubtleStyle.Render("enter install · zips unpack automatically (no unzip needed)"))
	if f.working {
		b.WriteString("\n" + ui.WarnStyle.Render("downloading…"))
	}
	if f.localMode {
		box := "Install local .ttf — full path:\n" + f.localPath.View(true) + "\n\nenter install · esc cancel"
		b.WriteString("\n\n" + ui.BoxStyle.Width(clampI(w-8, 30, 100)).Render(box))
	}
	if f.confirm != nil {
		b.WriteString("\n" + f.confirm.View(clampI(w-4, 30, 140)) + "\n")
	}
	return b.String()
}
