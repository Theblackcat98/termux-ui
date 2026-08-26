package app

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DevCoreXOfficial/termux-ui/internal/core"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

// screen is one full-page view managed by the root App.
type screen interface {
	Title() string
	Update(m *App, msg tea.Msg) tea.Cmd
	View() string
	// WantsGlobalKeys reports whether number-jump/help/quit keys should be
	// handled globally (false while a dialog or text input has focus).
	WantsGlobalKeys() bool
	// FooterHints returns the screen-specific keys shown in the status bar.
	FooterHints() string
}

// Version is injected at build time (-ldflags "-X main.Version=...").
var Version = "dev"

// App is the root bubbletea model: it owns the navigation stack, shared
// services (core bridge, state, toasts) and global keybindings.
type App struct {
	width, height int

	core   *core.CLI
	state  *State
	stack  []screen
	toasts ui.Toasts

	showHelp bool
	quitting bool
}

// New builds the root model with the dashboard on the stack.
func New() *App {
	a := &App{
		core:  core.Detect(),
		state: LoadState(),
	}
	a.Push(newDashboard(a))
	return a
}

func (a *App) Push(s screen) { a.stack = append(a.stack, s) }

func (a *App) Pop() {
	if len(a.stack) > 1 {
		a.stack = a.stack[:len(a.stack)-1]
	}
}

func (a *App) current() screen { return a.stack[len(a.stack)-1] }
func (a *App) atTop() bool     { return len(a.stack) == 1 }

// Toast proxies for screens.
func (a *App) ToastInfo(s string)  { a.toasts.Info(s) }
func (a *App) ToastGood(s string)  { a.toasts.Good(s) }
func (a *App) ToastWarn(s string)  { a.toasts.Warn(s) }
func (a *App) ToastError(s string) { a.toasts.Error(s) }

func (a *App) SaveState() {
	if err := a.state.Save(); err != nil {
		a.ToastError("could not save state: " + err.Error())
	}
}

// tickMsg drives toast expiry.
type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(toastTTL, func(time.Time) tea.Msg { return tickMsg{} })
}

const toastTTL = 3 * time.Second

// childDone reports completion of a streamed child process.
type childDone struct {
	title string
	err   error
	after func(m *App, err error) tea.Cmd
}

// RunChild streams a long-running command's output live via tea.ExecProcess,
// then resumes the TUI and invokes after(err).
func (a *App) RunChild(title string, cmd *exec.Cmd, after func(m *App, err error) tea.Cmd) tea.Cmd {
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return childDone{title: title, err: err, after: after}
	})
}

func (m childDone) run(a *App) tea.Cmd {
	if m.err != nil {
		a.ToastError(fmt.Sprintf("%s failed", m.title))
	} else {
		a.ToastGood(fmt.Sprintf("%s done", m.title))
	}
	if m.after != nil {
		return m.after(a, m.err)
	}
	return nil
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return a, a.HandleMsg(msg)
}

// HandleMsg routes a message through global handling and the active screen.
func (a *App) HandleMsg(msg tea.Msg) tea.Cmd {
	switch t := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = t.Width, t.Height
		return nil

	case tickMsg:
		if !a.toasts.ExpireOldest() {
			return nil
		}
		return tickCmd()

	case childDone:
		return t.run(a)

	case tea.KeyMsg:
		key := t.String()
		if key == "ctrl+c" {
			a.quitting = true
			return tea.Quit
		}
		if a.showHelp {
			if key == "?" || key == "esc" || key == "q" {
				a.showHelp = false
			}
			return nil
		}
		cur := a.current()
		if cur.WantsGlobalKeys() {
			switch key {
			case "?":
				a.showHelp = true
				return nil
			case "q":
				if a.atTop() {
					a.quitting = true
					return tea.Quit
				}
			case "esc":
				if !a.atTop() {
					a.Pop()
					return nil
				}
			case "1", "2", "3", "4", "5", "6", "7":
				return a.jumpTo(key)
			}
		}
		return cur.Update(a, msg)
	}
	return a.current().Update(a, msg)
}

func (a *App) jumpTo(key string) tea.Cmd {
	idx := int(key[0] - '1')
	factories := []func(*App) screen{
		func(m *App) screen { return newDashboard(m) },
		func(m *App) screen { return newModulesScreen(m) },
		func(m *App) screen { return newSettingsScreen(m) },
		func(m *App) screen { return newExtraKeysScreen(m) },
		func(m *App) screen { return newThemeFontScreen(m) },
		func(m *App) screen { return newShellScreen(m) },
		func(m *App) screen { return newAppsSystemScreen(m) },
	}
	target := factories[idx](a)
	a.stack = append(a.stack[:1], target)
	return target.Update(a, refreshMsg{})
}

// refreshMsg asks a freshly pushed screen to load async data.
type refreshMsg struct{}

func (a *App) footer(hintKeys string) string {
	left := ""
	if hintKeys != "" {
		left = fmt.Sprintf(" %s ", ui.KeyStyle.Render(hintKeys))
	}
	right := "? help · esc back · q quit "
	gap := a.width - len(ui.Strip(left)) - len(ui.Strip(right))
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + right
	return ui.SubtleStyle.Render(strings.Repeat("─", a.width)) + "\n" + bar
}

// View implements tea.Model.
func (a *App) View() string {
	if a.quitting {
		return "Bye.\n"
	}
	var b strings.Builder
	breadcrumb := ""
	for i, s := range a.stack {
		if i > 0 {
			breadcrumb += " › "
		}
		breadcrumb += s.Title()
	}
	header := ui.TitleStyle.Render("termux-ui") + "  " + ui.SubtleStyle.Render(Version+"  "+breadcrumb)
	b.WriteString(header + "\n")
	b.WriteString(strings.Repeat("─", clampW(a.width, 20)) + "\n")

	view := a.current().View()
	b.WriteString(view)
	if !strings.HasSuffix(view, "\n") {
		b.WriteString("\n")
	}

	if toastView := a.toasts.View(); toastView != "" {
		b.WriteString("\n" + toastView + "\n")
	}
	b.WriteString("\n")
	b.WriteString(a.footer(a.current().FooterHints()))
	if a.showHelp {
		b.WriteString("\n" + helpOverlay())
	}
	return b.String()
}

// FooterHints implementations: screen-specific keys shown in the status bar.
func (s *dashboard) FooterHints() string        { return "i core · s storage · u update" }
func (s *modulesScreen) FooterHints() string    { return "space select · i/u/x/r act · d docs · L log" }
func (s *settingsScreen) FooterHints() string   { return "S save · E editor · z reset field" }
func (s *ekScreen) FooterHints() string         { return "enter edit · p presets · S save · E editor" }
func (s *tfScreen) FooterHints() string         { return "tab tabs · enter apply/install" }
func (s *shellScreen) FooterHints() string      { return "enter run · ←/→ pick shell" }
func (s *appsSystemScreen) FooterHints() string { return "enter run step" }

func helpOverlay() string {
	return ui.BoxStyle.Width(64).Render(
		ui.TitleStyle.Render("termux-ui — global keys") + "\n" +
			"1–7  jump to screens\n" +
			"↑/↓ j/k  move · enter select · esc back\n" +
			"q quit from top level · ctrl+c quit anywhere\n" +
			"? toggle this help\n\n" +
			"All writes to ~/.termux/* go through: diff → confirm → backup → write → termux-reload-settings.")
}

func clampW(w, min int) int {
	if w < min {
		return min
	}
	return w
}

// Init kicks off the toast ticker and the first update check.
func (a *App) Init() tea.Cmd {
	return tea.Batch(tickCmd(), a.current().Update(a, refreshMsg{}))
}
