package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Theblackcat98/termux-ui/internal/catalog"
)

func jumpToCommonApps(t *testing.T, a *App) *commonAppsScreen {
	t.Helper()
	drainCmds(t, a, []tea.Cmd{a.jumpTo("8")})
	cur := a.current()
	cs, ok := cur.(*commonAppsScreen)
	if !ok {
		t.Fatalf("screen 8 is %T, want *commonAppsScreen", cur)
	}
	return cs
}

func TestCommonAppsViewListsCatalog(t *testing.T) {
	a := newTestApp(t)
	cs := jumpToCommonApps(t, a)

	// At the default 30-row test size the later sections sit below the fold.
	top := cs.View()
	for _, want := range []string{"Common Apps", "Essentials", "Neovim", "no core needed"} {
		if !strings.Contains(top, want) {
			t.Fatalf("common apps view missing %q:\n%s", want, top)
		}
	}
	if strings.Contains(top, "Fun & Flair") {
		t.Fatalf("later sections should stay below the fold until scrolled:\n%s", top)
	}

	// Scroll to the last row: the fun section becomes visible.
	for i := 0; i < 40; i++ {
		a.Update(keyMsg("down"))
	}
	bottom := cs.View()
	for _, want := range []string{"Fun & Flair", "Cowsay"} {
		if !strings.Contains(bottom, want) {
			t.Fatalf("scrolled view missing %q:\n%s", want, bottom)
		}
	}
}

func TestCommonAppsSelectSectionAndClear(t *testing.T) {
	a := newTestApp(t)
	cs := jumpToCommonApps(t, a)

	// Cursor starts on the first app; space selects it.
	if _, cmd := a.Update(keyMsg(" ")); cmd != nil {
		t.Fatal("space should not schedule a command")
	}
	if !strings.Contains(cs.View(), "1 selected") {
		t.Fatalf("space did not select:\n%s", cs.View())
	}

	// 'a' selects the whole section the cursor is on (essentials: 8 apps).
	a.Update(keyMsg("a"))
	if !strings.Contains(cs.View(), "8 selected") {
		t.Fatalf("'a' did not select the section:\n%s", cs.View())
	}

	// 'n' clears everything.
	a.Update(keyMsg("n"))
	if !strings.Contains(cs.View(), "0 selected") {
		t.Fatalf("'n' did not clear selection:\n%s", cs.View())
	}
}

func TestCommonAppsCursorSkipsHeaders(t *testing.T) {
	a := newTestApp(t)
	cs := jumpToCommonApps(t, a)

	if pkg := cs.rows()[cs.cursor].app.Pkg; pkg != "git" {
		t.Fatalf("initial cursor on %q, want git", pkg)
	}
	// Move down through all 8 essentials + the header to Neovim.
	for i := 0; i < 8; i++ {
		a.Update(keyMsg("down"))
	}
	if pkg := cs.rows()[cs.cursor].app.Pkg; pkg != "neovim" {
		t.Fatalf("cursor after 8 downs on %q, want neovim", pkg)
	}
	// Moving up past the top clamps on the first app, never a header.
	for i := 0; i < 20; i++ {
		a.Update(keyMsg("up"))
	}
	if pkg := cs.rows()[cs.cursor].app.Pkg; pkg != "git" {
		t.Fatalf("cursor did not clamp on git, on %q", pkg)
	}
}

func TestCommonAppsInstallNeedsSelection(t *testing.T) {
	a := newTestApp(t)
	jumpToCommonApps(t, a)

	if cmd := a.HandleMsg(keyMsg("i")); cmd != nil {
		t.Fatal("install without selection must not schedule a command")
	}
	if a.toasts.Empty() {
		t.Fatal("install without selection should toast a hint")
	}
}

func TestCommonAppsUninstallConfirms(t *testing.T) {
	a := newTestApp(t)
	cs := jumpToCommonApps(t, a)

	a.Update(keyMsg(" ")) // select git
	a.Update(keyMsg("x"))
	if cs.confirm == nil {
		t.Fatal("x should open the confirm dialog")
	}
	view := cs.View()
	if !strings.Contains(view, "Uninstall 1 app?") || !strings.Contains(view, "pkg uninstall -y git") {
		t.Fatalf("confirm dialog wrong:\n%s", view)
	}

	// n cancels: no command scheduled, dialog gone.
	if cmd := a.HandleMsg(keyMsg("n")); cmd != nil {
		t.Fatal("cancel must not schedule a command")
	}
	if cs.confirm != nil {
		t.Fatal("cancel did not close the dialog")
	}

	// y confirms: the RunChild command is returned (never executed here).
	a.Update(keyMsg("x"))
	cmd := a.HandleMsg(keyMsg("y"))
	if cmd == nil {
		t.Fatal("confirm should return the pkg command")
	}
	if cs.confirm != nil {
		t.Fatal("confirm did not close the dialog")
	}
}

func TestCommonAppsStatusRendering(t *testing.T) {
	a := newTestApp(t)

	// A brand-new screen has no status yet: everything is pending.
	fresh := newCommonAppsScreen(a)
	if s := fresh.statusOf("git"); s != "…" {
		t.Fatalf("fresh screen status %q, want …", s)
	}

	cs := jumpToCommonApps(t, a)
	installed := map[string]bool{}
	for _, app := range catalog.AllCommonApps() {
		installed[app.Pkg] = false
	}
	installed["git"] = true
	a.HandleMsg(appsStatusMsg{installed: installed})

	view := cs.View()
	if !strings.Contains(view, "(1/8 installed)") {
		t.Fatalf("section header should count installed:\n%s", view)
	}
	if s := cs.statusOf("git"); s != "installed" {
		t.Fatalf("git status %q, want installed", s)
	}
	if s := cs.statusOf("nano"); s != "not installed" {
		t.Fatalf("nano status %q, want not installed", s)
	}
}
