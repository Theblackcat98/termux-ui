package extrakeys

import (
	"strings"
	"testing"
)

func TestSerializeDefaultPresetShape(t *testing.T) {
	l := &Layout{Rows: [][]Cell{
		{Named("ESC"), Lit("/"), WithPopup(Lit("-"), SimplePopup("|")), Named("HOME"), Named("UP"), Named("END"), Named("PGUP")},
		{Named("TAB"), Named("CTRL"), Named("ALT"), Named("LEFT"), Named("DOWN"), Named("RIGHT"), Named("PGDN")},
	}}
	want := `[['ESC','/',{key:'-',popup:'|'},'HOME','UP','END','PGUP'],['TAB','CTRL','ALT','LEFT','DOWN','RIGHT','PGDN']]`
	if got := Serialize(l); got != want {
		t.Fatalf("serialize:\n got %s\nwant %s", got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	l := &Layout{Rows: [][]Cell{
		{Macro("CTRL f d", "A-f"), WithPopup(Lit("-"), SimplePopup("|")), Named("KEYBOARD")},
		{WithPopup(Lit("x"), MacroPopup(":q\n", "exit")), Lit("#"), Named("F12")},
	}}
	got := Serialize(l)
	back, err := Parse(got)
	if err != nil {
		t.Fatalf("parse %q: %v", got, err)
	}
	if again := Serialize(back); again != got {
		t.Fatalf("round trip mismatch:\n got %s\nagain %s", again, got)
	}
}

func TestParseOfficialDefault(t *testing.T) {
	in := `[['ESC','/','-','HOME','UP','END','PGUP'],['TAB','CTRL','ALT','LEFT','DOWN','RIGHT','PGDN']]`
	l, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Rows) != 2 || len(l.Rows[0]) != 7 || l.Rows[1][2].Value != "ALT" {
		t.Fatalf("parsed wrong: %+v", l.Rows)
	}
	if Serialize(l) != in {
		t.Fatalf("re-serialize mismatch")
	}
}

func TestParseContinuationForm(t *testing.T) {
	// Termux canonical multiline property value (after escape decoding the
	// engine hands us newlines; here we also tolerate literal backslash-newline).
	in := "[ \\\n ['ESC'], \\\n ['CTRL'] \\\n]"
	l, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Rows) != 2 || l.Rows[0][0].Value != "ESC" {
		t.Fatalf("got %+v", l.Rows)
	}
}

func TestValidationModifierDuplicate(t *testing.T) {
	l := &Layout{Rows: [][]Cell{
		{Named("CTRL"), Named("ALT")},
		{Named("SHIFT"), Named("CTRL")},
	}}
	errs := Validate(l)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "twice") && strings.Contains(e.Error(), "CTRL") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate CTRL error, got %v", errs)
	}
}

func TestValidationBackslashAndLimits(t *testing.T) {
	l := &Layout{Rows: [][]Cell{
		{Lit(`\`)},
		{},
	}}
	errs := Validate(l)
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	if !strings.Contains(joined, "BACKSLASH") || !strings.Contains(joined, "row 1 needs at least 1 key") {
		t.Fatalf("missing expected errors: %v", errs)
	}

	big := &Layout{}
	for r := 0; r < MaxRows+1; r++ {
		row := make([]Cell, MaxCols)
		for c := range row {
			row[c] = Lit("a")
		}
		big.Rows = append(big.Rows, row)
	}
	if len(Validate(big)) == 0 {
		t.Fatal("row cap not enforced")
	}
	wide := &Layout{Rows: [][]Cell{{}}}
	for i := 0; i < MaxCols+1; i++ {
		wide.Rows[0] = append(wide.Rows[0], Lit("a"))
	}
	if len(Validate(wide)) == 0 {
		t.Fatal("column cap not enforced")
	}
}

func TestValidLayoutPasses(t *testing.T) {
	l := &Layout{Rows: [][]Cell{
		{Named("ESC"), Lit("/"), Macro("CTRL f d", "")},
	}}
	if errs := Validate(l); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}
