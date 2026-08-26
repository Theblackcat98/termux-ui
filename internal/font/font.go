// Package font downloads and installs terminal fonts into ~/.termux/font.ttf.
// Direct TTF URLs and GitHub release zips are supported; zips are unpacked
// natively so no external tools are required.
package font

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DevCoreXOfficial/termux-ui/internal/termuxconf"
)

// Entry describes one downloadable font.
type Entry struct {
	Name        string // display name, e.g. "JetBrainsMono NF"
	URL         string
	ZipMember   string // when the URL is a zip: preferred member name prefix
	Recommended bool   // recommended for powerlevel10k
}

const p10kMedia = "https://github.com/romkatv/powerlevel10k-media/raw/master/"

// Catalog lists the fonts offered on Screen 5.
func Catalog() []Entry {
	return []Entry{
		{Name: "MesloLGS NF Regular", URL: p10kMedia + "MesloLGS%20NF%20Regular.ttf", Recommended: true},
		{Name: "MesloLGS NF Bold", URL: p10kMedia + "MesloLGS%20NF%20Bold.ttf"},
		{Name: "MesloLGS NF Italic", URL: p10kMedia + "MesloLGS%20NF%20Italic.ttf"},
		{Name: "MesloLGS NF Bold Italic", URL: p10kMedia + "MesloLGS%20NF%20BoldItalic.ttf"},
		{Name: "JetBrainsMono NF", URL: "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.zip", ZipMember: "JetBrainsMonoNerdFont-Regular"},
		{Name: "FiraCode NF", URL: "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/FiraCode.zip", ZipMember: "FiraCodeNerdFont-Regular"},
		{Name: "CaskaydiaCove NF", URL: "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/CascadiaCode.zip", ZipMember: "CaskaydiaCoveNerdFont-Regular"},
		{Name: "Hack NF", URL: "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/Hack.zip", ZipMember: "HackNerdFont-Regular"},
	}
}

// FontPath is the installed terminal font location.
func FontPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".termux", "font.ttf")
}

// Installed reports whether a font.ttf exists (and its size).
func Installed() (bool, int64) {
	st, err := os.Stat(FontPath())
	if err != nil {
		return false, 0
	}
	return true, st.Size()
}

func fetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// IsTTF checks TrueType/OpenType magic bytes.
func IsTTF(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	magic := data[:4]
	return bytes.Equal(magic, []byte{0x00, 0x01, 0x00, 0x00}) ||
		bytes.Equal(magic, []byte("true")) ||
		bytes.Equal(magic, []byte("OTTO")) ||
		bytes.Equal(magic, []byte("ttcf"))
}

// ExtractFromZip returns the wanted TTF from a nerd-fonts style zip,
// preferring the member matching wantPrefix (falls back to any *.ttf).
func ExtractFromZip(data []byte, wantPrefix string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a readable zip: %v", err)
	}
	var fallback []byte
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(f.Name), ".ttf") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil || !IsTTF(content) {
			continue
		}
		base := filepath.Base(f.Name)
		if wantPrefix != "" && strings.HasPrefix(base, wantPrefix) && !strings.Contains(base, "Unhinted") {
			return content, nil
		}
		if fallback == nil && !strings.Contains(strings.ToLower(base), "italic") {
			fallback = content
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("no usable .ttf found inside zip")
}

// Install writes data (a TTF) to ~/.termux/font.ttf after backing up the
// current file, then triggers termux-reload-settings.
func Install(data []byte) error {
	if !IsTTF(data) {
		return fmt.Errorf("downloaded file is not a valid TTF font")
	}
	path := FontPath()
	if _, err := termuxconf.Backup(path, time.Now()); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// InstallURL downloads url (direct TTF or zip) and installs it.
func InstallURL(e Entry) error {
	data, err := fetch(e.URL)
	if err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(e.URL), ".zip") || !IsTTF(data) {
		data, err = ExtractFromZip(data, e.ZipMember)
		if err != nil {
			return err
		}
	}
	return Install(data)
}

// InstallLocal installs a local .ttf picked via the file browser.
func InstallLocal(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Install(data)
}

// RemoveFont deletes ~/.termux/font.ttf after confirm (caller confirms),
// restoring the built-in Termux font.
func RemoveFont() error {
	err := os.Remove(FontPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
