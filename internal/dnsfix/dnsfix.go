// Package dnsfix keeps DNS working for Go's pure-Go resolver on Android.
//
// Android has no /etc/resolv.conf, so the pure-Go resolver falls back to its
// built-in nameservers (127.0.0.1, ::1) where nothing listens: every lookup
// fails with "dial udp [::1]:53: connection refused". Termux ships the real
// resolver config at $PREFIX/etc/resolv.conf instead. This package reads it
// and points net.DefaultResolver at those servers before the first lookup.
//
// Everything is a no-op unless running under GOOS=android with a missing
// /etc/resolv.conf; every other platform keeps the default resolver.
//
// Why GOOS=android matters beyond DNS: Android 11's seccomp filter kills any
// GOOS=linux binary the first time Go's os/exec probes a file with
// faccessat2 (SIGSYS) — which used to hit core detection the moment
// core-termux was installed. Building with GOOS=android makes internal Go
// paths avoid faccessat2 entirely, so this project must always target
// GOOS=android, never GOOS=linux.
package dnsfix

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// termuxResolvConf is $PREFIX/etc/resolv.conf on a standard Termux install,
// used when $PREFIX is unset ( Termux always sets it, this is just belt and
// braces).
const termuxResolvConf = "/data/data/com.termux/files/usr/etc/resolv.conf"

// Nameservers parses "nameserver <host>" lines out of a resolv.conf file.
// Comments (# and ;) are stripped; invalid addresses are skipped.
func Nameservers(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ns []string
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.IndexAny(line, "#;"); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) >= 2 && f[0] == "nameserver" && net.ParseIP(f[1]) != nil {
			ns = append(ns, f[1])
		}
	}
	return ns, nil
}

// Apply redirects the default resolver to Termux's resolv.conf when needed.
// Call it once at startup, before anything does network I/O.
func Apply() {
	if runtime.GOOS != "android" {
		return
	}
	if _, err := os.Stat("/etc/resolv.conf"); err == nil {
		return
	}

	path := termuxResolvConf
	if prefix := os.Getenv("PREFIX"); prefix != "" {
		p := filepath.Join(prefix, "etc", "resolv.conf")
		if _, err := os.Stat(p); err == nil {
			path = p
		}
	}
	ns, err := Nameservers(path)
	if err != nil || len(ns) == 0 {
		return
	}

	var next uint32
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			// The address argument is the resolver's built-in default
			// (loopback) — exactly what is broken on Android — so dial the
			// configured servers instead, round-robin.
			server := ns[atomic.AddUint32(&next, 1)%uint32(len(ns))]
			var d net.Dialer
			d.Timeout = 5 * time.Second
			return d.DialContext(ctx, network, net.JoinHostPort(server, "53"))
		},
	}
}
