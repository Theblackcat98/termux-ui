package app

import (
	"os"
	"regexp"
	"testing"
)

func TestCanonicalRepository(t *testing.T) {
	const canonicalRepo = "Theblackcat98/termux-ui"

	// Check install.sh
	installSh, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatalf("failed to read install.sh: %v", err)
	}
	if !regexp.MustCompile(`REPO="` + canonicalRepo + `"`).Match(installSh) {
		t.Errorf("install.sh does not contain canonical repo %s", canonicalRepo)
	}

	// Check README.md
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}
	if !regexp.MustCompile(`https://raw\.githubusercontent\.com/` + canonicalRepo + `/main/install\.sh`).Match(readme) {
		t.Errorf("README.md does not contain canonical repo %s", canonicalRepo)
	}

	// Check dashboard.go
	dashboardGo, err := os.ReadFile("dashboard.go")
	if err != nil {
		t.Fatalf("failed to read dashboard.go: %v", err)
	}
	if !regexp.MustCompile(`repoAPI = "https://api\.github\.com/repos/` + canonicalRepo + `/releases/latest"`).Match(dashboardGo) {
		t.Errorf("dashboard.go does not contain canonical repo %s in repoAPI", canonicalRepo)
	}
	if !regexp.MustCompile(`https://github\.com/` + canonicalRepo + `/releases/latest/download/termux-ui_linux_%s`).Match(dashboardGo) {
		t.Errorf("dashboard.go does not contain canonical repo %s in asset URL", canonicalRepo)
	}
}
