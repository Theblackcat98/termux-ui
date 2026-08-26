// Command termux-ui is a single-binary TUI that customizes everything
// configurable about the Termux app and acts as a menu front-end to the
// core-termux CLI.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/DevCoreXOfficial/termux-ui/internal/app"
)

var version = "dev"

func main() {
	noColor := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version":
			fmt.Println("termux-ui", version)
			return
		case "--no-color":
			noColor = true
		case "--help", "-h":
			fmt.Println(`termux-ui — Termux customization TUI (core-termux front-end)

Usage:
  termux-ui [--no-color]

Options:
  --version    print version and exit
  --no-color   force monochrome rendering
  --help       this text

Screens: 1 dashboard · 2 modules & packages · 3 termux settings ·
4 extra keys · 5 themes & fonts · 6 shell setup · 7 apps & system`)
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s (try --help)\n", arg)
			os.Exit(2)
		}
	}

	if os.Getenv("TERMUX_VERSION") == "" {
		prefix := os.Getenv("PREFIX")
		if prefix == "/data/data/com.termux/files/usr" {
			fmt.Fprintln(os.Stderr, "warning: TERMUX_VERSION is not set — this tool only runs inside Termux; continuing anyway because $PREFIX looks right")
		} else {
			if prefix == "" {
				fmt.Fprintln(os.Stderr, "error: not running under Termux ($PREFIX missing). Exiting.")
			} else {
				fmt.Fprintln(os.Stderr, "error: not running under Termux ($PREFIX incorrect). Exiting.")
			}
			os.Exit(1)
		}
	}

	if noColor {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	if _, err := os.Stat("/data/data/com.termux/files/home/.termux"); err != nil {
		if home := os.Getenv("HOME"); home != "" {
			_ = os.MkdirAll(home+"/.termux", 0o700)
		}
	}

	p := tea.NewProgram(app.New(), tea.WithAltScreen())
	final, err := p.Run()
	_ = final
	if err != nil {
		fmt.Fprintln(os.Stderr, "termux-ui:", err)
		os.Exit(1)
	}
}
