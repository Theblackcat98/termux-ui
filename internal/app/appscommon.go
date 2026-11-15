package app

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Theblackcat98/termux-ui/internal/catalog"
	"github.com/Theblackcat98/termux-ui/internal/ui"
)

// appsStatusMsg carries dpkg-backed installed flags for the catalog packages.
type appsStatusMsg struct {
	installed map[string]bool
}

// fetchAppsStatusCmd asks dpkg about every catalog package. Unlike core tools
// this works on a fresh install: dpkg and pkg are part of the bootstrap.
func fetchAppsStatusCmd(pkgs []string) tea.Cmd {
	return func() tea.Msg {
		installed := make(map[string]bool, len(pkgs))
		for _, p := range pkgs {
			installed[p] = pkgInstalled(p)
		}
		return appsStatusMsg{installed: installed}
	}
}

// appRow is one renderable line on the common apps screen: a section header
// or a single package row.
type appRow struct {
	header bool
	cat    catalog.AppCategory
	app    catalog.CommonApp
}

type commonAppsScreen struct {
	app    *App
	cursor int // index into rows(); always positioned on a non-header row

	status  map[string]bool // pkg -> dpkg -s result
	loading bool

	checked map[string]bool // pkg -> user selection
	confirm *ui.Confirm
	pending func(a *App) tea.Cmd
}

func newCommonAppsScreen(a *App) *commonAppsScreen {
	s := &commonAppsScreen{
		app:     a,
		status:  map[string]bool{},
		checked: map[string]bool{},
	}
	s.moveCursor(1) // land on the first app row, past the first header
	return s
}

func (s *commonAppsScreen) Title() string { return "Common Apps" }
func (s *commonAppsScreen) WantsGlobalKeys() bool {
	return s.confirm == nil
}

// rows builds the flat header+app display list.
func (s *commonAppsScreen) rows() []appRow {
	var rows []appRow
	for _, cat := range catalog.AppCategories {
		rows = append(rows, appRow{header: true, cat: cat})
		for _, a := range cat.Apps {
			rows = append(rows, appRow{cat: cat, app: a})
		}
	}
	return rows
}

// moveCursor steps the cursor by delta rows, skipping header rows. At the
// edges it stops on the first/last selectable row.
func (s *commonAppsScreen) moveCursor(delta int) {
	rows := s.rows()
	if len(rows) == 0 {
		return
	}
	cur := clampI(s.cursor, 0, len(rows)-1)
	i := cur
	for {
		next := i + delta
		if next < 0 || next >= len(rows) {
			break
		}
		i = next
		if !rows[i].header {
			s.cursor = i
			return
		}
	}
	s.cursor = cur
}

func (s *commonAppsScreen) checkedPkgs() []string {
	var pkgs []string
	for _, r := range s.rows() {
		if !r.header && s.checked[r.app.Pkg] {
			pkgs = append(pkgs, r.app.Pkg)
		}
	}
	return pkgs
}

func (s *commonAppsScreen) Update(a *App, msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case refreshMsg:
		s.loading = true
		var pkgs []string
		for _, c := range catalog.AppCategories {
			for _, ap := range c.Apps {
				pkgs = append(pkgs, ap.Pkg)
			}
		}
		return fetchAppsStatusCmd(pkgs)

	case appsStatusMsg:
		s.loading = false
		s.status = t.installed
		return nil

	case tea.KeyMsg:
		if s.confirm != nil {
			s.confirm.Update(t.String())
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

		switch t.String() {
		case "up", "k":
			s.moveCursor(-1)
		case "down", "j":
			s.moveCursor(1)
		case " ", "enter":
			rows := s.rows()
			if s.cursor < len(rows) && !rows[s.cursor].header {
				pkg := rows[s.cursor].app.Pkg
				s.checked[pkg] = !s.checked[pkg]
			}
		case "a":
			// Select every app in the section the cursor is on.
			rows := s.rows()
			if s.cursor < len(rows) {
				for _, r := range rows {
					if !r.header && r.cat.Key == rows[s.cursor].cat.Key {
						s.checked[r.app.Pkg] = true
					}
				}
			}
		case "n":
			s.checked = map[string]bool{}

		case "i":
			return s.runPkg(a, "install", false)
		case "u":
			return s.runPkg(a, "upgrade", false)
		case "x":
			return s.runPkg(a, "uninstall", true)
		}
	}
	return nil
}

// runPkg builds one batched pkg invocation over the checked packages.
// Uninstall is destructive and goes through the confirm dialog first;
// install and upgrade run directly, like screen 2 does.
func (s *commonAppsScreen) runPkg(a *App, verb string, needsConfirm bool) tea.Cmd {
	pkgs := s.checkedPkgs()
	if len(pkgs) == 0 {
		a.ToastWarn("select apps with space first")
		return nil
	}
	args := append([]string{verb, "-y"}, pkgs...)
	cmdline := "pkg " + strings.Join(args, " ")
	run := func(m *App) tea.Cmd {
		return m.RunChild(fmt.Sprintf("pkg %s (%d app%s)", verb, len(pkgs), plural(len(pkgs))),
			exec.Command("pkg", args...),
			func(m *App, err error) tea.Cmd {
				if err != nil {
					m.ToastError("pkg " + verb + " failed — output above")
				}
				return m.current().Update(m, refreshMsg{})
			})
	}
	if needsConfirm {
		s.confirm = ui.NewConfirm(
			strings.ToUpper(verb[:1])+verb[1:]+" "+fmt.Sprintf("%d app%s?", len(pkgs), plural(len(pkgs))),
			"Runs: "+cmdline,
			"y confirm · n cancel",
		)
		s.pending = run
		return nil
	}
	return run(a)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (s *commonAppsScreen) statusOf(pkg string) string {
	if s.loading {
		return "…"
	}
	if v, ok := s.status[pkg]; ok {
		if v {
			return "installed"
		}
		return "not installed"
	}
	return "…"
}

func (s *commonAppsScreen) catInstalled(cat catalog.AppCategory) int {
	n := 0
	for _, a := range cat.Apps {
		if s.status[a.Pkg] {
			n++
		}
	}
	return n
}

func (s *commonAppsScreen) View() string {
	w := clampW(s.app.width, 40)
	h := clampI(s.app.height-10, 6, 40)
	rows := s.rows()

	out := ui.TitleStyle.Render("Common Apps") + "  " +
		ui.SubtleStyle.Render(fmt.Sprintf("%d selected", len(s.checkedPkgs()))) + "\n" +
		ui.SubtleStyle.Render(strings.Repeat("─", w)) + "\n"

	// Window the flat list to the viewport height, cursor visible.
	start := clampI(s.cursor-h+1, 0, maxI(0, len(rows)-h))
	for i := start; i < len(rows) && i < start+h; i++ {
		r := rows[i]
		if r.header {
			out += uiTrunc(ui.TitleStyle.Render(fmt.Sprintf("%s (%d/%d installed)", r.cat.Name, s.catInstalled(r.cat), len(r.cat.Apps))), w) + "\n"
			continue
		}
		box := "[ ] "
		if s.checked[r.app.Pkg] {
			box = ui.GoodStyle.Render("[x] ")
		}
		name := r.app.Name
		if i == s.cursor {
			name = ui.SelStyle.Render(name)
		}
		line := box + name
		detail := r.app.Pkg
		if r.app.Command != "" {
			detail += " (" + r.app.Command + ")"
		}
		line += "  " + ui.SubtleStyle.Render(detail+" — "+r.app.Summary)
		line += "  " + ui.StatusText(s.statusOf(r.app.Pkg))
		out += uiTrunc(line, w) + "\n"
	}

	bar := "space toggle · a all in section · n none · i install · u upgrade · x uninstall — uses pkg, no core needed"
	body := out + "\n" + ui.SubtleStyle.Render(strings.Repeat("─", w)) + "\n" + bar + "\n"
	if s.confirm != nil {
		body += "\n" + s.confirm.View(w) + "\n"
	}
	return body
}
