package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DevCoreXOfficial/termux-ui/internal/catalog"
	"github.com/DevCoreXOfficial/termux-ui/internal/termuxconf"
)

func catalogPresets() []catalog.Preset { return catalog.Presets }

func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func parseForTest(t *testing.T, text string) *termuxconf.File {
	t.Helper()
	return termuxconf.Parse(text)
}

func newestBackup(t *testing.T, dir, prefix string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best := ""
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			best = filepath.Join(dir, e.Name())
		}
	}
	return best
}
