package font

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

var fakeTTF = append([]byte{0x00, 0x01, 0x00, 0x00}, bytes.Repeat([]byte{0xAB}, 64)...)

func TestIsTTF(t *testing.T) {
	if !IsTTF(fakeTTF) {
		t.Fatal("ttc/ttf magic not accepted")
	}
	for _, bad := range [][]byte{{}, {0, 0, 0}, []byte("<html>"), []byte("PK\x03\x04")} {
		if IsTTF(bad) {
			t.Fatalf("non-ttf accepted: %x", bad)
		}
	}
}

func zipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	zw.Close()
	return buf.Bytes()
}

func TestExtractFromZipPrefersRequestedMember(t *testing.T) {
	data := zipBytes(t, map[string][]byte{
		"JetBrainsMonoNerdFont-Regular.ttf":    fakeTTF,
		"JetBrainsMonoNerdFont-BoldItalic.ttf": {0xFF, 0xFF},
		"JetBrainsMonoNerdFont-Italic.ttf":     {0xEE, 0xEE},
	})
	got, err := ExtractFromZip(data, "JetBrainsMonoNerdFont-Regular")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fakeTTF) {
		t.Fatal("wrong member extracted")
	}
}

func TestExtractFromZipFallsBackToAnyNonItalic(t *testing.T) {
	data := zipBytes(t, map[string][]byte{
		"SomeNerdFont-Italic.ttf":  {0xEE},
		"SomeNerdFont-Regular.ttf": fakeTTF,
	})
	got, err := ExtractFromZip(data, "Missing")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fakeTTF) {
		t.Fatal("fallback picked the wrong member")
	}
}

func TestInstallWritesAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	os.MkdirAll(filepath.Join(dir, ".termux"), 0o700)
	os.WriteFile(FontPath(), []byte("old-font"), 0o644)

	if err := Install(fakeTTF); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(FontPath())
	if err != nil || !bytes.Equal(data, fakeTTF) {
		t.Fatalf("font.ttf wrong after install")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".termux", "font.ttf.bak.*"))
	if len(matches) != 1 {
		t.Fatalf("expected one backup, got %v", matches)
	}
	if err := Install([]byte("not a font")); err == nil {
		t.Fatal("garbage must be rejected before touching font.ttf")
	}
}
