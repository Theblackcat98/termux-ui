// Package termuxconf implements a comment-preserving reader/writer for Java
// `.properties` files as used by Termux (~/.termux/*.properties).
//
// Semantics follow java.util.Properties: `key=value` (also `key:value`),
// `#`/`!` comments, backslash line continuations and standard escapes
// (\n \t \r \f \\ \uXXXX). Every write goes through Backup+Save so the
// original file is preserved as <orig>.bak.<YYYYmmdd-HHMMSS>.
package termuxconf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// record is one logical unit of the document: either a run of comment/blank
// lines kept verbatim, or one property (possibly spanning several physical
// lines through backslash continuations).
type record struct {
	lines  []string // verbatim physical lines
	isProp bool
	key    string
	value  string // decoded value
}

// File is a parsed properties document.
type File struct {
	Records []record
}

// Load parses the file at path. A missing file yields an empty document.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, err
	}
	return Parse(string(data)), nil
}

// Parse decodes properties text into a File.
func Parse(text string) *File {
	f := &File{}
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	for i := 0; i < len(raw); i++ {
		trimmed := strings.TrimLeft(raw[i], " \t\f")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			f.Records = append(f.Records, record{lines: []string{raw[i]}})
			continue
		}
		// Property logical line: gather continuation lines.
		logical := []string{raw[i]}
		for continues(raw[i]) && i+1 < len(raw) {
			i++
			logical = append(logical, raw[i])
		}
		key, val := splitKeyValue(logical)
		f.Records = append(f.Records, record{lines: logical, isProp: true, key: key, value: val})
	}
	return f
}

// continues reports whether a physical line ends with an odd number of
// backslashes (i.e. the last backslash is a continuation marker).
func continues(line string) bool {
	n := 0
	for i := len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// stripContinuation removes one trailing continuation backslash.
func stripContinuation(s string) string {
	if continues(s) {
		s = s[:len(s)-1]
	}
	return s
}

// splitKeyValue parses a logical property line (already gathered physical
// lines) into key and decoded value.
func splitKeyValue(logical []string) (string, string) {
	// Join continuations: drop trailing backslash, concatenate without newline
	// (java.util.Properties drops leading whitespace of continuation lines).
	var b strings.Builder
	for idx, l := range logical {
		if idx > 0 {
			l = strings.TrimLeft(l, " \t\f")
		}
		b.WriteString(stripContinuation(l))
	}
	line := b.String()

	// Find key terminator: first unescaped separator (= : or whitespace).
	var key strings.Builder
	i := 0
	for ; i < len(line); i++ {
		c := line[i]
		if c == '\\' && i+1 < len(line) {
			key.WriteByte(c)
			key.WriteByte(line[i+1])
			i++
			continue
		}
		if c == '=' || c == ':' || c == ' ' || c == '\t' || c == '\f' {
			break
		}
		key.WriteByte(c)
	}
	k := decodeEscapes(key.String())

	// Skip whitespace, then an optional single = or :, then whitespace.
	j := i
	for j < len(line) && (line[j] == ' ' || line[j] == '\t' || line[j] == '\f') {
		j++
	}
	if j < len(line) && (line[j] == '=' || line[j] == ':') {
		j++
		for j < len(line) && (line[j] == ' ' || line[j] == '\t' || line[j] == '\f') {
			j++
		}
	} else if j == i && i < len(line) {
		// Separator was whitespace and no = /: followed: value starts here.
		j = i
	}
	return k, decodeEscapes(line[j:])
}

// decodeEscapes expands java.util.Properties escape sequences.
func decodeEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'f':
			b.WriteByte('\f')
		case 'u':
			if i+4 < len(s) {
				var r rune
				ok := true
				for k := 0; k < 4; k++ {
					d := hexVal(s[i+1+k])
					if d < 0 {
						ok = false
						break
					}
					r = r*16 + rune(d)
				}
				if ok {
					b.WriteRune(r)
					i += 4
					continue
				}
			}
			b.WriteString("\\u")
		default:
			b.WriteByte(s[i]) // \\ \/ \= \: \  etc. -> literal char
		}
	}
	return b.String()
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// escapeChars escapes one logical value into its single-line encoded form
// following java.util.Properties rules.
func escapeChars(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\f':
			b.WriteString(`\f`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

const escKeyChars = "=:# !\t\f"

// encodeKey escapes a key for writing.
func encodeKey(k string) string {
	var b strings.Builder
	for _, r := range k {
		if strings.ContainsRune(escKeyChars, r) || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// WrapEncoded splits an already-encoded `key=value` text across physical
// lines using backslash continuations. Java decoding joins continuations with
// no separator, so the decoded value is byte-identical to the single-line
// form. Splits happen only right after '[' or ',' so no escape sequence is
// ever cut in half; leading whitespace of continuation lines is dropped by
// the decoder, which keeps structural spacing readable.
func WrapEncoded(keyLine string) []string {
	const maxLen = 72
	if len(keyLine) <= maxLen {
		return []string{keyLine}
	}
	var lines []string
	cur := ""
	for i := 0; i < len(keyLine); i++ {
		cur += string(keyLine[i])
		if len(cur) >= maxLen && (keyLine[i] == ',' || keyLine[i] == '[') && i < len(keyLine)-1 {
			lines = append(lines, cur+"\\")
			cur = " "
		}
	}
	if strings.TrimSpace(cur) != "" {
		lines = append(lines, cur)
	}
	return lines
}

// Set writes key=value, replacing any existing entry in place (its position
// among comments is kept); unknown keys append at the end. Long values are
// stored in the canonical multiline backslash-continued form.
func (f *File) Set(key, value string) {
	encoded := encodeKey(key) + "=" + escapeChars(value)
	f.setLines(key, value, WrapEncoded(encoded))
}

// SetWrapped stores key=value in forced multiline backslash-continued form.
// Layouts break after row separators ("],") so each row reads on its own
// line, matching Termux's canonical pretty extra-keys form; if no row
// separator exists it falls back to width-based wrapping.
func (f *File) SetWrapped(key, value string) {
	encoded := encodeKey(key) + "=" + escapeChars(value)
	lines := wrapAfterRowSeparators(encoded)
	if len(lines) <= 1 {
		lines = WrapEncoded(encoded)
	}
	f.setLines(key, value, lines)
}

func wrapAfterRowSeparators(encoded string) []string {
	if !strings.Contains(encoded, "],") {
		return []string{encoded}
	}
	var lines []string
	cur := ""
	for i := 0; i < len(encoded); i++ {
		cur += string(encoded[i])
		if encoded[i] == ',' && i > 0 && encoded[i-1] == ']' && i < len(encoded)-1 {
			lines = append(lines, cur+"\\")
			cur = " "
		}
	}
	if s := strings.TrimSpace(cur); s != "" {
		lines = append(lines, s)
	}
	return lines
}

func (f *File) setLines(key, value string, lines []string) {
	for i := range f.Records {
		if f.Records[i].isProp && f.Records[i].key == key {
			f.Records[i].lines = lines
			f.Records[i].value = value
			return
		}
	}
	if n := len(f.Records); n > 0 && f.Records[n-1].isProp {
		f.Records = append(f.Records, record{lines: []string{""}})
	}
	f.Records = append(f.Records, record{lines: lines, isProp: true, key: key, value: value})
}

// Render serializes the document back to text, byte-preserving untouched parts.
func (f *File) Render() string {
	var b strings.Builder
	first := true
	for _, rec := range f.Records {
		for _, l := range rec.lines {
			if !first {
				b.WriteByte('\n')
			}
			first = false
			b.WriteString(l)
		}
	}
	return b.String()
}

// Get returns the decoded value of key, or def when absent.
func (f *File) Get(key, def string) string {
	for i := range f.Records {
		if f.Records[i].isProp && f.Records[i].key == key {
			return f.Records[i].value
		}
	}
	return def
}

// Has reports whether the key exists in the file.
func (f *File) Has(key string) bool {
	for i := range f.Records {
		if f.Records[i].isProp && f.Records[i].key == key {
			return true
		}
	}
	return false
}

// Unset removes the key's record entirely (used by "reset to default").
func (f *File) Unset(key string) bool {
	for i := range f.Records {
		if f.Records[i].isProp && f.Records[i].key == key {
			f.Records = append(f.Records[:i], f.Records[i+1:]...)
			return true
		}
	}
	return false
}

// Keys lists all property keys in document order.
func (f *File) Keys() []string {
	var out []string
	for _, rec := range f.Records {
		if rec.isProp {
			out = append(out, rec.key)
		}
	}
	return out
}

// BackupPath returns <orig>.bak.<YYYYmmdd-HHMMSS> for the given time.
func BackupPath(orig string, t time.Time) string {
	return orig + ".bak." + t.Format("20060102-150405")
}

// Backup copies the current file to a timestamped sibling. Missing originals
// are not backed up (nothing to preserve).
func Backup(orig string, now time.Time) (string, error) {
	data, err := os.ReadFile(orig)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	dst := BackupPath(orig, now)
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return "", err
	}
	return dst, nil
}

// Save backs up the existing file (if any) and atomically writes content.
func Save(path, content string, now time.Time) (backup string, err error) {
	if backup, err = Backup(path, now); err != nil {
		return "", fmt.Errorf("backup failed: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return backup, err
	}
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return backup, err
	}
	return backup, os.Rename(tmp, path)
}

// ReloadSettings runs termux-reload-settings. It returns a descriptive error
// when the command is unavailable (e.g. running outside Termux during dev).
func ReloadSettings() error {
	if _, err := os.Stat("/data/data/com.termux/files/usr/bin/termux-reload-settings"); err != nil {
		if os.Getenv("TERMUX_VERSION") == "" {
			return fmt.Errorf("termux-reload-settings not available outside Termux")
		}
	}
	out, err := runCmd("termux-reload-settings")
	if err != nil {
		return fmt.Errorf("termux-reload-settings: %v: %s", err, out)
	}
	return nil
}
