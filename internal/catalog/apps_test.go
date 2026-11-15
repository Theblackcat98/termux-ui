package catalog

import (
	"regexp"
	"strings"
	"testing"
)

var pkgNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*$`)

func TestCommonAppsCatalogSanity(t *testing.T) {
	seen := map[string]bool{}
	total := 0
	for _, cat := range AppCategories {
		if cat.Key == "" || cat.Name == "" {
			t.Fatalf("category with empty key or name: %+v", cat)
		}
		if len(cat.Apps) == 0 {
			t.Fatalf("category %s has no apps", cat.Key)
		}
		for _, a := range cat.Apps {
			total++
			if a.Name == "" || a.Summary == "" {
				t.Errorf("%s: app with empty name or summary: %+v", cat.Key, a)
			}
			if !pkgNameRe.MatchString(a.Pkg) {
				t.Errorf("%s: invalid pkg name %q", cat.Key, a.Pkg)
			}
			if seen[a.Pkg] {
				t.Errorf("duplicate pkg %q", a.Pkg)
			}
			seen[a.Pkg] = true
			if a.Command != "" && strings.ContainsAny(a.Command, " /") {
				t.Errorf("%s: command %q should be a single binary name", a.Name, a.Command)
			}
		}
	}
	if got := len(AllCommonApps()); got != total {
		t.Fatalf("AllCommonApps flattened %d apps, want %d", got, total)
	}
	if total < 20 {
		t.Fatalf("catalog suspiciously small: %d apps", total)
	}
}
