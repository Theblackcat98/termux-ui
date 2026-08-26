// Package ui provides the shared TUI widgets used by every screen:
// menus, toggles, radios, text inputs, confirm dialogs, diff views,
// pagers and a toast notification queue.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	TitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7C78FF"})
	SubtleStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#909090"})
	GoodStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7A0"})
	WarnStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"})
	BadStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"})
	SelStyle    = lipgloss.NewStyle().Bold(true).Reverse(true)
	BoxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	KeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7C78FF"}).Bold(true)
)

// StatusText styles an install status value.
func StatusText(s string) string {
	switch s {
	case "installed":
		return GoodStyle.Render(s)
	case "not installed":
		return BadStyle.Render(s)
	default:
		return SubtleStyle.Render(s)
	}
}

// MenuItem is one selectable row of a menu.
type MenuItem struct {
	Label string // rendered text (already styled if desired)
	Desc  string // right-side or second-line description
	Data  any    // payload for the action handler
}

// Menu is a vertical list with a moving cursor.
type Menu struct {
	Items  []MenuItem
	Cursor int
	Offset int
	Height int
}

func NewMenu(items []MenuItem) *Menu { return &Menu{Items: items} }

func (m *Menu) Up() {
	if m.Cursor > 0 {
		m.Cursor--
	}
	m.clamp()
}
func (m *Menu) Down() {
	if m.Cursor < len(m.Items)-1 {
		m.Cursor++
	}
	m.clamp()
}

func (m *Menu) clamp() {
	if m.Height <= 0 {
		m.Offset = 0
		return
	}
	if m.Cursor < m.Offset {
		m.Offset = m.Cursor
	}
	if m.Cursor >= m.Offset+m.Height {
		m.Offset = m.Cursor - m.Height + 1
	}
}

func (m *Menu) View(width int) string {
	var b strings.Builder
	for i, it := range m.Items {
		if m.Height > 0 && (i < m.Offset || i >= m.Offset+m.Height) {
			continue
		}
		cursor := "  "
		label := it.Label
		if i == m.Cursor {
			cursor = "> "
			label = SelStyle.Render(Strip(it.Label))
		}
		line := cursor + label
		if it.Desc != "" && i != m.Cursor {
			line += "  " + SubtleStyle.Render(it.Desc)
		} else if it.Desc != "" {
			line += "  " + it.Desc
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// Strip removes ANSI escape sequences from s (used to measure plain width).
func Strip(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Truncate shortens s to max cells, appending an ellipsis when cut.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(Strip(s))
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// Toggle is a boolean form control.
type Toggle struct {
	Value bool
}

func (t Toggle) View() string {
	if t.Value {
		return GoodStyle.Render("[x] on")
	}
	return SubtleStyle.Render("[ ] off")
}

func (t *Toggle) Flip() { t.Value = !t.Value }

// Radio is a single-choice form control.
type Radio struct {
	Options []string
	Index   int
}

func (r Radio) View() string {
	var rows []string
	for i, o := range r.Options {
		if i == r.Index {
			rows = append(rows, SelStyle.Render("(o) "+o))
		} else {
			rows = append(rows, SubtleStyle.Render("( ) "+o))
		}
	}
	return strings.Join(rows, "\n")
}

func (r *Radio) Left() {
	if r.Index > 0 {
		r.Index--
	}
}
func (r *Radio) Right() {
	if r.Index < len(r.Options)-1 {
		r.Index++
	}
}
func (r *Radio) Value() string { return r.Options[r.Index] }
func (r *Radio) Set(value string) {
	for i, o := range r.Options {
		if o == value {
			r.Index = i
			return
		}
	}
	r.Index = 0
}

// TextInput is a single-line editor with optional masking and validation.
type TextInput struct {
	Value  []rune
	Cursor int
	Masked bool
	Err    string
	Width  int
}

func (t *TextInput) Insert(rs ...rune) {
	t.Value = append(t.Value[:t.Cursor], append(rs, t.Value[t.Cursor:]...)...)
	t.Cursor += len(rs)
}

func (t *TextInput) Backspace() {
	if t.Cursor > 0 {
		t.Value = append(t.Value[:t.Cursor-1], t.Value[t.Cursor:]...)
		t.Cursor--
	}
}

func (t *TextInput) Delete() {
	if t.Cursor < len(t.Value) {
		t.Value = append(t.Value[:t.Cursor], t.Value[t.Cursor+1:]...)
	}
}

func (t *TextInput) Left() {
	if t.Cursor > 0 {
		t.Cursor--
	}
}
func (t *TextInput) Right() {
	if t.Cursor < len(t.Value) {
		t.Cursor++
	}
}

func (t TextInput) String() string { return string(t.Value) }

func (t *TextInput) Set(s string) {
	t.Value = []rune(s)
	t.Cursor = len(t.Value)
}

func (t TextInput) View(focused bool) string {
	shown := t.Value
	if t.Masked {
		mask := make([]rune, len(shown))
		for i := range mask {
			mask[i] = '*'
		}
		shown = mask
	}
	out := string(shown)
	if focused {
		if t.Err != "" {
			return BadStyle.Render("✗ "+out+"▏") + " " + BadStyle.Render(t.Err)
		}
		if t.Cursor >= len(shown) {
			return out + "▏"
		}
		r := []rune(out)
		return string(r[:t.Cursor]) + SelStyle.Render(string(r[t.Cursor])) + string(r[t.Cursor+1:])
	}
	if t.Err != "" {
		return out + " " + BadStyle.Render(t.Err)
	}
	return out
}

// Confirm is a modal yes/no dialog. Result: 0 open, 1 confirmed, -1 cancelled.
type Confirm struct {
	Title   string
	Detail  string
	Action  string
	FocusNo bool
	Result  int
}

func NewConfirm(title, detail, action string) *Confirm {
	return &Confirm{Title: title, Detail: detail, Action: action}
}

func (c *Confirm) Update(key string) {
	switch key {
	case "left", "h", "shift+tab":
		c.FocusNo = !c.FocusNo
	case "right", "l", "tab":
		c.FocusNo = !c.FocusNo
	case "y", "Y":
		c.Result = 1
	case "n", "N", "esc":
		c.Result = -1
	case "enter":
		if c.FocusNo {
			c.Result = -1
		} else {
			c.Result = 1
		}
	}
}

func (c *Confirm) View(width int) string {
	w := width - 4
	if w < 20 {
		w = 20
	}
	noBtn, yesBtn := SubtleStyle.Render(" no "), KeyStyle.Render(" yes ")
	if c.FocusNo {
		noBtn = SelStyle.Render(" no ")
		yesBtn = SubtleStyle.Render(" yes ")
	}
	body := c.Title + "\n\n" + c.Detail + "\n\n" + yesBtn + "   " + noBtn
	if c.Action != "" {
		body += "\n\n" + SubtleStyle.Render(c.Action)
	}
	return BoxStyle.Width(w).Render(body)
}

// Pager is a read-only scrollable view used for logs, docs and diffs.
type Pager struct {
	Lines  []string
	Offset int
	Height int
	Title  string
}

func NewPager(title, content string) *Pager {
	lines := strings.Split(content, "\n")
	return &Pager{Lines: lines, Title: title, Height: 20}
}

func (p *Pager) Up() {
	if p.Offset > 0 {
		p.Offset--
	}
}
func (p *Pager) Down() {
	if p.Offset < len(p.Lines)-p.Height && p.Offset < len(p.Lines)-1 {
		p.Offset++
	}
}
func (p *Pager) Top() { p.Offset = 0 }
func (p *Pager) Bottom() {
	max := len(p.Lines) - p.Height
	if max < 0 {
		max = 0
	}
	p.Offset = max
}

func (p Pager) View(width int) string {
	h := p.Height
	end := p.Offset + h
	if end > len(p.Lines) {
		end = len(p.Lines)
	}
	visible := p.Lines[p.Offset:end]
	var b strings.Builder
	b.WriteString(pagerHeader(p.Title, len(p.Lines), p.Offset, h, width))
	for _, l := range visible {
		b.WriteString(Truncate(l, width) + "\n")
	}
	return b.String()
}

func pagerHeader(title string, total, offset, height, width int) string {
	pos := fmt.Sprintf("%d-%d / %d", offset+1, min(offset+height, total), total)
	header := fmt.Sprintf("%s   %s", TitleStyle.Render(title), SubtleStyle.Render(pos+"  ↑↓ scroll · pgup/pgdn fast · esc close"))
	return header + "\n" + strings.Repeat("─", clampInt(width, 10, 200)) + "\n"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Toasts is a transient message queue shown at the bottom of the screen.
type Toasts struct {
	items []toast
}

type toast struct {
	text  string
	style lipgloss.Style
}

func (t *Toasts) Info(s string)  { t.items = append(t.items, toast{s, SubtleStyle}) }
func (t *Toasts) Good(s string)  { t.items = append(t.items, toast{"✔ " + s, GoodStyle}) }
func (t *Toasts) Warn(s string)  { t.items = append(t.items, toast{"⚠ " + s, WarnStyle}) }
func (t *Toasts) Error(s string) { t.items = append(t.items, toast{"✖ " + s, BadStyle}) }

// Tick ages the queue (called on every animation tick); returns true while
// messages remain.
func (t *Toasts) ExpireOldest() bool {
	if len(t.items) == 0 {
		return false
	}
	t.items = t.items[1:]
	return len(t.items) > 0
}

func (t *Toasts) View() string {
	var lines []string
	for _, it := range t.items {
		lines = append(lines, it.style.Render(Truncate(it.text, 100)))
	}
	return strings.Join(lines, "\n")
}

func (t *Toasts) Empty() bool { return len(t.items) == 0 }
