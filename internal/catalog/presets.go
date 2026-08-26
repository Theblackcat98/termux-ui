package catalog

import (
	"github.com/DevCoreXOfficial/termux-ui/internal/extrakeys"
)

// Preset is a named extra-keys layout shipped with the TUI.
type Preset struct {
	Name   string
	Layout *extrakeys.Layout
}

// Presets ship the four layouts required by the plan. Serialized forms are
// produced by extrakeys.Serialize so edits round-trip byte-identically.
var Presets = []Preset{
	{
		Name: "Default (2 rows)",
		Layout: &extrakeys.Layout{Rows: [][]extrakeys.Cell{
			{
				extrakeys.Named("ESC"), extrakeys.Lit("/"),
				extrakeys.WithPopup(extrakeys.Lit("-"), extrakeys.SimplePopup("|")),
				extrakeys.Named("HOME"), extrakeys.Named("UP"), extrakeys.Named("END"), extrakeys.Named("PGUP"),
			},
			{
				extrakeys.Named("TAB"), extrakeys.Named("CTRL"), extrakeys.Named("ALT"),
				extrakeys.Named("LEFT"), extrakeys.Named("DOWN"), extrakeys.Named("RIGHT"), extrakeys.Named("PGDN"),
			},
		}},
	},
	{
		Name: "Single row",
		Layout: &extrakeys.Layout{Rows: [][]extrakeys.Cell{
			{
				extrakeys.Named("ESC"), extrakeys.Named("TAB"), extrakeys.Named("CTRL"), extrakeys.Named("ALT"),
				extrakeys.WithPopup(extrakeys.Lit("-"), extrakeys.SimplePopup("|")),
				extrakeys.Named("DOWN"), extrakeys.Named("UP"),
			},
		}},
	},
	{
		Name: "tmux",
		Layout: &extrakeys.Layout{Rows: [][]extrakeys.Cell{
			{
				extrakeys.Named("ESC"), extrakeys.Named("CTRL"), extrakeys.Named("ALT"),
				extrakeys.Macro("CTRL f", "prefix"),
				extrakeys.Macro("CTRL f d", "f d"),
				extrakeys.Macro("ALT j", "A-j"),
				extrakeys.Macro("ALT g", "A-g"),
				extrakeys.Named("HOME"), extrakeys.Named("END"),
			},
			{
				extrakeys.Named("TAB"),
				extrakeys.WithPopup(extrakeys.Lit("-"), extrakeys.SimplePopup("|")),
				extrakeys.WithPopup(extrakeys.Named("UP"), extrakeys.SimplePopup("PGUP")),
				extrakeys.WithPopup(extrakeys.Named("DOWN"), extrakeys.SimplePopup("PGDN")),
				extrakeys.WithPopup(extrakeys.Named("LEFT"), extrakeys.SimplePopup("HOME")),
				extrakeys.WithPopup(extrakeys.Named("RIGHT"), extrakeys.SimplePopup("END")),
				extrakeys.WithPopup(extrakeys.Named("KEYBOARD"), extrakeys.MacroPopup("CTRL d", "exit")),
			},
		}},
	},
	{
		Name: "Vim",
		Layout: &extrakeys.Layout{Rows: [][]extrakeys.Cell{
			{
				extrakeys.WithPopup(extrakeys.Named("ESC"), extrakeys.MacroPopup(":q\n", "QuickExit")),
				extrakeys.WithPopup(extrakeys.Lit("/"), extrakeys.SimplePopup("BACKSLASH")),
				extrakeys.WithPopup(extrakeys.Named("TAB"), extrakeys.SimplePopup(":w")),
				extrakeys.WithPopup(extrakeys.Named("CTRL"), extrakeys.MacroPopup(":wq\n", "")),
				extrakeys.Named("HOME"), extrakeys.Named("UP"), extrakeys.Named("END"), extrakeys.Named("PGUP"),
			},
			{
				extrakeys.WithPopup(extrakeys.Lit("("), extrakeys.SimplePopup("{")),
				extrakeys.WithPopup(extrakeys.Lit("#"), extrakeys.SimplePopup("$")),
				extrakeys.WithPopup(extrakeys.Lit(")"), extrakeys.SimplePopup("}")),
				extrakeys.WithPopup(extrakeys.Named("LEFT"), extrakeys.MacroPopup("CTRL HOME", "")),
				extrakeys.WithPopup(extrakeys.Named("DOWN"), extrakeys.MacroPopup("CTRL PGDN", "")),
				extrakeys.WithPopup(extrakeys.Named("UP"), extrakeys.MacroPopup("CTRL PGUP", "")),
				extrakeys.WithPopup(extrakeys.Named("RIGHT"), extrakeys.MacroPopup("CTRL END", "")),
				extrakeys.Named("PGDN"),
			},
		}},
	},
}

// ExtraKeysStyleValues are the allowed values of extra-keys-style.
var ExtraKeysStyleValues = []string{"default", "arrows-only", "arrows-all", "all", "none"}
