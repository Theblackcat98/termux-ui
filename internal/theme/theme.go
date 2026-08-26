// Package theme manages ~/.termux/colors.properties: parsing, rendering,
// hex validation and the registry of bundled color schemes.
package theme

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Theme is a 16-color ANSI scheme plus foreground/background/cursor.
type Theme struct {
	Name       string
	Colors     [16]string // #RRGGBB
	Foreground string
	Background string
	Cursor     string
}

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ValidHex reports whether s is a well-formed #RRGGBB color.
func ValidHex(s string) bool { return hexRe.MatchString(s) }

// NormalizeHex validates and ensures the leading '#'; case is preserved so
// rendered files stay byte-stable.
func NormalizeHex(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty color")
	}
	if s[0] != '#' {
		s = "#" + s
	}
	if !ValidHex(s) {
		return "", fmt.Errorf("invalid color %q (want #RRGGBB)", s)
	}
	return s, nil
}

// defaults used when a key is absent (partial files still load).
var defaults = map[string]string{
	"color":      "#000000",
	"foreground": "#FFFFFF",
	"background": "#000000",
	"cursor":     "#FFFFFF",
}

// Parse reads a colors.properties text into a Theme. Missing entries fall
// back to safe defaults, so partially populated files load cleanly; values
// that are present must be valid #RRGGBB.
func Parse(name, text string) (*Theme, error) {
	t := &Theme{
		Name:       name,
		Foreground: defaults["foreground"],
		Background: defaults["background"],
		Cursor:     defaults["cursor"],
	}
	get := func(key string) (string, bool) {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if ok && strings.TrimSpace(k) == key {
				return strings.TrimSpace(v), true
			}
		}
		return "", false
	}
	for i := 0; i < 16; i++ {
		key := fmt.Sprintf("color%d", i)
		t.Colors[i] = defaults["color"]
		if v, ok := get(key); ok {
			h, err := NormalizeHex(v)
			if err != nil {
				return nil, fmt.Errorf("theme %q %s: %v", name, key, err)
			}
			t.Colors[i] = h
		}
	}
	for _, kv := range []struct {
		key string
		dst *string
	}{
		{"foreground", &t.Foreground},
		{"background", &t.Background},
		{"cursor", &t.Cursor},
	} {
		if v, ok := get(kv.key); ok {
			h, err := NormalizeHex(v)
			if err != nil {
				return nil, fmt.Errorf("theme %q %s: %v", name, kv.key, err)
			}
			*kv.dst = h
		}
	}
	return t, nil
}

// Render serializes the theme to colors.properties content.
func (t *Theme) Render() string {
	var b strings.Builder
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&b, "color%d=%s\n", i, t.Colors[i])
	}
	fmt.Fprintf(&b, "foreground=%s\nbackground=%s\ncursor=%s\n", t.Foreground, t.Background, t.Cursor)
	return b.String()
}

// ColorsPath is the user's colors.properties location.
func ColorsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".termux", "colors.properties")
}

// LoadColorsFile parses the live colors.properties if present.
func LoadColorsFile() (*Theme, error) {
	data, err := os.ReadFile(ColorsPath())
	if err != nil {
		return nil, err
	}
	return Parse("current", string(data))
}

//go:embed themes/*.properties
var themes embed.FS

// Bundled lists all shipped themes, sorted by name.
func Bundled() []*Theme {
	entries, err := themes.ReadDir("themes")
	if err != nil {
		return nil
	}
	var out []*Theme
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".properties") {
			continue
		}
		data, err := themes.ReadFile("themes/" + e.Name())
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".properties")
		t, err := Parse(name, string(data))
		if err != nil {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// BundledByName returns one shipped theme.
func BundledByName(name string) (*Theme, error) {
	for _, t := range Bundled() {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, fmt.Errorf("no bundled theme named %q", name)
}
