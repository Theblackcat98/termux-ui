package app

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/DevCoreXOfficial/termux-ui/internal/termuxconf"
)

// propKind is the control type of a property field.
type propKind int

const (
	kToggle propKind = iota
	kNumber
	kText
	kRadio
)

// propDef fully describes one termux.properties property in the form.
type propDef struct {
	Key      string
	Kind     propKind
	Def      string   // built-in default (effective when absent from file)
	Options  []string // radio choices
	Min, Max int      // number bounds (0 when unused)
	WarnOver int      // inline warning threshold
	Note     string   // fixed warning/help text
	Danger   bool     // requires typed confirmation to enable
	Restart  bool     // restart-required after change
	Validate func(string) error
}

var shortcutRe = regexp.MustCompile(`^ctrl \+ [A-Za-z0-9]+$`)

func validateShortcut(v string) error {
	if v == "" || shortcutRe.MatchString(v) {
		return nil
	}
	return errShortcutForm
}

type errString string

func (e errString) Error() string { return string(e) }

const errShortcutForm = errString("use the form: ctrl + <key>, e.g. ctrl + t")

// restartRequired lists properties that only apply after a full app restart.
var restartRequired = map[string]bool{
	"fullscreen":                 true,
	"use-fullscreen-workaround":  true,
	"use-black-ui":               true,
	"terminal-cursor-style":      true,
	"terminal-cursor-blink-rate": true,
}

// homeDirDefault resolves at call time ($HOME).
func homeDirDefault() string { return homeDir }

// propGroups is the §5.3 schema: groups and their fields.
var propGroups = []struct {
	Title string
	Props []propDef
	Info  []string // read-only info lines (G6)
}{
	{
		Title: "General",
		Props: []propDef{
			{Key: "default-working-directory", Kind: kText, Def: homeDir,
				Note: "Absolute path; relative paths are rejected", Validate: validateAbsDir},
			{Key: "terminal-transcript-rows", Kind: kNumber, Def: "2000", Min: 0, Max: 50000, WarnOver: 10000,
				Note: "large buffers slow scrolling"},
			{Key: "disable-terminal-session-change-toast", Kind: kToggle, Def: "false"},
			{Key: "hide-soft-keyboard-on-startup", Kind: kToggle, Def: "false"},
			{Key: "soft-keyboard-toggle-behaviour", Kind: kRadio, Def: "show/hide", Options: []string{"show/hide", "enable/disable"}},
			{Key: "volume-keys", Kind: kRadio, Def: "virtual", Options: []string{"virtual", "volume"},
				Note: "volume = keys adjust media volume; Ctrl/Alt emulation off"},
			{Key: "allow-external-apps", Kind: kToggle, Def: "false", Danger: true,
				Note: "Exposes Termux commands to other apps (security risk)"},
		},
	},
	{
		Title: "Terminal & Cursor",
		Props: []propDef{
			{Key: "terminal-cursor-style", Kind: kRadio, Def: "block", Options: []string{"block", "bar", "underline"}, Restart: true},
			{Key: "terminal-cursor-blink-rate", Kind: kNumber, Def: "0", Min: 0, Max: 2000, Restart: true,
				Note: "0 disables blinking; otherwise 100–2000 ms"},
			{Key: "use-black-ui", Kind: kToggle, Def: "false", Restart: true,
				Note: "Dark drawer/dialogs; auto on Android 9+ dark theme"},
			{Key: "bell-character", Kind: kRadio, Def: "vibrate", Options: []string{"vibrate", "beep", "ignore"}},
			{Key: "back-key", Kind: kRadio, Def: "back", Options: []string{"back", "escape"},
				Note: "escape sends ESC instead of leaving"},
		},
	},
	{
		Title: "Fullscreen & Layout",
		Props: []propDef{
			{Key: "fullscreen", Kind: kToggle, Def: "false", Restart: true},
			{Key: "use-fullscreen-workaround", Kind: kToggle, Def: "false",
				Note: "unstable on some devices; fixes extra-keys visibility in fullscreen"},
			{Key: "terminal-margin-horizontal", Kind: kNumber, Def: "3", Min: 0, Max: 100,
				Note: "dp; for curved screens / screen protectors"},
			{Key: "terminal-margin-vertical", Kind: kNumber, Def: "0", Min: 0, Max: 100},
		},
	},
	{
		Title: "Keyboard Workarounds",
		Props: []propDef{
			{Key: "enforce-char-based-input", Kind: kToggle, Def: "false",
				Note: "fixes keyboards (e.g. Samsung) that only commit text on Enter"},
			{Key: "ctrl-space-workaround", Kind: kToggle, Def: "false",
				Note: "breaks Ctrl+Space on devices where it already works"},
		},
	},
	{
		Title: "Hardware Session Shortcuts",
		Props: []propDef{
			{Key: "shortcut.create-session", Kind: kText, Def: "", Validate: validateShortcut},
			{Key: "shortcut.next-session", Kind: kText, Def: "", Validate: validateShortcut},
			{Key: "shortcut.previous-session", Kind: kText, Def: "", Validate: validateShortcut},
			{Key: "shortcut.rename-session", Kind: kText, Def: "", Validate: validateShortcut},
			{Key: "disable-hardware-keyboard-shortcuts", Kind: kToggle, Def: "false"},
		},
	},
	{
		Title: "Info",
		Info: []string{
			"Volume-key shortcuts while typing:",
			"  Vol-down = Ctrl",
			"  Vol-up + key:",
			"    E=Esc  T=Tab  1..0=F1..F10  W/A/S/D=arrows",
			"    B/F/X=Alt combos  L='|'  H='~'  U='_'",
			"    P/N=PGUP/PGDN  Q=show extra keys  V=volume control",
			"",
			"Open the raw file with E (uses $EDITOR).",
		},
	},
}

var homeDir = "/data/data/com.termux/files/home"

func validateAbsDir(v string) error {
	if v == "" {
		return nil
	}
	if !strings.HasPrefix(v, "/") {
		return errString("path must be absolute")
	}
	return nil
}

// parseBoolValue maps property truthy spellings.
func parseBoolValue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	}
	return false
}

func boolToProp(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// numberOK validates numeric fields against their bounds; blink rate has a
// special hole (only 0 or 100–2000 allowed).
func numberOK(d propDef, v string) error {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return errString("not a number")
	}
	if d.Min == 0 && d.Max == 0 {
		return nil
	}
	if n < d.Min || n > d.Max {
		return errString("out of range " + strconv.Itoa(d.Min) + "–" + strconv.Itoa(d.Max))
	}
	if d.Key == "terminal-cursor-blink-rate" && n != 0 && n < 100 {
		return errString("use 0 or 100–2000")
	}
	return nil
}

// effectiveValue implements §5.3 engine rules: file value if present else default.
func effectiveValue(f *termuxconf.File, d propDef) string {
	if v := f.Get(d.Key, ""); f.Has(d.Key) {
		return v
	}
	return d.Def
}

// applyFieldToDoc mutates the doc for one field following §5.3 write rules:
// absent keys are written only when the value differs from the default;
// resetting to the default removes the key; keys already at their effective
// value stay untouched.
func applyFieldToDoc(doc *termuxconf.File, d propDef, current string) (changed bool, restartTouched bool) {
	inFile := doc.Has(d.Key)
	effective := d.Def
	if inFile {
		effective = doc.Get(d.Key, "")
	}
	if current == effective {
		return false, false
	}
	if current == d.Def {
		if inFile {
			doc.Unset(d.Key)
			return true, restartRequired[d.Key]
		}
		return false, false
	}
	doc.Set(d.Key, current)
	return true, restartRequired[d.Key]
}
