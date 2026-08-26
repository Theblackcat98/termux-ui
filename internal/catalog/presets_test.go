package catalog

import (
	"strings"
	"testing"

	"github.com/DevCoreXOfficial/termux-ui/internal/extrakeys"
)

func TestPresetsSerializeAndRoundTrip(t *testing.T) {
	if len(Presets) != 4 {
		t.Fatalf("want 4 presets, got %d", len(Presets))
	}
	for _, p := range Presets {
		s := extrakeys.Serialize(p.Layout)
		back, err := extrakeys.Parse(s)
		if err != nil {
			t.Fatalf("%s: parse: %v (%s)", p.Name, err, s)
		}
		if again := extrakeys.Serialize(back); again != s {
			t.Fatalf("%s: not byte-identical:\n%s\n%s", p.Name, s, again)
		}
		if errs := extrakeys.Validate(back); len(errs) != 0 {
			t.Fatalf("%s: shipped preset fails validation: %v", p.Name, errs)
		}
	}
}

func TestDefaultPresetMatchesPlanSpec(t *testing.T) {
	got := extrakeys.Serialize(Presets[0].Layout)
	want := `[['ESC','/',{key:'-',popup:'|'},'HOME','UP','END','PGUP'],['TAB','CTRL','ALT','LEFT','DOWN','RIGHT','PGDN']]`
	if got != want {
		t.Fatalf("default preset drift:\n got %s\nwant %s", got, want)
	}
}

func TestTmuxPresetContainsRequiredMacros(t *testing.T) {
	s := extrakeys.Serialize(Presets[2].Layout)
	for _, frag := range []string{"'ALT j'", "'ALT g'", "KEYBOARD", "CTRL d"} {
		if !strings.Contains(s, frag) {
			t.Fatalf("tmux preset missing %q: %s", frag, s)
		}
	}
}

func TestVimPresetContainsQuickExitAndPopups(t *testing.T) {
	s := extrakeys.Serialize(Presets[3].Layout)
	for _, frag := range []string{":q", "QuickExit", "BACKSLASH", ":w"} {
		if !strings.Contains(s, frag) {
			t.Fatalf("vim preset missing %q: %s", frag, s)
		}
	}
}
