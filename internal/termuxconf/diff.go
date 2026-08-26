package termuxconf

import (
	"fmt"
	"os/exec"
	"strings"
)

func runCmd(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// UnifiedDiff renders a unified diff (---/+++ header, @@ hunks, +/- lines)
// between two file contents. It uses a longest-common-subsequence over lines,
// which is fine for Termux config sizes (< a few thousand lines).
func UnifiedDiff(oldLabel, newLabel, oldText, newText string) string {
	a := strings.Split(strings.ReplaceAll(oldText, "\r\n", "\n"), "\n")
	b := strings.Split(strings.ReplaceAll(newText, "\r\n", "\n"), "\n")
	trimNL := func(ls []string) []string {
		if n := len(ls); n > 0 && ls[n-1] == "" {
			return ls[:n-1]
		}
		return ls
	}
	a, b = trimNL(a), trimNL(b)

	type op struct {
		kind byte // ' ', '-', '+'
		i, j int
	}
	// LCS table.
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []op
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, op{' ', i, j})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, op{'-', i, j})
			i++
		default:
			ops = append(ops, op{'+', i, j})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, op{'-', i, j})
	}
	for ; j < m; j++ {
		ops = append(ops, op{'+', i, j})
	}

	if len(ops) == 0 {
		return "" // no changes
	}
	anyChange := false
	for _, o := range ops {
		if o.kind != ' ' {
			anyChange = true
			break
		}
	}
	if !anyChange {
		return ""
	}

	// Group ops into hunks with at most CTX context lines around changes.
	const ctx = 3
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", oldLabel, newLabel)

	start := 0
	for start < len(ops) {
		if ops[start].kind == ' ' {
			start++
			continue
		}
		h0 := start
		if h0-ctx > 0 {
			h0 -= ctx
		} else {
			h0 = 0
		}
		end := start
		lastChange := start
		for k := start; k < len(ops); k++ {
			if ops[k].kind != ' ' {
				lastChange = k
			}
			if k-lastChange > 2*ctx {
				break
			}
			end = k
		}
		if end+ctx < len(ops) {
			end += ctx
		} else {
			end = len(ops) - 1
		}
		start = lastChange + 1

		ai, aj := 1, 1 // count preceding lines for @@ header
		for k := 0; k < h0; k++ {
			switch ops[k].kind {
			case ' ', '-':
				ai++
			case '+':
				aj++
			}
		}
		acnt, jcnt := 0, 0
		for k := h0; k <= end; k++ {
			switch ops[k].kind {
			case ' ':
				acnt++
				jcnt++
			case '-':
				acnt++
			case '+':
				jcnt++
			}
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", ai, acnt, aj, jcnt)
		for k := h0; k <= end; k++ {
			o := ops[k]
			lineNo := ""
			switch o.kind {
			case ' ':
				lineNo = "  " + a[o.i]
			case '-':
				lineNo = "- " + a[o.i]
			case '+':
				lineNo = "+ " + b[o.j]
			}
			out.WriteString(lineNo)
			out.WriteByte('\n')
		}
	}
	return out.String()
}
