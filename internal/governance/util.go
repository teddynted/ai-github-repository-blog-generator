package governance

import (
	"strings"
	"unicode"
)

func wordCount(s string) int { return len(strings.Fields(s)) }

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// sentences splits prose into sentences on ./!/? boundaries (boundary requires a
// following space or end-of-string, so identifiers like v0.2.0 are not split).
func sentences(s string) []string {
	s = collapse(s)
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '.' && s[i] != '!' && s[i] != '?' {
			continue
		}
		if i+1 < len(s) && s[i+1] != ' ' {
			continue
		}
		if seg := strings.TrimSpace(s[start : i+1]); seg != "" {
			out = append(out, seg)
		}
		start = i + 1
	}
	if seg := strings.TrimSpace(s[start:]); seg != "" {
		out = append(out, seg)
	}
	return out
}

// containsWord reports whether needle appears in haystack as a whole word
// (case-insensitive, bounded by non-alphanumerics).
func containsWord(haystack, needle string) bool {
	h := strings.ToLower(haystack)
	n := strings.ToLower(strings.TrimSpace(needle))
	if n == "" {
		return false
	}
	from := 0
	for {
		idx := strings.Index(h[from:], n)
		if idx < 0 {
			return false
		}
		i := from + idx
		if boundary(h, i-1) && boundary(h, i+len(n)) {
			return true
		}
		from = i + 1
		if from >= len(h) {
			return false
		}
	}
}

func boundary(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return true
	}
	r := rune(s[i])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func topStrings(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
