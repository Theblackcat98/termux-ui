package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/extrakeys"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	os.MkdirAll(filepath.Join(dir, ".termux"), 0o700)
	a := New()
	a.HandleMsg(tea.WindowSizeMsg{Width: 100, Height: 30})
	return a
}

func drainCmds(t *testing.T, a *App, cmds []tea.Cmd) {
	t.Helper()
	for i := 0; i < 8 && len(cmds) > 0; i++ {
		var next []tea.Cmd
		for _, c := range cmds {
			if c == nil {
				continue
			}
			switch m := c().(type) {
			case tickMsg:
			default:
				if cmd := a.HandleMsg(m); cmd != nil {
					next = append(next, cmd)
				}
			}
		}
		cmds = next
	}
}

func TestDashboardViewShowsStatus(t *testing.T) {
	a := newTestApp(t)
	view := a.View()
	for _, want := range []string{"termux-ui", "Termux version", "Storage permission", "Quick actions", "Modules & Packages"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard missing %q:\n%s", want, view)
		}
	}
}

func TestModulesScreenLoadsLiveCoreList(t *testing.T) {
	a := newTestApp(t)
	cmd := a.jumpTo("2")
	drainCmds(t, a, []tea.Cmd{cmd})
	ms := a.current().(*modulesScreen)
	rows, moduleLevel := ms.rows(ms.currentModule())
	if moduleLevel || len(rows) == 0 {
		t.Fatalf("expected tool rows for lang module, got %d rows (moduleLevel=%v)", len(rows), moduleLevel)
	}
	view := a.View()
	if !strings.Contains(view, "lang") || !strings.Contains(view, "install") {
		t.Fatalf("modules view incomplete:\n%.400s", view)
	}
}

func TestSettingsScreenFormAndDiff(t *testing.T) {
	a := newTestApp(t)
	cmd := a.jumpTo("3")
	drainCmds(t, a, []tea.Cmd{cmd})
	s := a.current().(*settingsScreen)

	view := a.View()
	if !strings.Contains(view, "General") || !strings.Contains(view, "volume-keys") {
		t.Fatalf("settings view wrong:\n%.300s", view)
	}

	vol := s.fields[0][5]
	if vol.def.Key != "volume-keys" {
		t.Fatalf("schema drift: %s", vol.def.Key)
	}
	vol.radio.Index = 1 // volume
	doc, err := s.buildDoc()
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Get("volume-keys", ""); got != "volume" {
		t.Fatalf("doc not updated: %q", got)
	}
	if !strings.Contains(a.View(), "Modified: 1") {
		t.Fatalf("modified count not shown")
	}
	// Reset removes nothing.
	doc2, _ := s.buildDoc()
	applyFieldToDoc(doc2, vol.def, "virtual")
	if doc2.Has("volume-keys") {
		t.Fatalf("default reset must not write anything new")
	}
}

func TestExtraKeysScreenSaveFlow(t *testing.T) {
	a := newTestApp(t)
	cmd := a.jumpTo("4")
	drainCmds(t, a, []tea.Cmd{cmd})
	e := a.current().(*ekScreen)

	os.WriteFile(e.path, []byte("# existing config\nfullscreen=true\n"), 0o600)

	e.layout = cloneLayout(presetLayoutForTest(t, "Default (2 rows)"))
	if err := validateOrDie(e.layout); err != nil {
		t.Fatal(err)
	}
	e.beginSave(a)
	if e.diffPager == nil {
		t.Fatal("diff review did not open (nothing to change?)")
	}
	drainCmds(t, a, []tea.Cmd{e.diffCommit(a)})

	raw, err := os.ReadFile(e.path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "extra-keys=") || !strings.Contains(text, "\\") {
		t.Fatalf("extra-keys not written multiline:\n%s", text)
	}
	doc := parseForTest(t, text)
	got := doc.Get("extra-keys", "")
	l, err := extrakeys.Parse(got)
	if err != nil {
		t.Fatalf("written value does not parse: %v (%q)", err, got)
	}
	want := extrakeys.Serialize(cloneLayout(presetLayoutForTest(t, "Default (2 rows)")))
	if extrakeys.Serialize(l) != want {
		t.Fatalf("round trip changed bytes:\n%s\n%s", extrakeys.Serialize(l), want)
	}
	if bak := newestBackup(t, filepath.Dir(e.path), "termux.properties.bak."); bak == "" {
		t.Fatalf("no timestamped backup written")
	}
}

func TestThemeScreenRendersGallery(t *testing.T) {
	a := newTestApp(t)
	cmd := a.jumpTo("5")
	drainCmds(t, a, []tea.Cmd{cmd})
	view := a.View()
	for _, want := range []string{"Nord", "Dracula", "Preview"} {
		if !strings.Contains(view, want) {
			t.Fatalf("theme gallery missing %q:\n%.500s", want, view)
		}
	}
}

func TestShellAndAppsScreensRender(t *testing.T) {
	a := newTestApp(t)
	cmd := a.jumpTo("6")
	drainCmds(t, a, []tea.Cmd{cmd})
	if v := a.View(); !strings.Contains(v, "ZSH stack") {
		t.Fatalf("shell screen broken:\n%.300s", v)
	}
	cmd = a.jumpTo("7")
	drainCmds(t, a, []tea.Cmd{cmd})
	if v := a.View(); !strings.Contains(v, "S1 Storage") || !strings.Contains(v, "F-Droid") {
		t.Fatalf("apps screen broken:\n%.400s", v)
	}
}

func TestHelpOverlayAndQuitKeys(t *testing.T) {
	a := newTestApp(t)
	a.HandleMsg(keyMsg("?"))
	if !a.showHelp {
		t.Fatal("? should open help overlay")
	}
	a.HandleMsg(keyMsg("esc"))
	if a.showHelp {
		t.Fatal("esc should close help overlay")
	}
}

func keyMsg(k string) tea.KeyMsg {
	switch k {
	case "?":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
}

func presetLayoutForTest(t *testing.T, name string) *extrakeys.Layout {
	t.Helper()
	for _, p := range catalogPresets() {
		if p.Name == name {
			return p.Layout
		}
	}
	t.Fatalf("preset %q not found", name)
	return nil
}

func validateOrDie(l *extrakeys.Layout) error { return joinErrors(extrakeys.Validate(l)) }
