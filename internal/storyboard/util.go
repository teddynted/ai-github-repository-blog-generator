package storyboard

import (
	"regexp"
	"strings"
	"unicode"
)

// splitExtension matches a filename whose extension was separated by a stray
// space ("ami-manifest. json"), which TTS reads with an awkward pause and breaks
// path detection. Lowercase extensions only, so a real sentence boundary
// ("...the config. JSON output...") is left alone.
var splitExtension = regexp.MustCompile(`([A-Za-z0-9_-])\.\s+(json|ya?ml|sh|go|py|toml|cfg|env|txt|md|lock)\b`)

// joinFileExtensions rejoins a filename split from its extension by a stray space
// ("manifest. json" -> "manifest.json"). Must run before file-path handling and
// before timing.
func joinFileExtensions(s string) string { return splitExtension.ReplaceAllString(s, "$1.$2") }

// filePathArticle matches an indefinite article before an absolute file path
// ("an /etc/ami-manifest.json"), which neural TTS reads awkwardly.
var filePathArticle = regexp.MustCompile(`(?i)\b(an?)\s+(/[A-Za-z0-9._/-]*[A-Za-z0-9])`)

// bareAbsPath matches an absolute path at a word boundary with no leading
// article. The leading-slash requirement keeps it off mid-token slashes like
// "and/or" or "24/7".
var bareAbsPath = regexp.MustCompile(`(^|\s)(/[A-Za-z0-9._/-]*[A-Za-z0-9])`)

// spellPath renders an absolute file path as spoken words:
// "/etc/ami-manifest.json" -> "slash etc slash ami-manifest dot json".
func spellPath(p string) string {
	p = strings.ReplaceAll(p, "/", " slash ")
	p = strings.ReplaceAll(p, ".", " dot ")
	return strings.Join(strings.Fields(p), " ")
}

// speakFilePaths converts absolute file paths into spoken form ("an <path>" ->
// "the file <spelled path>", bare paths spelled in place). It adds words, so it
// must run before scene timing so they are counted against the slot.
func speakFilePaths(s string) string {
	s = filePathArticle.ReplaceAllStringFunc(s, func(m string) string {
		sub := filePathArticle.FindStringSubmatch(m)
		the := "the"
		if sub[1][0] == 'A' {
			the = "The"
		}
		return the + " file " + spellPath(sub[2])
	})
	return bareAbsPath.ReplaceAllStringFunc(s, func(m string) string {
		sub := bareAbsPath.FindStringSubmatch(m)
		return sub[1] + spellPath(sub[2])
	})
}

// capitalizeFirst upper-cases the first letter of s so a scene's narration never
// opens on a lower-case word. Leading non-letters are skipped; already
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

// codeBlock is a fenced code block extracted from Markdown.
type codeBlock struct {
	Lang string
	Code string
}

// fencedBlocks returns the fenced code blocks in body, tagged with their info
// string (language). Mermaid blocks are excluded — those are diagrams, handled
// by the diagram planner.
func fencedBlocks(body string) []codeBlock {
	var blocks []codeBlock
	lines := strings.Split(body, "\n")
	in := false
	lang := ""
	var buf []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if !in && strings.HasPrefix(t, "```") {
			in = true
			lang = strings.TrimSpace(strings.TrimPrefix(t, "```"))
			buf = nil
			continue
		}
		if in && t == "```" {
			in = false
			if strings.ToLower(lang) != "mermaid" {
				blocks = append(blocks, codeBlock{Lang: strings.ToLower(lang), Code: strings.Join(buf, "\n")})
			}
			continue
		}
		if in {
			buf = append(buf, ln)
		}
	}
	return blocks
}

// prose returns the body with fenced code blocks and inline markdown removed,
// so narration is drawn only from spoken-word content.
func prose(body string) string {
	lines := strings.Split(body, "\n")
	in := false
	var out []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") {
			in = !in
			continue
		}
		if in {
			continue
		}
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "|") || strings.HasPrefix(t, ">") {
			continue
		}
		out = append(out, t)
	}
	return cleanInline(strings.Join(out, " "))
}

// isSentenceBoundary reports whether s[i] ends a sentence: a '.', '!' or '?'
// followed by whitespace or end-of-text. Requiring the trailing space avoids
// splitting inside decimals and version numbers like "1.0.0", where the dot is
// followed by a digit rather than a space.
func isSentenceBoundary(s string, i int) bool {
	if c := s[i]; c != '.' && c != '!' && c != '?' {
		return false
	}
	return i+1 >= len(s) || s[i+1] == ' ' || s[i+1] == '\t' || s[i+1] == '\n'
}

// firstSentences returns up to n sentences of s, also capped at maxWords.
func firstSentences(s string, n, maxWords int) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	// Collect up to n whole sentences.
	var sents []string
	start := 0
	for i := 0; i < len(s) && len(sents) < n; i++ {
		if isSentenceBoundary(s, i) {
			sents = append(sents, strings.TrimSpace(s[start:i+1]))
			start = i + 1
		}
	}
	if len(sents) == 0 {
		sents = []string{s}
	}
	// Bound length to maxWords on SENTENCE boundaries — never mid-sentence. A
	// hard word-cap here used to clip the draft mid-clause with an ellipsis
	// ("…an OS update, then installs of…"), which is unspeakable as narration
	// and, worse, drops the very facts the model needs (it then either reproduces
	// the truncation or is forced to invent). Keep whole sentences until the next
	// would blow the budget; always keep the first sentence complete, even if it
	// alone exceeds the cap.
	out := []string{sents[0]}
	words := wordCount(sents[0])
	for _, sent := range sents[1:] {
		w := wordCount(sent)
		if words+w > maxWords {
			break
		}
		out = append(out, sent)
		words += w
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

func capWords(s string, max int) string {
	f := strings.Fields(s)
	if len(f) <= max {
		return s
	}
	return strings.Join(f[:max], " ") + "…"
}

func wordCount(s string) int { return len(strings.Fields(s)) }

func truncate(s string, max int) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…"
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

func topStrings(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}
