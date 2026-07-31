package voiceover

import (
	"strings"
	"unicode"
)

// wordCount counts whitespace-separated words.
func wordCount(s string) int { return len(strings.Fields(s)) }

// collapse squeezes runs of whitespace into single spaces.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// capitalizeFirst upper-cases the first letter of s so narration never opens on
// a lower-case word (a refinement model occasionally returns "every time..."
// instead of "Every time..."). Leading non-letters (quotes) are skipped; already
// capitalized text is returned unchanged.
func capitalizeFirst(s string) string {
	for i, r := range s {
		if unicode.IsLetter(r) {
			if !unicode.IsUpper(r) {
				s = s[:i] + string(unicode.ToUpper(r)) + s[i+len(string(r)):]
			}
			break
		}
	}
	return s
}

// clampInt bounds v to [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// mmss formats seconds as "M:SS".
func mmss(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return fmtInt(sec/60) + ":" + pad2(sec%60)
}

// clock formats seconds as a zero-padded "MM:SS" timeline stamp.
func clock(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return pad2(sec/60) + ":" + pad2(sec%60)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + fmtInt(n)
	}
	return fmtInt(n)
}

// fmtInt renders a non-negative int without importing strconv at call sites.
func fmtInt(n int) string {
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

// sentences splits prose into sentences on ., !, ? boundaries.
func sentences(s string) []string {
	s = collapse(s)
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == '!' || s[i] == '?' {
			if seg := strings.TrimSpace(s[start : i+1]); seg != "" {
				out = append(out, seg)
			}
			start = i + 1
		}
	}
	if seg := strings.TrimSpace(s[start:]); seg != "" {
		out = append(out, seg)
	}
	return out
}

// firstSentence returns the first sentence of s (or the whole string).
func firstSentence(s string) string {
	if ss := sentences(s); len(ss) > 0 {
		return ss[0]
	}
	return strings.TrimSpace(s)
}

// containsWord reports whether needle appears in haystack as a whole,
// case-insensitive word/phrase (bounded by non-alphanumeric characters).
func containsWord(haystack, needle string) bool {
	h := strings.ToLower(haystack)
	n := strings.ToLower(needle)
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

// boundary reports whether the rune at index i is a word boundary (or the
// position is off either end of s).
func boundary(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return true
	}
	r := rune(s[i])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// dedupe returns the input with duplicate strings removed, preserving order.
func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		key := strings.ToLower(strings.TrimSpace(s))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// firstNonEmpty returns the first non-blank string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
