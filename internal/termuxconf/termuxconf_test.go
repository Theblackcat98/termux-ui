package termuxconf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sample = `# Termux properties
# written by hand

extra-keys = [ \
 ['ESC','/'], \
 ['TAB','CTRL'] \
]

fullscreen=true   # trailing comments are not java-standard but must survive? no:
use-black-ui = true
volume-keys=virtual
`

func TestParsePreservesCommentsAndUnknowns(t *testing.T) {
	f := Parse(sample)
	got := f.Render()
	if got != sample {
		t.Fatalf("round-trip mismatch:\n--- want ---\n%s\n--- got ---\n%s", sample, got)
	}
}

func TestSetExistingKeyKeepsPosition(t *testing.T) {
	f := Parse(sample)
	f.Set("volume-keys", "volume")
	out := f.Render()
	if !strings.Contains(out, "volume-keys=volume") {
		t.Fatalf("value not updated:\n%s", out)
	}
	idxVol := strings.Index(out, "volume-keys")
	idxBlack := strings.Index(out, "use-black-ui")
	if idxVol < idxBlack {
		t.Fatalf("position not preserved (moved after later keys)")
	}
	if strings.Contains(out, "virtual") {
		t.Fatalf("old value still present")
	}
}

func TestMultilineReplace(t *testing.T) {
	f := Parse(sample)
	newLayout := "[['ESC','|'],['CTRL','ALT'],['1','2','3']]"
	f.SetWrapped("extra-keys", newLayout)
	out := f.Render()
	if strings.Contains(out, "'TAB'") {
		t.Fatalf("stale continuation lines left behind:\n%s", out)
	}
	if !strings.Contains(out, `'ESC','|'`) || !strings.Contains(out, `'1','2','3'`) {
		t.Fatalf("new layout incomplete:\n%s", out)
	}
	if !strings.Contains(out, "\\\n") {
		t.Fatalf("long value not wrapped with continuations:\n%s", out)
	}
	f2 := Parse(out)
	if v := f2.Get("extra-keys", ""); v != newLayout {
		t.Fatalf("decoded value %q, want %q (continuations must join losslessly)", v, newLayout)
	}
}

func TestUnset(t *testing.T) {
	f := Parse(sample)
	if !f.Unset("use-black-ui") {
		t.Fatal("Unset returned false for existing key")
	}
	if f.Has("use-black-ui") {
		t.Fatal("key still present")
	}
	if f.Unset("no-such-key") {
		t.Fatal("Unset returned true for missing key")
	}
}

func TestEscapesDecode(t *testing.T) {
	f := Parse("a=b\\nc\\nd\nb=x\\ty\nc=p\\:q\n")
	if v := f.Get("a", ""); v != "b\nc\nd" {
		t.Fatalf("a=%q", v)
	}
	if v := f.Get("b", ""); v != "x\ty" {
		t.Fatalf("b=%q", v)
	}
	if v := f.Get("c", ""); v != "p:q" {
		t.Fatalf("c=%q", v)
	}
}

func TestColonSeparatorAndContinuation(t *testing.T) {
	f := Parse("# c\nlongkey : value \\\n continued\n")
	if v := f.Get("longkey", ""); v != "value continued" {
		t.Fatalf("got %q", v)
	}
}

func TestUnifiedDiffShowsOnlyChangedLines(t *testing.T) {
	oldText := "a=1\nb=2\nc=3\n"
	newText := "a=1\nb=CHANGED\nc=3\nd=4\n"
	d := UnifiedDiff("old", "new", oldText, newText)
	for _, want := range []string{"--- old", "+++ new", "- b=2", "+ b=CHANGED", "+ d=4"} {
		if !strings.Contains(d, want) {
			t.Fatalf("diff missing %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, "a=1") && strings.Contains(d, "- a=1") {
		t.Fatal("unchanged line reported as removed")
	}
	if UnifiedDiff("o", "n", oldText, oldText) != "" {
		t.Fatal("identical content should produce empty diff")
	}
}

func TestSaveBacksUpOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "termux.properties")
	os.WriteFile(path, []byte("a=1\n"), 0o600)

	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	bak, err := Save(path, "a=2\n", now)
	if err != nil {
		t.Fatal(err)
	}
	wantBak := BackupPath(path, now)
	if bak != wantBak {
		t.Fatalf("backup path %q, want %q", bak, wantBak)
	}
	data, err := os.ReadFile(bak)
	if err != nil || string(data) != "a=1\n" {
		t.Fatalf("backup content wrong: %q err=%v", data, err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "a=2\n" {
		t.Fatalf("new content wrong: %q", data)
	}
}

func TestBackupMissingFileIsNoop(t *testing.T) {
	bak, err := Backup(filepath.Join(t.TempDir(), "missing.properties"), time.Now())
	if err != nil || bak != "" {
		t.Fatalf("expected no-op backup, got %q, %v", bak, err)
	}
}

func TestDuplicateHandling(t *testing.T) {
	text := "k=old\nk=new\n"
	f := Parse(text)

	// Get should return the last occurrence
	if got := f.Get("k", ""); got != "new" {
		t.Fatalf("Get: got %q, want %q", got, "new")
	}

	// Set should update the last occurrence and remove earlier ones
	f.Set("k", "updated")
	out := f.Render()
	if strings.Count(out, "k=") != 1 {
		t.Fatalf("Set: expected exactly 1 occurrence of k= in output, got: %q", out)
	}
	if !strings.Contains(out, "k=updated") {
		t.Fatalf("Set: expected output to contain k=updated, got: %q", out)
	}

	// Has should be true
	if !f.Has("k") {
		t.Fatalf("Has: expected true")
	}

	// Keys should only list "k" once
	f = Parse(text)
	keys := f.Keys()
	if len(keys) != 1 || keys[0] != "k" {
		t.Fatalf("Keys: expected [\"k\"], got %v", keys)
	}

	// Unset should remove all occurrences
	f = Parse(text)
	if !f.Unset("k") {
		t.Fatalf("Unset: expected true")
	}
	out = f.Render()
	if strings.Contains(out, "k=") {
		t.Fatalf("Unset: expected all occurrences to be removed, got: %q", out)
	}
}
