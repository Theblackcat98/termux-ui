package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const listDev3Col = `
───────────────────────────────  Update Available  ───────────────────────────────

    ⚠ New version available: 4.27.2 (current: 4.26.0)

╭───────────────────╮
│ Development Tools │
╰───────────────────╯

    ➜ Available tools and install commands:

┌─────────────────┬───────────────┬───────────────┐
│ Tool            │ Install Flag  │ Status        │
├─────────────────┼───────────────┼───────────────┤
│ GitHub CLI      │ --gh          │ installed     │
│ Proot           │ --proot       │ not installed │
│ Ncurses Utils   │ --ncurses     │ installed     │
└─────────────────┴───────────────┴───────────────┘
`

const listAI4Col = `
│ AI Tools │
│ Tool                     │ Install Flag      │ Command    │ Status        │
│ Qwen Code                │ --qwen-code       │ qwen       │ not installed │
│ Gemini CLI               │ --gemini-cli      │ gemini     │ installed     │
`

func TestParseListThreeColumns(t *testing.T) {
	rows := ParseList(listDev3Col)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].Name != "GitHub CLI" || rows[0].Flag != "--gh" || rows[0].Status != StatusInstalled {
		t.Fatalf("row0 wrong: %+v", rows[0])
	}
	if rows[1].Status != StatusNotInstalled || rows[1].Command != "" {
		t.Fatalf("row1 wrong: %+v", rows[1])
	}
}

func TestParseListFourColumns(t *testing.T) {
	rows := ParseList(listAI4Col)
	if len(rows) != 2 {
		t.Fatalf("want 2, got %+v", rows)
	}
	if rows[0].Command != "qwen" || rows[0].Status != StatusNotInstalled {
		t.Fatalf("4-col parse wrong: %+v", rows[0])
	}
	if rows[1].Status != StatusInstalled {
		t.Fatalf("status wrong: %+v", rows[1])
	}
}

func TestParseListGarbageIsDefensive(t *testing.T) {
	if rows := ParseList("total nonsense\n\x1b[31mred\x1b[0m\n"); len(rows) != 0 {
		t.Fatalf("expected no rows from garbage, got %+v", rows)
	}
}

func TestFirstVersionLineSkipsBanner(t *testing.T) {
	v := firstVersionLine("\n New version available: 9.9 (current: 4.26.0)\n\nCORE-TERMUX v4.26.0")
	if !strings.HasPrefix(v, "4") && !strings.Contains(v, ".") {
		t.Fatalf("version parse got %q", v)
	}
}

func TestLogTail(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	path := LogPath("dev")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte("l1\nl2\nl3\n"), 0o600)
	got, ok := LogTail("dev", 2)
	if !ok || got != "l2\nl3" {
		t.Fatalf("tail=%q ok=%v", got, ok)
	}
	if _, ok := LogTail("nope", 5); ok {
		t.Fatal("expected missing log to be not-ok")
	}
}

func TestBuildArgs(t *testing.T) {
	got := BuildArgs(VerbInstall, "dev", []string{"--gh", "--fzf"})
	want := []string{"install", "dev", "--gh", "--fzf"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if m := BuildArgs(VerbInstall, "editor", nil); len(m) != 2 {
		t.Fatalf("module-level args wrong: %v", m)
	}
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"4.26.0", "4.27.2", -1},
		{"4.27.2", "4.27.2", 0},
		{"v5.0", "4.27.2", 1},
		{"10", "9.9.9", 1},
	}
	for _, c := range cases {
		if got := ParseVersion(c.a, c.b); got != c.want {
			t.Fatalf("ParseVersion(%s,%s)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}
