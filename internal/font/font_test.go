package font

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestFetchSizeLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write 150MB + 2 bytes to exceed limit
		w.Header().Set("Content-Length", fmt.Sprintf("%d", 150*1024*1024+2))
		buf := make([]byte, 1024*1024)
		for i := 0; i < 151; i++ {
			w.Write(buf)
		}
	}))
	defer ts.Close()

	_, err := fetch(ts.URL, "")
	if err == nil {
		t.Fatal("expected error due to size limit, got nil")
	}
	if !strings.Contains(err.Error(), "exceeded size limit") && !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("expected size limit error, got: %v", err)
	}
}

func TestFetchSha256Mismatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("some data"))
	}))
	defer ts.Close()

	_, err := fetch(ts.URL, "invalidhash")
	if err == nil {
		t.Fatal("expected error due to sha256 mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch error, got: %v", err)
	}
}

func TestFetchSuccess(t *testing.T) {
	content := []byte("valid font data")
	hash := sha256.Sum256(content)
	expectedHash := hex.EncodeToString(hash[:])

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer ts.Close()

	data, err := fetch(ts.URL, expectedHash)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("expected %q, got %q", content, data)
	}
}
