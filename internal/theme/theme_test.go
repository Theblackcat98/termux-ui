package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledThemesAllParse(t *testing.T) {
	all := Bundled()
	if len(all) != 10 {
		t.Fatalf("want 10 bundled themes, got %d", len(all))
	}
	want := []string{"CatppuccinMacchiato", "Dracula", "GruvboxDark", "Monokai", "Nord", "OneDark", "RosePine", "SolarizedDark", "SolarizedLight", "TokyoNight"}
	for i, name := range want {
		if all[i].Name != name {
			t.Fatalf("theme %d: got %s want %s", i, all[i].Name, name)
		}
	}
}

func TestAssetsDirInSyncWithEmbedded(t *testing.T) {
	root := "../../assets/themes"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("assets dir not present: %v", err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(e.Name(), ".properties")
		bundled, err := BundledByName(name)
		if err != nil {
			t.Fatalf("%s missing from embedded bundle: %v", name, err)
		}
		if bundled.Render() != string(data) {
			t.Fatalf("%s: assets copy out of sync with embedded theme", e.Name())
		}
	}
}

func TestHexValidation(t *testing.T) {
	for _, ok := range []string{"#aBc123", "#000000", "ffffff", "#FFFFFF"} {
		if _, err := NormalizeHex(ok); err != nil {
			t.Fatalf("%q should pass: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "#12345", "zzzzzz", "#12g45z"} {
		if _, err := NormalizeHex(bad); err == nil {
			t.Fatalf("%q should fail", bad)
		}
	}
}

func TestRenderRoundTrip(t *testing.T) {
	th, err := Parse("test", "color0=#111111\ncolor1=#222222\nforeground=#eeeeee\nbackground=#101010\ncursor=#dddddd\ncolor2=#333333\n")
	if err != nil {
		t.Fatal(err)
	}
	out := th.Render()
	th2, err := Parse("test", out)
	if err != nil {
		t.Fatal(err)
	}
	if th2.Render() != th.Render() || th2.Colors[0] != "#111111" {
		t.Fatalf("round trip mismatch: %+v", th2)
	}
}

func TestParseLenientDefaults(t *testing.T) {
	th, err := Parse("partial", "color0=#111111\n")
	if err != nil {
		t.Fatalf("partial theme should load: %v", err)
	}
	if th.Colors[0] != "#111111" || th.Foreground != "#FFFFFF" || th.Background != "#000000" {
		t.Fatalf("defaults wrong: %+v", th)
	}
	if _, err := Parse("bad", "color0=nothex\n"); err == nil {
		t.Fatal("invalid hex must fail")
	}
}
