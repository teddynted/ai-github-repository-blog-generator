package xthread

import "strings"

// snippetMaxChars keeps a code snippet short enough to sit inside a post.
const snippetMaxChars = 180

// extractSnippet returns a short, grounded code snippet from the blog's fenced
// code blocks (excluding Mermaid diagrams). It never fabricates code — if the
// blog has no suitable block, it returns "".
func extractSnippet(pkg ReleasePackage) string {
	blocks := fencedBlocks(pkg.Blog.Markdown)
	// Prefer the shortest non-trivial block so it fits in a post.
	best := ""
	for _, b := range blocks {
		code := strings.TrimSpace(b)
		if code == "" || runeLen(code) > snippetMaxChars {
			continue
		}
		if best == "" || runeLen(code) < runeLen(best) {
			best = code
		}
	}
	return best
}

// hasCode reports whether the blog contains an extractable code snippet.
func hasCode(pkg ReleasePackage) bool { return extractSnippet(pkg) != "" }

// fencedBlocks returns the fenced code blocks in markdown, excluding Mermaid.
func fencedBlocks(markdown string) []string {
	if markdown == "" {
		return nil
	}
	var blocks []string
	lines := strings.Split(markdown, "\n")
	in := false
	lang := ""
	var buf []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if !in && strings.HasPrefix(t, "```") {
			in = true
			lang = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(t, "```")))
			buf = nil
			continue
		}
		if in && t == "```" {
			in = false
			if lang != "mermaid" {
				blocks = append(blocks, strings.Join(buf, "\n"))
			}
			continue
		}
		if in {
			buf = append(buf, ln)
		}
	}
	return blocks
}
