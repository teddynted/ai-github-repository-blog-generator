package releasecontext

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
)

// newContextID returns a random RFC 4122 version-4 UUID. Using crypto/rand
// keeps the package dependency-free (no third-party UUID library).
func newContextID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is catastrophic and effectively never happens;
		// fall back to a clearly-marked non-random id rather than panicking.
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func itoa(n int) string { return strconv.Itoa(n) }

// firstSentence returns the first sentence of s (up to the first period,
// newline, or a sensible length cap), trimmed of markdown noise.
func firstSentence(s string) string {
	s = strings.TrimSpace(stripMarkdown(s))
	if s == "" {
		return ""
	}
	for i, r := range s {
		if r == '\n' {
			return strings.TrimSpace(s[:i])
		}
		if r == '.' && i+1 < len(s) && (s[i+1] == ' ' || s[i+1] == '\n') {
			return strings.TrimSpace(s[:i+1])
		}
	}
	if len(s) > 240 {
		return strings.TrimSpace(s[:240]) + "…"
	}
	return s
}

// stripMarkdown removes the most common inline markdown so a summary reads as
// prose: emphasis markers, inline code backticks, link syntax, heading hashes.
func stripMarkdown(s string) string {
	replacer := strings.NewReplacer("**", "", "__", "", "`", "", "#", "")
	s = replacer.Replace(s)
	// Collapse [text](url) -> text.
	for {
		open := strings.IndexByte(s, '[')
		if open < 0 {
			break
		}
		close := strings.IndexByte(s[open:], ']')
		if close < 0 {
			break
		}
		close += open
		if close+1 < len(s) && s[close+1] == '(' {
			end := strings.IndexByte(s[close:], ')')
			if end < 0 {
				break
			}
			s = s[:open] + s[open+1:close] + s[close+end+1:]
		} else {
			s = s[:open] + s[open+1:close] + s[close+1:]
		}
	}
	return strings.TrimSpace(s)
}

// dedupeStrings returns list with duplicates removed, preserving order.
func dedupeStrings(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, s := range list {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// topN returns at most n items from list.
func topN(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}
