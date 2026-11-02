package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Browser is a directory and file picker widget.
type Browser struct {
	Path       string
	Title      string
	Filter     func(os.DirEntry) bool // if nil, shows all
	OnSelect   func(string)
	OnCancel   func()
	Menu       *Menu
	dirsOnly   bool
	cachedList []os.DirEntry
}

// NewBrowser creates a new file/directory browser.
func NewBrowser(startPath, title string, dirsOnly bool) *Browser {
	if startPath == "" {
		startPath, _ = os.UserHomeDir()
		if startPath == "" {
			startPath = "/"
		}
	}
	b := &Browser{
		Path:     startPath,
		Title:    title,
		dirsOnly: dirsOnly,
		Menu:     NewMenu(nil),
	}
	b.load()
	return b
}

func (b *Browser) load() {
	b.Menu.Items = nil
	entries, err := os.ReadDir(b.Path)
	if err != nil {
		// Just show an empty list on error
		entries = nil
	}

	var filtered []os.DirEntry
	for _, e := range entries {
		if b.dirsOnly && !e.IsDir() {
			continue
		}
		if b.Filter != nil && !b.Filter(e) {
			continue
		}
		filtered = append(filtered, e)
	}

	sort.Slice(filtered, func(i, j int) bool {
		iDir, jDir := filtered[i].IsDir(), filtered[j].IsDir()
		if iDir != jDir {
			return iDir // directories first
		}
		return strings.ToLower(filtered[i].Name()) < strings.ToLower(filtered[j].Name())
	})

	items := []MenuItem{}
	if b.Path != "/" {
		items = append(items, MenuItem{Label: "..", Desc: "Go up"})
	}

	for _, e := range filtered {
		desc := ""
		if e.IsDir() {
			desc = "dir"
		}
		items = append(items, MenuItem{Label: e.Name(), Desc: desc})
	}
	b.Menu.Items = items
	b.cachedList = filtered
	b.Menu.Cursor = 0
	b.Menu.Offset = 0
}

func (b *Browser) Update(key string) {
	switch key {
	case "up", "k":
		b.Menu.Up()
	case "down", "j":
		b.Menu.Down()
	case "esc":
		if b.OnCancel != nil {
			b.OnCancel()
		}
	case "enter":
		i := b.Menu.Cursor
		if b.Path != "/" {
			if i == 0 {
				b.Path = filepath.Dir(b.Path)
				b.load()
				return
			}
			i--
		}

		if i < 0 || i >= len(b.cachedList) {
			return
		}

		e := b.cachedList[i]
		newPath := filepath.Join(b.Path, e.Name())

		if e.IsDir() {
			b.Path = newPath
			b.load()
		} else {
			if b.OnSelect != nil {
				b.OnSelect(newPath)
			}
		}
	case "right", "l":
		// Enter directory
		i := b.Menu.Cursor
		if b.Path != "/" {
			if i == 0 {
				return
			}
			i--
		}
		if i >= 0 && i < len(b.cachedList) {
			e := b.cachedList[i]
			if e.IsDir() {
				b.Path = filepath.Join(b.Path, e.Name())
				b.load()
			}
		}
	case "left", "h":
		// Go up
		if b.Path != "/" {
			b.Path = filepath.Dir(b.Path)
			b.load()
		}
	case "s", "S":
		if b.dirsOnly && b.OnSelect != nil {
			b.OnSelect(b.Path)
		}
	}
}

func (b *Browser) View(width int) string {
	boxW := width
	if boxW < 30 {
		boxW = 30
	}

	header := TitleStyle.Render(b.Title) + "\n" + SubtleStyle.Render(Truncate(b.Path, boxW-4)) + "\n\n"

	b.Menu.Height = 10 // default height, caller can override
	menuView := b.Menu.View(boxW - 4)

	footer := ""
	if b.dirsOnly {
		footer = "\n\n" + SubtleStyle.Render("enter/l open dir · s select current · esc cancel")
	} else {
		footer = "\n\n" + SubtleStyle.Render("enter select/open · esc cancel")
	}

	return BoxStyle.Width(boxW).Render(header + menuView + footer)
}
