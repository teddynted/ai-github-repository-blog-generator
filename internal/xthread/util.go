package xthread

import (
	"strings"
	"unicode/utf8"
)

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// runeLen counts characters the way X does (approximately) — by rune.
func runeLen(s string) int { return utf8.RuneCountInString(s) }

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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func topStrings(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}

func firstSentences(s string, n int) string {
	s = collapse(s)
	if s == "" {
		return ""
	}
	var out []string
	start, count := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] != '.' && s[i] != '!' && s[i] != '?' {
			continue
		}
		if i+1 < len(s) && s[i+1] != ' ' {
			continue
		}
		out = append(out, strings.TrimSpace(s[start:i+1]))
		start = i + 1
		count++
		if count >= n {
			break
		}
	}
	if count == 0 {
		return s
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

// fitChars trims s to at most max runes on a word boundary, adding an ellipsis
// when it truncates. Used to keep every post within the platform limit.
func fitChars(s string, max int) string {
	s = collapse(s)
	if runeLen(s) <= max {
		return s
	}
	// Reserve one rune for the ellipsis.
	limit := max - 1
	runes := []rune(s)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	cut := string(runes)
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}

func slug(s string) string { return strings.ToLower(collapse(s)) }

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
