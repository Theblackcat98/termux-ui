package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/DevCoreXOfficial/termux-ui/internal/termuxconf"
	"github.com/DevCoreXOfficial/termux-ui/internal/ui"
)

// timeNow is indirected for tests.
var timeNow = time.Now

func clampI(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// lipJoin places two panes side by side within total width.
func lipJoin(left, right string, width int) string {
	leftW := lipgloss.Width(left)
	rightW := clampI(width-leftW-1, 1, 200)
	rightLines := strings.Split(right, "\n")
	var b strings.Builder
	leftLines := strings.Split(left, "\n")
	n := len(leftLines)
	if len(rightLines) > n {
		n = len(rightLines)
	}
	for i := 0; i < n; i++ {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		pad := leftW - lipgloss.Width(l)
		if pad < 0 {
			pad = 0
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		r = uiTrunc(r, rightW)
		b.WriteString(l + strings.Repeat(" ", pad+1) + r + "\n")
	}
	return b.String()
}

// overlay centers a modal over the base view.
func overlay(base, modal string, width int) string {
	baseLines := strings.Split(base, "\n")
	modalLines := strings.Split(modal, "\n")
	vOff := (len(baseLines) - len(modalLines)) / 2
	if vOff < 0 {
		vOff = 0
	}
	hOff := (width - lipgloss.Width(modal)) / 2
	if hOff < 0 {
		hOff = 0
	}
	out := make([]string, len(baseLines))
	copy(out, baseLines)
	for i, ml := range modalLines {
		y := vOff + i
		if y >= len(out) {
			break
		}
		line := out[y]
		w := lipgloss.Width(line)
		padRight := width - w - hOff - lipgloss.Width(ml)
		if padRight < 0 {
			padRight = 0
		}
		if hOff > w {
			out[y] = line + strings.Repeat(" ", hOff-w) + ml + strings.Repeat(" ", padRight)
		} else {
			cut := uiTrunc(line, hOff)
			out[y] = cut + ml
		}
	}
	return strings.Join(out, "\n")
}

func uiTrunc(s string, max int) string {
	if max <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= 3 {
		return string(runes)
	}
	return string(runes[:maxI(1, max-1)]) + "…"
}

// typeInto feeds printable keys and editing keys into a text input.
func typeInto(t *ui.TextInput, key string) {
	switch key {
	case "backspace":
		t.Backspace()
	case "delete":
		t.Delete()
	case "left":
		t.Left()
	case "right":
		t.Right()
	default:
		if len(key) == 1 {
			t.Insert([]rune(key)[0])
		}
	}
}

// downloadURL fetches raw bytes over HTTP with a generous timeout.
func downloadURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// saveWithBackup backs up the original and atomically replaces it.
func saveWithBackup(path, content string) (string, error) {
	if _, err := backupFile(path); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

// backupFile copies an existing file to its timestamped sibling.
func backupFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	dst := path + ".bak." + timeNow().Format("20060102-150405")
	err = os.WriteFile(dst, data, 0o600)
	return dst, err
}

// reloadSettingsSafe runs termux-reload-settings when available.
func reloadSettingsSafe() error { return termuxconf.ReloadSettings() }

func termuxconfReload() error { return termuxconf.ReloadSettings() }

func termuxDiff(a, b string) string {
	return termuxconf.UnifiedDiff("current", "new", a, b)
}

// OpenInEditor streams $EDITOR over a path via tea.ExecProcess.
func (a *App) OpenInEditor(path string) tea.Cmd {
	if os.Getenv("EDITOR") == "" {
		os.Setenv("EDITOR", "vi")
	}
	cmd := exec.Command("sh", "-c", `exec "$EDITOR" "$1"`, "sh", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return a.RunChild("edit "+filepath.Base(path), cmd, nil)
}

// fixedWidth normalizes a block to exactly w columns per line so it can be
// placed reliably by lipJoin.
func fixedWidth(block string, w int) string {
	lines := strings.Split(block, "\n")
	for i, l := range lines {
		t := uiTrunc(l, w)
		pad := w - lipgloss.Width(t)
		if pad > 0 {
			t += strings.Repeat(" ", pad)
		}
		lines[i] = t
	}
	return strings.Join(lines, "\n")
}
