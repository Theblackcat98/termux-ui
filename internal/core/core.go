// Package core bridges the TUI to the real core-termux CLI.
//
// The TUI never installs anything itself: every action becomes a
// `core install|update|uninstall|reinstall <module> [--tool ...]` invocation,
// executed through tea.ExecProcess so output streams live to the user.
package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Status is per-tool install state as reported by `core list <module>`.
type Status int

const (
	StatusUnknown Status = iota
	StatusInstalled
	StatusNotInstalled
)

func (s Status) String() string {
	switch s {
	case StatusInstalled:
		return "installed"
	case StatusNotInstalled:
		return "not installed"
	default:
		return "unknown"
	}
}

// Tool is one row of `core list <module>` merged onto the static catalog.
type Tool struct {
	Name    string
	Flag    string // "--python"; empty for module-level entries
	Command string // provided binary (4-column tables); may be empty
	Status  Status
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// CLI describes a detected core installation.
type CLI struct {
	Path    string // absolute path; "" when absent
	Version string // e.g. "4.26.0"
}

// Detect looks up core on $PATH and reads its version.
func Detect() *CLI {
	p, err := exec.LookPath("core")
	if err != nil {
		return &CLI{}
	}
	c := &CLI{Path: p}
	out, err := exec.Command(p, "--version").CombinedOutput()
	if err == nil {
		c.Version = firstVersionLine(string(out))
	}
	return c
}

func firstVersionLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(ansiRe.ReplaceAllString(line, ""))
		if line != "" && !strings.Contains(line, "update") && !strings.Contains(line, "Update") {
			return strings.TrimPrefix(line, "v")
		}
	}
	return ""
}

// Installed reports whether core was found.
func (c *CLI) Installed() bool { return c.Path != "" }

// List parses `core list <module>` into tool rows. The parser is defensive:
// any unrecognizable line is skipped; when nothing parses it returns an error
// and callers fall back to the static catalog with Status unknown.
func (c *CLI) List(module string) ([]Tool, error) {
	if !c.Installed() {
		return nil, fmt.Errorf("core not installed")
	}
	out, err := exec.Command(c.Path, "list", module).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("core list %s: %v", module, err)
	}
	rows := ParseList(string(out))
	if len(rows) == 0 {
		return nil, fmt.Errorf("could not parse `core list %s` output", module)
	}
	return rows, nil
}

// ParseList extracts tool rows from raw (possibly ANSI-colored) list output.
// It accepts both table shapes seen in core 4.x:
//
//	│ Tool │ Install Flag │ Status │
//	│ Tool │ Install Flag │ Command │ Status │
func ParseList(raw string) []Tool {
	var tools []Tool
	for _, line := range strings.Split(raw, "\n") {
		line = ansiRe.ReplaceAllString(line, "")
		if !strings.Contains(line, "│") {
			continue
		}
		cells := splitCells(line)
		// Drop single-cell title boxes ("│ AI Tools │").
		if len(cells) < 3 {
			continue
		}
		name := strings.TrimSpace(cells[0])
		flag := strings.TrimSpace(cells[1])
		statusIdx := 2
		command := ""
		if len(cells) >= 4 {
			command = strings.TrimSpace(cells[2])
			statusIdx = 3
		}
		statusTxt := strings.ToLower(strings.TrimSpace(cells[statusIdx]))
		// Skip header and separator rows.
		if name == "" || name == "Tool" || name == "Plugin" || name == "Module" ||
			strings.HasPrefix(name, "─") || strings.HasPrefix(name, "==") ||
			strings.Contains(statusTxt, "flag") || statusTxt == "" {
			continue
		}
		t := Tool{Name: name, Flag: flag, Command: command}
		switch {
		case strings.HasPrefix(statusTxt, "not"):
			t.Status = StatusNotInstalled
		case statusTxt == "installed":
			t.Status = StatusInstalled
		default:
			t.Status = StatusUnknown
		}
		tools = append(tools, t)
	}
	return tools
}

// splitCells splits a box-drawing table row into its cells.
func splitCells(line string) []string {
	line = strings.Trim(strings.TrimSpace(line), "│")
	parts := strings.Split(line, "│")
	for i := range parts {
		parts[i] = strings.TrimRight(parts[i], " ")
	}
	return parts
}

// Action verbs accepted by BuildArgs.
const (
	VerbInstall   = "install"
	VerbUpdate    = "update"
	VerbUninstall = "uninstall"
	VerbReinstall = "reinstall"
)

// BuildArgs assembles `core <verb> <module> [flags...]`. When flags is empty
// the whole module is operated on (correct for both tool and module-level
// modules).
func BuildArgs(verb, module string, flags []string) []string {
	args := []string{verb, module}
	return append(args, flags...)
}

// Cmd returns the exec.Cmd for a core action; the app runs it through
// tea.ExecProcess with this terminal's stdio.
func (c *CLI) Cmd(verb, module string, flags []string) *exec.Cmd {
	return exec.Command(c.Path, BuildArgs(verb, module, flags)...)
}

// CacheDir is where core writes install logs (~/.cache/core-termux).
func CacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "core-termux")
}

// LogPath is the log file core writes for a module operation.
func LogPath(module string) string {
	return filepath.Join(CacheDir(), "install_"+module+".log")
}

// LogTail returns the last n lines of the module's install log.
func LogTail(module string, n int) (string, bool) {
	data, err := os.ReadFile(LogPath(module))
	if err != nil {
		return "", false
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n"), true
}

// ParseVersion compares dotted versions; returns -1, 0 or 1.
func ParseVersion(a, b string) int {
	as, bs := strings.Split(strings.TrimPrefix(a, "v"), "."), strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv int
		if i < len(as) {
			av, _ = strconv.Atoi(strings.TrimSpace(as[i]))
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(strings.TrimSpace(bs[i]))
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}
