package youtube

import "strings"

// wordCount counts whitespace-separated words.
func wordCount(s string) int { return len(strings.Fields(s)) }

// collapse squeezes runs of whitespace into single spaces.
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

// mmss formats seconds as "M:SS".
func mmss(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return itoa(sec/60) + ":" + pad2(sec%60)
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
		return "0" + itoa(n)
	}
	return itoa(n)
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

// stamp builds a Timestamp for [startSec, endSec].
func stamp(startSec, endSec int) Timestamp {
	if endSec < startSec {
		endSec = startSec
	}
	return Timestamp{
		Start:    clock(startSec),
		End:      clock(endSec),
		StartSec: startSec,
		EndSec:   endSec,
		Label:    clock(startSec) + "–" + clock(endSec),
	}
}

// dedupe removes duplicate strings, preserving order (case-insensitive key).
func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
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

// topStrings returns at most n items.
func topStrings(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}

// firstSentences returns up to n sentences of s. A '.'/'!'/'?' ends a sentence
// only when it is followed by whitespace or the end of the string, so version
// numbers and identifiers like "v0.2.0" are never split.
// capWordsAtSentence trims s to at most maxWords, cutting only on sentence
// boundaries (never mid-sentence) and always keeping the first sentence. Returns
// s unchanged when it already fits or maxWords <= 0.
func capWordsAtSentence(s string, maxWords int) string {
	s = collapse(s)
	if maxWords <= 0 || wordCount(s) <= maxWords {
		return s
	}
	var sents []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '.' && s[i] != '!' && s[i] != '?' {
			continue
		}
		if i+1 < len(s) && s[i+1] != ' ' { // mid-token dot (e.g. v0.2.0) — not a boundary
			continue
		}
		sents = append(sents, strings.TrimSpace(s[start:i+1]))
		start = i + 1
	}
	if tail := strings.TrimSpace(s[start:]); tail != "" {
		sents = append(sents, tail)
	}
	if len(sents) == 0 {
		return s
	}
	out := []string{sents[0]}
	w := wordCount(sents[0])
	for _, sent := range sents[1:] {
		n := wordCount(sent)
		if w+n > maxWords {
			break
		}
		out = append(out, sent)
		w += n
	}
	return strings.TrimSpace(strings.Join(out, " "))
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
		if i+1 < len(s) && s[i+1] != ' ' { // mid-token dot (e.g. v0.2.0) — not a boundary
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
