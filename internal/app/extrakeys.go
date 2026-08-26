package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/catalog"
	"github.com/DevCoreXOfficial/termux-ui/internal/extrakeys"
	"github.com/DevCoreXOfficial/termux-ui/internal/termuxconf"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

// ekCellEditor is the modal form for one grid cell.
type ekCellEditor struct {
	kind         int // 0 literal, 1 named, 2 macro
	value        ui.TextInput
	display      ui.TextInput
	popupKind    int // 0 none, 1 simple key, 2 macro
	popupValue   ui.TextInput
	popupDisplay ui.TextInput
	field        int

	paletteOpen   bool
	paletteCur    int
	paletteForPop bool
}

func newEkCellEditor(cell extrakeys.Cell) *ekCellEditor {
	e := &ekCellEditor{}
	switch cell.Kind {
	case extrakeys.KindNamed:
		e.kind = 1
	case extrakeys.KindMacro:
		e.kind = 2
	default:
		e.kind = 0
	}
	e.value.Set(cell.Value)
	e.display.Set(cell.Display)
	if cell.Popup.Has {
		if cell.Popup.Kind == extrakeys.KindMacro {
			e.popupKind = 2
		} else {
			e.popupKind = 1
			if cell.Popup.Kind == extrakeys.KindNamed {
				e.popupKind = 1
			}
		}
		e.popupValue.Set(cell.Popup.Value)
		e.popupDisplay.Set(cell.Popup.Display)
	}
	return e
}

func (e *ekCellEditor) build() extrakeys.Cell {
	var c extrakeys.Cell
	switch e.kind {
	case 1:
		c = extrakeys.Named(e.value.String())
	case 2:
		c = extrakeys.Macro(e.value.String(), e.display.String())
	default:
		c = extrakeys.Lit(e.value.String())
	}
	switch e.popupKind {
	case 1:
		c.Popup = extrakeys.Popup{Has: true, Kind: classifyKind(e.popupValue.String()), Value: e.popupValue.String()}
	case 2:
		c.Popup = extrakeys.MacroPopup(e.popupValue.String(), e.popupDisplay.String())
	}
	return c
}

func classifyKind(v string) extrakeys.CellKind {
	c := extrakeys.Classify(v)
	if c.Kind == extrakeys.KindMacro && len(strings.Fields(v)) <= 1 {
		return extrakeys.KindLiteral
	}
	return c.Kind
}

type ekScreen struct {
	app    *App
	layout *extrakeys.Layout
	r, c   int

	editor      *ekCellEditor
	editorAt    [2]int
	styleRadio  ui.Radio
	caps        ui.Toggle
	presetOpen  bool
	presetCur   int
	confirm     *ui.Confirm
	diffPager   *ui.Pager
	diffCommit  func(a *App) tea.Cmd
	errorsShown []error
	path        string
}

func newExtraKeysScreen(a *App) *ekScreen {
	s := &ekScreen{app: a, layout: &extrakeys.Layout{}}
	home, _ := os.UserHomeDir()
	s.path = filepath.Join(home, ".termux", "termux.properties")
	doc, _ := termuxconf.Load(s.path)
	if raw := doc.Get("extra-keys", ""); raw != "" {
		if l, err := extrakeys.Parse(raw); err == nil {
			s.layout = l
		}
	} else {
		s.layout.Rows = [][]extrakeys.Cell{{extrakeys.Named("ESC"), extrakeys.Named("TAB"), extrakeys.Named("CTRL")}}
	}
	s.styleRadio = ui.Radio{Options: catalog.ExtraKeysStyleValues}
	cur := doc.Get("extra-keys-style", "default")
	for i, v := range catalog.ExtraKeysStyleValues {
		if v == cur {
			s.styleRadio.Index = i
		}
	}
	s.caps = ui.Toggle{Value: parseBoolValue(doc.Get("extra-keys-text-all-caps", "true"))}
	return s
}

func (s *ekScreen) Title() string { return "Extra Keys Builder" }
func (s *ekScreen) WantsGlobalKeys() bool {
	return s.editor == nil && !s.presetOpen && s.confirm == nil && s.diffPager == nil
}

func (s *ekScreen) Update(a *App, msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case refreshMsg:
		return nil
	case tea.KeyMsg:
		key := t.String()

		if s.diffPager != nil {
			switch key {
			case "up", "k":
				s.diffPager.Up()
			case "down", "j":
				s.diffPager.Down()
			case "y":
				fn := s.diffCommit
				s.diffPager, s.diffCommit = nil, nil
				return fn(a)
			case "n", "esc", "q":
				s.diffPager, s.diffCommit = nil, nil
				a.ToastInfo("cancelled — nothing written")
			}
			return nil
		}
		if s.confirm != nil {
			s.confirm.Update(key)
			if s.confirm.Result == 1 && s.diffCommit != nil {
				fn := s.diffCommit
				s.confirm = nil
				return fn(a)
			}
			if s.confirm.Result != 0 {
				s.confirm = nil
			}
			return nil
		}
		if s.presetOpen {
			switch key {
			case "up", "k":
				if s.presetCur > 0 {
					s.presetCur--
				}
			case "down", "j":
				if s.presetCur < len(catalog.Presets)-1 {
					s.presetCur++
				}
			case "enter":
				p := catalog.Presets[s.presetCur]
				s.layout = cloneLayout(p.Layout)
				s.r, s.c = 0, 0
				s.presetOpen = false
				a.ToastGood("preset loaded: " + p.Name)
			case "esc", "q":
				s.presetOpen = false
			}
			return nil
		}
		if s.editor != nil {
			return s.updateEditor(a, key)
		}

		rows := s.layout.Rows
		switch key {
		case "up", "k":
			if s.r > 0 {
				s.r--
				s.c = clampI(s.c, 0, maxI(0, len(rows[s.r])-1))
			}
		case "down", "j":
			if s.r < len(rows)-1 {
				s.r++
				s.c = clampI(s.c, 0, maxI(0, len(rows[s.r])-1))
			}
		case "left", "h":
			if s.c > 0 {
				s.c--
			}
		case "right", "l":
			if s.c < len(rows[s.r])-1 {
				s.c++
			}
		case "enter", " ", "space":
			if len(rows[s.r]) > 0 {
				s.editorAt = [2]int{s.r, s.c}
				s.editor = newEkCellEditor(rows[s.r][s.c])
			}
		case "+", "=":
			row := append(rows[s.r], extrakeys.Lit(""))
			copy(row[s.c+2:], row[s.c+1:])
			if s.c+1 < len(row) {
				row[s.c+1] = extrakeys.Lit("")
			} else {
				row[len(row)-1] = extrakeys.Lit("")
			}
			rows[s.r] = row
			if s.c < len(row)-1 {
				s.c++
			}
		case "-":
			if len(rows[s.r]) > 1 {
				rows[s.r] = append(rows[s.r][:s.c], rows[s.r][s.c+1:]...)
				s.c = clampI(s.c, 0, len(rows[s.r])-1)
			}
		case "R":
			if len(rows) < extrakeys.MaxRows {
				s.layout.Rows = append(s.layout.Rows, []extrakeys.Cell{extrakeys.Lit("")})
				s.r = len(s.layout.Rows) - 1
				s.c = 0
			}
		case "X":
			if len(rows) > 1 {
				s.layout.Rows = append(rows[:s.r], rows[s.r+1:]...)
				s.r = clampI(s.r, 0, len(s.layout.Rows)-1)
				s.c = clampI(s.c, 0, maxI(0, len(s.layout.Rows[s.r])-1))
			}
		case "p":
			s.presetOpen = true
			s.presetCur = 0
		case "S":
			return s.beginSave(a)
		case "E":
			return a.OpenInEditor(s.path)
		}
	}
	return nil
}

func cloneLayout(l *extrakeys.Layout) *extrakeys.Layout {
	out := &extrakeys.Layout{}
	for _, row := range l.Rows {
		out.Rows = append(out.Rows, append([]extrakeys.Cell(nil), row...))
	}
	return out
}

func (s *ekScreen) updateEditor(a *App, key string) tea.Cmd {
	e := s.editor
	if e.paletteOpen {
		names := flatPalette()
		switch key {
		case "esc":
			e.paletteOpen = false
		case "up", "k":
			if e.paletteCur > 0 {
				e.paletteCur--
			}
		case "down", "j":
			if e.paletteCur < len(names)-1 {
				e.paletteCur++
			}
		case "enter":
			if n := names[e.paletteCur]; n.name != "" {
				target := &e.value
				if e.paletteForPop {
					target = &e.popupValue
				}
				target.Set(n.name)
				e.paletteOpen = false
			}
		}
		return nil
	}

	fields := 6
	if e.popupKind == 0 {
		fields = 4
	}
	switch key {
	case "up", "k":
		if e.field > 0 {
			e.field--
		}
	case "down", "j", "tab":
		if e.field < fields-1 {
			e.field++
		}
	case "left":
		if e.field == 0 && e.kind > 0 {
			e.kind--
		}
		if e.field == 3 && e.popupKind > 0 {
			e.popupKind--
		}
	case "right":
		if e.field == 0 && e.kind < 2 {
			e.kind++
		}
		if e.field == 3 && e.popupKind < 2 {
			e.popupKind++
		}
	case "enter":
		switch e.field {
		case 1:
			e.paletteOpen = true
			e.paletteForPop = false
			e.paletteCur = 0
		case 4:
			if e.popupKind >= 1 {
				e.paletteOpen = true
				e.paletteForPop = true
				e.paletteCur = 0
			}
		}
	case "esc":
		s.editor = nil
	case "ctrl+s":
		cell := e.build()
		if err := validateCell(cell); err != "" {
			a.ToastError(err)
			return nil
		}
		s.layout.Rows[s.editorAt[0]][s.editorAt[1]] = cell
		s.editor = nil
	case "backspace":
		switch e.field {
		case 1:
			e.value.Backspace()
		case 2:
			e.display.Backspace()
		case 4:
			e.popupValue.Backspace()
		case 5:
			e.popupDisplay.Backspace()
		}
	default:
		if len(key) == 1 {
			switch e.field {
			case 1:
				e.value.Insert([]rune(key)[0])
			case 2:
				e.display.Insert([]rune(key)[0])
			case 4:
				e.popupValue.Insert([]rune(key)[0])
			case 5:
				e.popupDisplay.Insert([]rune(key)[0])
			}
		}
	}
	return nil
}

func validateCell(c extrakeys.Cell) string {
	switch c.Kind {
	case extrakeys.KindLiteral:
		if c.Value == "" {
			return "empty key"
		}
		if strings.ContainsRune(c.Value, '\\') {
			return "backslash not allowed as a key — use BACKSLASH from the palette"
		}
		if n := len([]rune(c.Value)); n > 1 {
			return "literal keys must be a single character"
		}
	case extrakeys.KindNamed:
		if !extrakeys.IsNamedKey(c.Value) {
			return "unknown key name: " + c.Value
		}
	case extrakeys.KindMacro:
		if strings.TrimSpace(c.Value) == "" {
			return "macro needs content"
		}
	}
	if c.Popup.Has {
		if c.Popup.Kind == extrakeys.KindLiteral && strings.ContainsRune(c.Popup.Value, '\\') {
			return "popup backslash not allowed — use BACKSLASH"
		}
		if c.Popup.Kind != extrakeys.KindMacro && c.Popup.Value == "" {
			return "popup needs a value"
		}
	}
	return ""
}

type paletteItem struct {
	group string
	name  string
}

func flatPalette() []paletteItem {
	var out []paletteItem
	for _, g := range extrakeys.NamedKeyGroups {
		out = append(out, paletteItem{group: g.Title})
		out = append(out, newItems(g.Keys)...)
	}
	return out
}

func newItems(keys []string) []paletteItem {
	out := make([]paletteItem, len(keys))
	for i, k := range keys {
		out[i] = paletteItem{name: k}
	}
	return out
}

func (s *ekScreen) beginSave(a *App) tea.Cmd {
	errs := extrakeys.Validate(s.layout)
	if len(errs) > 0 {
		s.errorsShown = errs
		a.ToastError(fmt.Sprintf("%d problem(s) — fix before saving (see below)", len(errs)))
		return nil
	}
	s.errorsShown = nil

	oldData, _ := os.ReadFile(s.path)
	before := string(oldData)

	work := termuxconf.Parse(before)
	work.SetWrapped("extra-keys", extrakeys.Serialize(s.layout))
	setOrCleanDefault(work, "extra-keys-style", s.styleRadio.Value(), "default")
	newCaps := boolToProp(s.caps.Value)
	if newCaps == "true" {
		setOrCleanDefault(work, "extra-keys-text-all-caps", "true", "true")
	} else {
		work.Set("extra-keys-text-all-caps", "false")
	}

	newContent := work.Render()
	diff := termuxconf.UnifiedDiff(s.path+" (current)", s.path+" (new)", before, newContent)
	if diff == "" {
		a.ToastInfo("nothing to change")
		return nil
	}
	s.diffPager = ui.NewPager("unified diff", diff)
	s.diffPager.Height = clampI(a.height-8, 6, 40)
	s.diffCommit = func(m *App) tea.Cmd {
		return func() tea.Msg {
			if _, err := termuxconf.Save(s.path, newContent, timeNow()); err != nil {
				return childDone{title: "save extra-keys", err: err}
			}
			reloadErr := termuxconf.ReloadSettings()
			return childDone{title: "saved · reloaded", err: reloadErr,
				after: func(m *App, e error) tea.Cmd {
					m.ToastInfo("Show/hide the row: long-press the keyboard button or Vol-up+Q")
					return nil
				}}
		}
	}
	return nil
}

func setOrCleanDefault(doc *termuxconf.File, key, val, def string) {
	if val == def {
		if doc.Has(key) && doc.Get(key, "") == def {
			return
		}
		if doc.Has(key) {
			doc.Set(key, val)
		}
		return
	}
	doc.Set(key, val)
}

func (s *ekScreen) View() string {
	w := clampW(s.app.width, 40)

	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Preview") + "\n")
	b.WriteString(s.previewView() + "\n\n")

	b.WriteString(ui.TitleStyle.Render("Grid editor") + "\n")
	b.WriteString(s.gridView() + "\n\n")

	b.WriteString(ui.TitleStyle.Render("Companion settings") + "\n")
	b.WriteString("extra-keys-style: " + strings.ReplaceAll(s.styleRadio.View(), "\n", "  ") + "\n")
	b.WriteString("all-caps labels: " + s.caps.View() + "\n\n")

	bar := "arrows move · enter edit cell · + add key · - del key · R add row · X del row · p presets · S save · E raw file"
	b.WriteString(ui.SubtleStyle.Render(bar) + "\n")

	if termuxTooOld() {
		b.WriteString(ui.WarnStyle.Render("popups need Termux v0.95+ (found "+os.Getenv("TERMUX_VERSION")+") — saving still works") + "\n")
	}
	for _, e := range s.errorsShown {
		b.WriteString(ui.BadStyle.Render("✖ "+e.Error()) + "\n")
	}

	out := b.String()
	if s.presetOpen {
		var pb strings.Builder
		pb.WriteString(ui.TitleStyle.Render("Presets") + "\n")
		for i, p := range catalog.Presets {
			cursor := "  "
			label := p.Name
			if i == s.presetCur {
				cursor = "> "
				label = ui.SelStyle.Render(label)
			}
			pb.WriteString(cursor + label + "\n")
		}
		out = overlay(out, ui.BoxStyle.Width(clampI(w-10, 24, 80)).Render(pb.String()), w)
	}
	if s.editor != nil {
		out = overlay(out, s.editorView(clampI(w-8, 30, 100)), w)
	}
	if s.diffPager != nil {
		return overlay(out, s.diffPager.View(w), w) + "\n y apply & reload · n cancel\n"
	}
	if s.confirm != nil {
		out += "\n" + s.confirm.View(clampI(w-4, 30, 140)) + "\n"
	}
	return out
}

func termuxTooOld() bool {
	v := os.Getenv("TERMUX_VERSION")
	if v == "" {
		return false
	}
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return false
	}
	maj, e1 := strconv.Atoi(parts[0])
	min, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil {
		return false
	}
	return maj == 0 && min < 95
}

func (s *ekScreen) previewView() string {
	var lines []string
	for ri, row := range s.layout.Rows {
		var cells []string
		for ci, cell := range row {
			label := cellLabel(cell)
			badges := ""
			if cell.Popup.Has {
				badges += "P"
			}
			if cell.Kind == extrakeys.KindMacro {
				badges += "M"
			}
			if badges != "" {
				label += " " + ui.WarnStyle.Render(badges)
			}
			if ri == s.r && ci == s.c {
				label = ui.SelStyle.Render(label)
			} else if cell.Kind == extrakeys.KindNamed && extrakeys.Modifiers[cell.Value] {
				label = ui.KeyStyle.Render(label)
			}
			cells = append(cells, "[ "+label+" ]")
		}
		lines = append(lines, " "+strings.Join(cells, " "))
	}
	return strings.Join(lines, "\n")
}

func cellLabel(c extrakeys.Cell) string {
	if c.Display != "" {
		return c.Display
	}
	if c.Value == "" {
		return "·"
	}
	return c.Value
}

func (s *ekScreen) gridView() string {
	var lines []string
	for ri, row := range s.layout.Rows {
		var cells []string
		for ci := range row {
			cursor := " "
			if ri == s.r && ci == s.c {
				cursor = ui.KeyStyle.Render(">")
			}
			cells = append(cells, fmt.Sprintf("%s%d,%d", cursor, ri, ci))
		}
		lines = append(lines, strings.Join(cells, "  ")+fmt.Sprintf("   %s%d keys%s", ui.SubtleStyle.Render("("), len(row), ui.SubtleStyle.Render(")")))
	}
	return strings.Join(lines, "\n")
}

func (s *ekScreen) editorView(w int) string {
	e := s.editor
	focus := func(i int) bool { return e.field == i }
	line := func(i int, name, val string) string {
		cur := "  "
		if focus(i) {
			cur = ui.KeyStyle.Render("> ")
		}
		return cur + fmt.Sprintf("%-*s %s", 14, name+":", val)
	}

	kinds := []string{"literal character", "named key", "macro"}
	kv := make([]string, len(kinds))
	for i, k := range kinds {
		if i == e.kind {
			kv[i] = ui.SelStyle.Render(k)
		} else {
			kv[i] = k
		}
	}
	popups := []string{"none", "simple key", "macro"}
	pv := make([]string, len(popups))
	for i, k := range popups {
		if i == e.popupKind {
			pv[i] = ui.SelStyle.Render(k)
		} else {
			pv[i] = k
		}
	}

	var body strings.Builder
	body.WriteString(ui.TitleStyle.Render("Edit cell") + "\n")
	body.WriteString(line(0, "Type", strings.Join(kv, "  ")) + "\n")
	body.WriteString(line(1, "Value", e.value.View(focus(1))) + "\n")
	if focus(1) && e.kind == 1 {
		body.WriteString(ui.SubtleStyle.Render("    enter opens the named-key palette") + "\n")
	}
	body.WriteString(line(2, "Display label", e.display.View(focus(2))+" "+ui.SubtleStyle.Render("(optional, macros)")) + "\n")
	body.WriteString(line(3, "Popup", strings.Join(pv, "  ")) + "\n")
	if e.popupKind > 0 {
		body.WriteString(line(4, "Popup value", e.popupValue.View(focus(4))) + "\n")
		if e.popupKind == 2 {
			body.WriteString(line(5, "Popup label", e.popupDisplay.View(focus(5))) + "\n")
		}
	}
	body.WriteString("\n")
	body.WriteString(ui.SubtleStyle.Render("ctrl+s save cell · esc cancel · enter opens palette for named keys") + "\n")

	if e.paletteOpen {
		names := flatPalette()
		start := clampI(e.paletteCur-8, 0, maxI(0, len(names)-16))
		end := clampI(start+16, start, len(names))
		var pal strings.Builder
		pal.WriteString(ui.TitleStyle.Render("Named keys") + "\n")
		for i := start; i < end; i++ {
			n := names[i]
			if n.group != "" {
				pal.WriteString(" " + ui.SubtleStyle.Render("— "+n.group) + "\n")
				continue
			}
			cur := "  "
			label := n.name
			if i == e.paletteCur {
				cur = "> "
				label = ui.SelStyle.Render(label)
			}
			pal.WriteString(cur + label + "\n")
		}
		return ui.BoxStyle.Width(clampI(w-6, 28, 96)).Render(body.String()) + "\n" +
			ui.BoxStyle.Width(clampI(w-6, 20, 40)).Render(pal.String())
	}
	return ui.BoxStyle.Width(clampI(w-6, 28, 96)).Render(body.String())
}
