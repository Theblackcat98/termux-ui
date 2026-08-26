package app

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/catalog"
	"github.com/DevCoreXOfficial/termux-ui/internal/core"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

// listMsg carries parsed `core list <module>` output.
type listMsg struct {
	module string
	rows   []core.Tool
	err    error
}

// textMsg carries captured command/file content for the pager.
type textMsg struct {
	title   string
	content string
}

func fetchListCmd(c *core.CLI, module string) tea.Cmd {
	return func() tea.Msg {
		if !c.Installed() {
			return listMsg{module: module, err: fmt.Errorf("core not installed")}
		}
		rows, err := c.List(module)
		if err != nil {
			return listMsg{module: module, err: err}
		}
		return listMsg{module: module, rows: rows}
	}
}

func captureCmd(title string, cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			return textMsg{title: title, content: fmt.Sprintf("command failed: %v", err)}
		}
		content := string(out)
		if err != nil {
			content += "\n[exit status: non-zero — see log with L]"
		}
		return textMsg{title: title, content: content}
	}
}

type toolRow struct {
	tool    core.Tool
	checked bool
}

type modulesScreen struct {
	app     *App
	modMenu *ui.Menu
	cursor2 int // cursor within the right pane
	focusR  bool

	live    map[string][]core.Tool
	loading map[string]bool
	errs    map[string]string

	checked map[string]bool // "module|flag" -> user selection
	confirm *ui.Confirm
	pending func(a *App) tea.Cmd
	pager   *ui.Pager
}

func newModulesScreen(a *App) *modulesScreen {
	m := &modulesScreen{
		app:     a,
		checked: map[string]bool{},
		live:    map[string][]core.Tool{},
		loading: map[string]bool{},
		errs:    map[string]string{},
	}
	items := make([]ui.MenuItem, len(catalog.Modules))
	for i, mod := range catalog.Modules {
		items[i] = ui.MenuItem{Label: mod.Key + "  " + mod.Title, Desc: mod.Description, Data: i}
	}
	m.modMenu = ui.NewMenu(items)
	return m
}

func (m *modulesScreen) Title() string { return "Modules & Packages" }
func (m *modulesScreen) WantsGlobalKeys() bool {
	return m.confirm == nil && m.pager == nil
}

func (m *modulesScreen) currentModule() catalog.Module {
	i := clampI(m.modMenu.Cursor, 0, len(catalog.Modules)-1)
	return catalog.Modules[i]
}

// rows merges live parse results onto the static catalog.
func (m *modulesScreen) rows(mod catalog.Module) ([]toolRow, bool) {
	if mod.ModuleLevel {
		return nil, true
	}
	var tools []core.Tool
	for _, t := range mod.Tools {
		tools = append(tools, core.Tool{Name: t.Name, Flag: t.Flag, Command: t.Command})
	}
	if live, ok := m.live[mod.Key]; ok && len(live) > 0 {
		merged := make([]core.Tool, 0, len(live)+len(tools))
		seen := map[string]bool{}
		for _, t := range live {
			merged = append(merged, t)
			seen[t.Flag] = true
		}
		for _, t := range tools {
			if !seen[t.Flag] {
				merged = append(merged, core.Tool{Name: t.Name, Flag: t.Flag, Command: t.Command, Status: core.StatusUnknown})
			}
		}
		tools = merged
	}
	rows := make([]toolRow, len(tools))
	for i, t := range tools {
		rows[i] = toolRow{tool: t, checked: m.checked[mod.Key+"|"+t.Flag]}
	}
	return rows, false
}

func (m *modulesScreen) selectedFlags(mod catalog.Module) []string {
	rows, _ := m.rows(mod)
	var flags []string
	for _, r := range rows {
		if r.checked {
			flags = append(flags, r.tool.Flag)
		}
	}
	return flags
}

func (m *modulesScreen) Update(a *App, msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case refreshMsg:
		key := m.currentModule().Key
		m.loading[key] = true
		return fetchListCmd(a.core, key)

	case listMsg:
		m.loading[t.module] = false
		if t.err != nil {
			m.errs[t.module] = t.err.Error()
			if a.core.Installed() {
				a.ToastWarn(t.module + ": " + t.err.Error())
			}
			return nil
		}
		delete(m.errs, t.module)
		m.live[t.module] = t.rows
		return nil

	case textMsg:
		m.pager = ui.NewPager(t.title, t.content)
		m.pager.Height = clampI(a.height-8, 6, 40)
		return nil

	case tea.KeyMsg:
		if m.pager != nil {
			switch t.String() {
			case "up", "k":
				m.pager.Up()
			case "down", "j":
				m.pager.Down()
			case "pgup":
				m.pager.Offset = maxI(0, m.pager.Offset-m.pager.Height)
			case "pgdn":
				m.pager.Down()
				m.pager.Down()
			case "g", "home":
				m.pager.Top()
			case "G", "end":
				m.pager.Bottom()
			case "esc", "q", "enter":
				m.pager = nil
			}
			return nil
		}
		if m.confirm != nil {
			m.confirm.Update(t.String())
			if m.confirm.Result == 1 && m.pending != nil {
				fn := m.pending
				m.confirm, m.pending = nil, nil
				return fn(a)
			}
			if m.confirm.Result != 0 {
				m.confirm, m.pending = nil, nil
			}
			return nil
		}

		mod := m.currentModule()
		switch t.String() {
		case "tab", "right", "l":
			if !mod.ModuleLevel {
				m.focusR = true
			}
		case "left", "h":
			m.focusR = false
		case "up", "k":
			if m.focusR {
				if m.cursor2 > 0 {
					m.cursor2--
				}
			} else {
				m.modMenu.Up()
				return m.loadIfNeeded(a)
			}
		case "down", "j":
			if m.focusR {
				rows, _ := m.rows(mod)
				if m.cursor2 < len(rows)-1 {
					m.cursor2++
				}
			} else {
				m.modMenu.Down()
				return m.loadIfNeeded(a)
			}
		case "enter":
			if m.focusR {
				return m.toggleChecked(mod)
			}
			return m.loadIfNeeded(a)
		case " ":
			return m.toggleChecked(mod)

		case "a":
			rows, _ := m.rows(mod)
			for _, r := range rows {
				m.checked[mod.Key+"|"+r.tool.Flag] = true
			}
		case "n":
			rows, _ := m.rows(mod)
			for _, r := range rows {
				delete(m.checked, mod.Key+"|"+r.tool.Flag)
			}

		case "i":
			return m.runVerb(a, core.VerbInstall, false)
		case "u":
			return m.runVerb(a, core.VerbUpdate, false)
		case "x":
			m.confirmOp(a, core.VerbUninstall)
		case "r":
			m.confirmOp(a, core.VerbReinstall)
		case "d":
			flags := m.selectedFlags(mod)
			args := []string{"show", mod.Key}
			args = append(args, flags...)
			return captureCmd("core "+strings.Join(args, " "), exec.Command(a.core.Path, args...))
		case "o":
			return a.RunChild("core open "+mod.Key, exec.Command(a.core.Path, "open", mod.Key), nil)
		case "L":
			if data, ok := core.LogTail(mod.Key, 500); ok {
				m.pager = ui.NewPager(core.LogPath(mod.Key), data)
				m.pager.Height = clampI(a.height-8, 6, 40)
			} else {
				a.ToastWarn("no log yet for " + mod.Key)
			}
		}
	}
	return nil
}

func (m *modulesScreen) loadIfNeeded(a *App) tea.Cmd {
	key := m.currentModule().Key
	if _, done := m.live[key]; !done && !m.loading[key] {
		m.loading[key] = true
		return fetchListCmd(a.core, key)
	}
	return nil
}

func (m *modulesScreen) toggleChecked(mod catalog.Module) tea.Cmd {
	rows, _ := m.rows(mod)
	if m.cursor2 < len(rows) {
		flag := rows[m.cursor2].tool.Flag
		k := mod.Key + "|" + flag
		m.checked[k] = !m.checked[k]
	}
	return nil
}

func (m *modulesScreen) confirmOp(a *App, verb string) {
	mod := m.currentModule()
	flags := m.selectedFlags(mod)
	noun := describeOp(mod, flags)
	m.confirm = ui.NewConfirm(
		strings.ToUpper(verb[:1])+verb[1:]+" "+noun+"?",
		"Runs: "+opCommandline(a, verb, mod.Key, flags),
		"y confirm · n cancel",
	)
	m.pending = func(m2 *App) tea.Cmd { return m2.runCoreOp(verb, mod.Key, flags) }
}

func describeOp(mod catalog.Module, flags []string) string {
	if len(flags) == 0 {
		return "the whole '" + mod.Key + "' module"
	}
	return fmt.Sprintf("%d tool(s): %s", len(flags), strings.Join(flags, " "))
}

func opCommandline(a *App, verb, module string, flags []string) string {
	if !a.core.Installed() {
		return "(core not installed)"
	}
	args := append([]string{"core", verb, module}, flags...)
	return strings.Join(args, " ")
}

func (m *modulesScreen) runVerb(a *App, verb string, confirmed bool) tea.Cmd {
	if !a.core.Installed() {
		a.ToastError("core not installed — press i on the dashboard first")
		return nil
	}
	mod := m.currentModule()
	flags := m.selectedFlags(mod)
	return a.runCoreOp(verb, mod.Key, flags)
}

// runCoreOp streams a real core invocation and refreshes statuses after.
func (a *App) runCoreOp(verb, module string, flags []string) tea.Cmd {
	if !a.core.Installed() {
		a.ToastError("core not installed")
		return nil
	}
	cmd := a.core.Cmd(verb, module, flags)
	return a.RunChild(fmt.Sprintf("core %s %s", verb, module), cmd, func(m *App, err error) tea.Cmd {
		if err != nil {
			if tail, ok := core.LogTail(module, 30); ok {
				p := ui.NewPager("install_"+module+".log (last 30 lines)", tail)
				cur := m.current()
				if ms, ok := cur.(*modulesScreen); ok {
					ms.pager = p
				}
			}
			m.ToastError(fmt.Sprintf("core %s %s failed — see log", verb, module))
		} else if verb == core.VerbInstall && module == "shell" {
			m.state.MarkRestart("shell-stack")
			m.SaveState()
			m.ToastWarn("Installed. Restart Termux to apply the ZSH stack.")
		}
		return m.current().Update(m, refreshMsg{})
	})
}

func (m *modulesScreen) View() string {
	w := clampW(m.app.width, 40)
	h := clampI(m.app.height-10, 6, 40)
	m.modMenu.Height = h
	mod := m.currentModule()

	leftW := clampI(w/3, 20, 34)
	left := ui.TitleStyle.Render("Modules") + "\n" +
		ui.SubtleStyle.Render(strings.Repeat("─", leftW)) + "\n" +
		m.modMenu.View(leftW)

	right := ui.TitleStyle.Render(mod.Key+" — "+mod.Title) + "\n"
	if mod.ModuleLevel {
		right += ui.SubtleStyle.Render(strings.Repeat("─", w-leftW-4)) + "\n"
		right += ui.SubtleStyle.Render("This module installs as a whole (no per-tool selection).") + "\n\n"
		status := ui.StatusText(m.statusOfWhole(mod.Key))
		right += "Stack status: " + status + "\n"
	} else {
		right += ui.SubtleStyle.Render(strings.Repeat("─", w-leftW-4)) + "\n"
		rows, _ := m.rows(mod)
		start := clampI(m.cursor2-h+1, 0, maxI(0, len(rows)-h))
		for i, r := range rows[start:] {
			idx := start + i
			box := "[ ] "
			if r.checked {
				box = ui.GoodStyle.Render("[x] ")
			}
			name := r.tool.Name
			if idx == m.cursor2 && m.focusR {
				name = ui.SelStyle.Render(name)
			}
			line := box + name
			detail := r.tool.Flag
			if r.tool.Command != "" {
				detail += " (" + r.tool.Command + ")"
			}
			line += "  " + ui.SubtleStyle.Render(detail) + "  " + ui.StatusText(r.tool.Status.String())
			if m.isGLibc(mod, r.tool.Flag) {
				line += " " + ui.WarnStyle.Render("[glibc]")
			}
			right += line + "\n"
		}
		if len(rows) == 0 {
			right += ui.SubtleStyle.Render("(loading…)") + "\n"
		}
		if e := m.errs[mod.Key]; e != "" && len(rows) == 0 {
			right += ui.WarnStyle.Render("live status unavailable: "+e+" — using static catalog") + "\n"
		}
	}

	bar := "space toggle · a all · n none · i install · u update · x uninstall · r reinstall · d docs · o docs(web) · L log"
	if !m.app.core.Installed() {
		bar = ui.BadStyle.Render("core is not installed — press q then i on the dashboard to install it")
	}

	left = fixedWidth(left, leftW)
	body := lipJoin(left, right, w)
	out := body + "\n" + ui.SubtleStyle.Render(strings.Repeat("─", w)) + "\n" + bar + "\n"
	if m.confirm != nil {
		out += "\n" + m.confirm.View(w) + "\n"
	}
	if m.pager != nil {
		return overlay(body, m.pager.View(w), w)
	}
	return out
}

func (m *modulesScreen) statusOfWhole(module string) string {
	if live, ok := m.live[module]; ok && len(live) > 0 {
		inst := 0
		for _, t := range live {
			if t.Status == core.StatusInstalled {
				inst++
			}
		}
		if inst == 0 {
			return "not installed"
		}
		if inst == len(live) {
			return "installed"
		}
		return fmt.Sprintf("partial (%d/%d)", inst, len(live))
	}
	if e := m.errs[module]; e != "" {
		return "unknown"
	}
	return "…"
}

func (m *modulesScreen) isGLibc(mod catalog.Module, flag string) bool {
	for _, t := range mod.Tools {
		if t.Flag == flag {
			return t.GLibc
		}
	}
	return false
}
