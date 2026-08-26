// Package extrakeys models Termux extra-keys layouts (rows of keys with
// popups and macros), enforces Termux constraints and serializes to the
// canonical multiline backslash-continued form written into
// termux.properties.
package extrakeys

import (
	"fmt"
	"strings"
)

// CellKind distinguishes the three kinds of key definitions.
type CellKind int

const (
	KindLiteral CellKind = iota // a printable character, e.g. '/'
	KindNamed                   // a palette name, e.g. ESC, CTRL, F5
	KindMacro                   // space-separated token sequence, e.g. "CTRL f d"
)

// Popup is an optional long-press action attached to a cell.
type Popup struct {
	Has     bool
	Kind    CellKind
	Value   string
	Display string // label shown for macro popups
}

// Cell is one key slot in the grid.
type Cell struct {
	Kind    CellKind
	Value   string
	Display string // optional custom label (macros mainly)
	Popup   Popup
}

// Layout is a full grid. Rows may be ragged (Termux allows variable widths).
type Layout struct {
	Rows [][]Cell
}

// MaxRows / MaxCols are usability caps enforced before save.
const (
	MaxRows = 6
	MaxCols = 12
)

// NamedKey groups for the palette UI.
var NamedKeyGroups = []struct {
	Title string
	Keys  []string
}{
	{"Modifiers", []string{"CTRL", "ALT", "FN", "SHIFT"}},
	{"Navigation & editing", []string{"SCROLL", "SPACE", "ESC", "TAB", "HOME", "END", "PGUP", "PGDN", "INS", "DEL", "BKSP", "UP", "DOWN", "LEFT", "RIGHT", "ENTER"}},
	{"Symbols", []string{"BACKSLASH", "QUOTE", "APOSTROPHE"}},
	{"Function keys", []string{"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12"}},
	{"Actions", []string{"KEYBOARD", "DRAWER"}},
}

// Modifiers may each appear at most once in the entire layout.
var Modifiers = map[string]bool{"CTRL": true, "ALT": true, "FN": true, "SHIFT": true}

// Lit builds a literal-character cell.
func Lit(s string) Cell { return Cell{Kind: KindLiteral, Value: s} }

// Named builds a named-key cell.
func Named(s string) Cell { return Cell{Kind: KindNamed, Value: s} }

// Macro builds a macro cell with optional display label.
func Macro(value, display string) Cell { return Cell{Kind: KindMacro, Value: value, Display: display} }

// WithPopup attaches a popup to a cell.
func WithPopup(c Cell, p Popup) Cell { c.Popup = p; return c }

// SimplePopup makes a popup from a scalar key value (literal or named).
func SimplePopup(v string) Popup {
	k := Classify(v)
	return Popup{Has: true, Kind: k.Kind, Value: v}
}

// MacroPopup makes a popup that runs a macro sequence.
func MacroPopup(value, display string) Popup {
	return Popup{Has: true, Kind: KindMacro, Value: value, Display: display}
}

// Classify maps a plain scalar to a Cell kind: known palette names become
// named cells, multi-token strings become macros, else literal.
func Classify(s string) Cell {
	if IsNamedKey(s) {
		return Named(s)
	}
	if len([]rune(s)) > 1 {
		return Macro(s, "")
	}
	return Lit(s)
}

// IsNamedKey reports whether s is a known palette key name.
func IsNamedKey(s string) bool {
	for _, g := range NamedKeyGroups {
		for _, k := range g.Keys {
			if k == s {
				return true
			}
		}
	}
	return false
}

// Validate checks all save-blocking constraints. Returned errors reference
// row/column positions (0-based) so the UI can highlight offenders.
func Validate(l *Layout) []error {
	var errs []error
	if len(l.Rows) == 0 {
		errs = append(errs, fmt.Errorf("layout needs at least one row"))
		return errs
	}
	if len(l.Rows) > MaxRows {
		errs = append(errs, fmt.Errorf("too many rows: %d (max %d)", len(l.Rows), MaxRows))
	}
	modSeen := map[string][2]int{}
	for r, row := range l.Rows {
		if len(row) < 1 {
			errs = append(errs, fmt.Errorf("row %d needs at least 1 key", r))
		}
		if len(row) > MaxCols {
			errs = append(errs, fmt.Errorf("row %d has %d keys (max %d)", r, len(row), MaxCols))
		}
		for c, cell := range row {
			switch cell.Kind {
			case KindLiteral:
				if cell.Value == "" {
					errs = append(errs, fmt.Errorf("row %d col %d: empty key", r, c))
				}
				if strings.ContainsRune(cell.Value, '\\') {
					errs = append(errs, fmt.Errorf("row %d col %d: backslash not allowed as a key (use BACKSLASH)", r, c))
				}
				if n := len([]rune(cell.Value)); n > 1 {
					errs = append(errs, fmt.Errorf("row %d col %d: literal keys must be a single character", r, c))
				}
			case KindNamed:
				if !IsNamedKey(cell.Value) {
					errs = append(errs, fmt.Errorf("row %d col %d: unknown key name %q", r, c, cell.Value))
				}
			case KindMacro:
				if strings.TrimSpace(cell.Value) == "" {
					errs = append(errs, fmt.Errorf("row %d col %d: macro needs content", r, c))
				}
			}
			check := func(p Popup, where string) {
				if !p.Has {
					return
				}
				if p.Kind == KindLiteral && strings.ContainsRune(p.Value, '\\') {
					errs = append(errs, fmt.Errorf("row %d col %d: popup backslash not allowed (use BACKSLASH)", r, c))
				}
				if p.Kind == KindNamed && Modifiers[p.Value] {
					trackModifier(p.Value, modSeen, r, c, &errs)
				}
			}
			if cell.Kind == KindNamed && Modifiers[cell.Value] {
				trackModifier(cell.Value, modSeen, r, c, &errs)
			}
			check(cell.Popup, "popup")
		}
	}
	return errs
}

func trackModifier(name string, seen map[string][2]int, r, c int, errs *[]error) {
	if prev, dup := seen[name]; dup {
		*errs = append(*errs, fmt.Errorf("%s appears twice: row %d col %d and row %d col %d (modifiers may appear only once)", name, prev[0], prev[1], r, c))
	} else {
		seen[name] = [2]int{r, c}
	}
}

// Serialize renders the layout as the canonical Termux property value:
// rows separated by " \\\n" continuations, wrapped in outer brackets.
func Serialize(l *Layout) string {
	var rows []string
	for _, row := range l.Rows {
		cells := make([]string, len(row))
		for i, cell := range row {
			cells[i] = serializeCell(cell)
		}
		rows = append(rows, "["+strings.Join(cells, ",")+"]")
	}
	return "[" + strings.Join(rows, ",") + "]"
}

func serializePopup(p Popup) string {
	if p.Kind == KindMacro {
		s := "{macro:" + quote(p.Value)
		if p.Display != "" {
			s += ",display:" + quote(p.Display)
		}
		return s + "}"
	}
	return quote(p.Value)
}

func serializeCell(c Cell) string {
	var base string
	switch c.Kind {
	case KindMacro:
		base = "{macro:" + quote(c.Value)
		if c.Display != "" {
			base += ",display:" + quote(c.Display)
		}
		base += "}"
	default:
		base = quote(c.Value)
	}
	if !c.Popup.Has {
		return base
	}
	if c.Kind != KindMacro {
		keyField := "key:" + base
		return "{" + keyField + ",popup:" + serializePopup(c.Popup) + "}"
	}
	// Macro cell with popup.
	return "{macro:" + quote(c.Value) + ",popup:" + serializePopup(c.Popup) + "}"
}

func quote(s string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(s) + "'"
}

// Parse decodes an extra-keys property value (lenient JSON: single or double
// quotes) back into a Layout so existing configs can be edited in place.
func Parse(value string) (*Layout, error) {
	p := &parser{s: strings.TrimSpace(value)}
	l := &Layout{}
	if err := p.expect('['); err != nil {
		return nil, err
	}
	for {
		row, err := p.parseRow()
		if err != nil {
			return nil, err
		}
		l.Rows = append(l.Rows, row)
		tok, err := p.next()
		if err != nil {
			return nil, err
		}
		if tok == "," {
			p.skipSpace()
			continue
		}
		if tok == "]" {
			break
		}
		return nil, fmt.Errorf("unexpected token %q", tok)
	}
	return l, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) skipSpace() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t' || p.s[p.i] == '\n' || p.s[p.i] == '\\' || p.s[p.i] == '\r') {
		// A lone continuation backslash followed by newline is skipped via \n;
		// bare trailing "\" also skipped.
		if p.s[p.i] == '\\' && p.i+1 < len(p.s) && p.s[p.i+1] != '\\' && p.s[p.i+1] != '\'' && p.s[p.i+1] != '"' {
			p.i++
			continue
		}
		p.i++
	}
}

func (p *parser) expect(ch byte) error {
	p.skipSpace()
	if p.i >= len(p.s) || p.s[p.i] != ch {
		return fmt.Errorf("expected %q at offset %d", ch, p.i)
	}
	p.i++
	return nil
}

// next returns the next structural token: one of [ ] , : or a full scalar
// (quoted string or bare word).
func (p *parser) next() (string, error) {
	p.skipSpace()
	if p.i >= len(p.s) {
		return "", fmt.Errorf("unexpected end of input")
	}
	switch c := p.s[p.i]; c {
	case '[', ']', '{', '}', ',', ':':
		p.i++
		return string(c), nil
	case '\'', '"':
		q := c
		p.i++
		var b strings.Builder
		for p.i < len(p.s) {
			ch := p.s[p.i]
			if ch == '\\' && p.i+1 < len(p.s) {
				b.WriteByte(p.s[p.i+1])
				p.i += 2
				continue
			}
			if ch == q {
				p.i++
				return b.String(), nil
			}
			b.WriteByte(ch)
			p.i++
		}
		return "", fmt.Errorf("unterminated string at offset %d", p.i)
	default:
		start := p.i
		for p.i < len(p.s) && !strings.ContainsRune("[],{}: \t\n\r\\'\"", rune(p.s[p.i])) {
			p.i++
		}
		if start == p.i {
			return "", fmt.Errorf("unexpected byte %q at offset %d", c, start)
		}
		return p.s[start:p.i], nil
	}
}

func (p *parser) parseRow() ([]Cell, error) {
	if err := p.expect('['); err != nil {
		return nil, err
	}
	var row []Cell
	for {
		cell, err := p.parseCell()
		if err != nil {
			return nil, err
		}
		row = append(row, cell)
		tok, err := p.next()
		if err != nil {
			return nil, err
		}
		if tok == "," {
			continue
		}
		if tok == "]" {
			return row, nil
		}
		return nil, fmt.Errorf("unexpected token %q in row", tok)
	}
}

// parseCell reads one scalar or object into a Cell.
func (p *parser) parseCell() (Cell, error) {
	p.skipSpace()
	if p.i < len(p.s) && p.s[p.i] == '{' {
		return p.parseObject()
	}
	tok, err := p.next()
	if err != nil {
		return Cell{}, err
	}
	return classify(tok), nil
}

// classifyLegacy is kept for the parser; see Classify.
func classify(s string) Cell { return Classify(s) }

// parseObject reads {field:value,...}. Recognized fields: key, macro,
// display, popup.
func (p *parser) parseObject() (Cell, error) {
	if err := p.expect('{'); err != nil {
		return Cell{}, err
	}
	var cell Cell
	for {
		field, err := p.next()
		if err != nil {
			return Cell{}, err
		}
		if t, err := p.next(); err != nil || t != ":" {
			return Cell{}, fmt.Errorf("expected ':' after field %q", field)
		}
		switch field {
		case "key":
			v, err := p.scalarOrObject()
			if err != nil {
				return Cell{}, err
			}
			cell.Kind = v.kind
			cell.Value = v.value
			cell.Display = v.display
		case "macro":
			v, err := p.scalarOrObject()
			if err != nil {
				return Cell{}, err
			}
			cell.Kind = KindMacro
			cell.Value = v.value
			if v.display != "" {
				cell.Display = v.display
			}
		case "display":
			v, err := p.scalarOrObject()
			if err != nil {
				return Cell{}, err
			}
			cell.Display = v.value
		case "popup":
			pp, err := p.parsePopup()
			if err != nil {
				return Cell{}, err
			}
			cell.Popup = pp
		default:
			return Cell{}, fmt.Errorf("unknown field %q", field)
		}
		tok, err := p.next()
		if err != nil {
			return Cell{}, err
		}
		if tok == "," {
			continue
		}
		if tok == "}" {
			if cell.Kind == 0 && cell.Value == "" && cell.Popup.Has {
				cell.Kind = KindLiteral
			}
			return cell, nil
		}
		return Cell{}, fmt.Errorf("unexpected token %q in object", tok)
	}
}

type scalar struct {
	value   string
	display string
	kind    CellKind
}

// scalarOrObject reads either a plain scalar or a {macro:...,display:...}
// nested object (used for key/popups that are macros).
func (p *parser) scalarOrObject() (scalar, error) {
	p.skipSpace()
	if p.i < len(p.s) && p.s[p.i] == '{' {
		cell, err := p.parseObject()
		if err != nil {
			return scalar{}, err
		}
		return scalar{value: cell.Value, display: cell.Display, kind: cell.Kind}, nil
	}
	tok, err := p.next()
	if err != nil {
		return scalar{}, err
	}
	c := classify(tok)
	return scalar{value: c.Value, kind: c.Kind}, nil
}

func (p *parser) parsePopup() (Popup, error) {
	v, err := p.scalarOrObject()
	if err != nil {
		return Popup{}, err
	}
	return Popup{Has: true, Kind: v.kind, Value: v.value, Display: v.display}, nil
}
