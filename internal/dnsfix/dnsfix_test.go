package dnsfix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNameservers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")
	content := "" +
		"# comment line\n" +
		"nameserver 8.8.8.8\n" +
		"nameserver 2001:4860:4860::8888 ; trailing comment\n" +
		"search lan\n" +
		"nameserver not-an-ip\n" +
		"nameserver\n" +
		"options edns0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ns, err := Nameservers(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 2 || ns[0] != "8.8.8.8" || ns[1] != "2001:4860:4860::8888" {
		t.Fatalf("got %v, want [8.8.8.8 2001:4860:4860::8888]", ns)
	}
}

func TestNameserversMissingFile(t *testing.T) {
	if _, err := Nameservers(filepath.Join(t.TempDir(), "absent.conf")); err == nil {
		t.Fatal("want error for missing file")
	}
}
