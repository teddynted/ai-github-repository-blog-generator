package governance

import (
	"regexp"
	"strings"
)

// minBodyWords is the completeness floor below which content is considered a stub.
const minBodyWords = 20

var (
	linkRe        = regexp.MustCompile(`\[[^\]]*\]\(([^)]*)\)`)
	imageRe       = regexp.MustCompile(`!\[[^\]]*\]\(([^)]*)\)`)
	doubleSpaceRe = regexp.MustCompile(`[ \t]{2,}`)
	placeholderRe = regexp.MustCompile(`(?i)\b(TODO|TBD|FIXME|lorem ipsum|xxx+|placeholder)\b`)
)

// hasRepeatedWord reports an adjacent duplicated word (e.g. "the the"). Go's
// RE2 regexp has no backreferences, so this is a deterministic scan.
func hasRepeatedWord(body string) bool {
	fields := strings.Fields(body)
	prev := ""
	for _, f := range fields {
		w := strings.ToLower(strings.TrimFunc(f, func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))
		}))
		if w != "" && w == prev && len(w) > 1 {
			return true
		}
		prev = w
	}
	return false
}

// ValidationEngine performs deterministic content validation.
type ValidationEngine struct{}

// Validate runs every deterministic check appropriate to the content type and
// returns a report. This is authoritative and reproducible — it never calls a model.
func (ValidationEngine) Validate(c Content) ValidationReport {
	var issues []ValidationIssue
	add := func(sev Severity, code, msg, field string) {
		issues = append(issues, ValidationIssue{Severity: sev, Code: code, Message: msg, Field: field})
	}

	body := c.Body

	// Completeness.
	if strings.TrimSpace(body) == "" {
		add(SeverityError, "empty", "content body is empty", "body")
	} else if wordCount(body) < minBodyWords {
		add(SeverityWarning, "too-short", "content is very short; it may be incomplete", "body")
	}
	if strings.TrimSpace(c.Title) == "" {
		add(SeverityWarning, "no-title", "content has no title", "title")
	}

	// Placeholder / stub text.
	if placeholderRe.MatchString(body) {
		add(SeverityError, "placeholder", "content contains placeholder text (TODO/TBD/lorem ipsum)", "body")
	}

	// Grammar/formatting heuristics (deterministic, not a full grammar checker).
	if doubleSpaceRe.MatchString(body) {
		add(SeverityWarning, "double-space", "content contains double spaces", "body")
	}
	if hasRepeatedWord(body) {
		add(SeverityWarning, "repeated-word", "content contains a repeated word (e.g. \"the the\")", "body")
	}

	// Broken/relative link and image reference checks.
	checkLinks(body, add)

	// Type-specific structural validation.
	switch c.Type {
	case TypeBlog:
		validateBlog(c, add)
	case TypeArchitectureDiagram:
		validateMermaid(body, add)
	case TypeSEO:
		validateSEO(c, add)
	case TypeThumbnailPrompt, TypeVisualAsset:
		validateVisual(c, add)
	}

	errors, warnings := 0, 0
	for _, i := range issues {
		if i.Severity == SeverityError {
			errors++
		} else {
			warnings++
		}
	}
	return ValidationReport{Issues: issues, Errors: errors, Warnings: warnings, Passed: errors == 0}
}

func checkLinks(body string, add func(Severity, string, string, string)) {
	for _, m := range linkRe.FindAllStringSubmatch(body, -1) {
		url := strings.TrimSpace(m[1])
		if url == "" {
			add(SeverityError, "empty-link", "a markdown link has an empty target", "body")
			continue
		}
		if strings.HasPrefix(url, "http://") {
			add(SeverityWarning, "insecure-link", "a link uses http:// (prefer https://): "+url, "body")
		}
		if strings.Contains(url, "example.com") || strings.Contains(url, "TODO") {
			add(SeverityError, "placeholder-link", "a link points at a placeholder: "+url, "body")
		}
	}
	for _, m := range imageRe.FindAllStringSubmatch(body, -1) {
		if strings.TrimSpace(m[1]) == "" {
			add(SeverityError, "empty-image", "an image reference has an empty source", "body")
		}
	}
}

func validateBlog(c Content, add func(Severity, string, string, string)) {
	body := c.Body
	// YAML front matter (optional but, if opened, must close).
	if strings.HasPrefix(strings.TrimLeft(body, "\n"), "---") {
		trimmed := strings.TrimLeft(body, "\n")
		if !strings.Contains(trimmed[3:], "\n---") {
			add(SeverityError, "frontmatter", "YAML front matter is opened with --- but never closed", "body")
		}
	}
	// Must have at least one heading.
	if !hasHeading(body) {
		add(SeverityWarning, "no-heading", "blog has no Markdown heading", "body")
	}
	// Unbalanced code fences.
	if strings.Count(body, "```")%2 != 0 {
		add(SeverityError, "code-fence", "unbalanced Markdown code fences (```)", "body")
	}
	// Mermaid blocks inside the blog must also validate.
	validateMermaidBlocks(body, add)
}

func hasHeading(body string) bool {
	for _, ln := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "#") {
			return true
		}
	}
	return false
}

// validateMermaid validates a standalone Mermaid diagram body.
func validateMermaid(body string, add func(Severity, string, string, string)) {
	s := strings.TrimSpace(body)
	if s == "" {
		add(SeverityError, "empty-diagram", "diagram is empty", "body")
		return
	}
	first := firstLine(s)
	known := []string{"flowchart", "graph", "sequenceDiagram", "stateDiagram", "classDiagram", "erDiagram", "journey", "gitGraph"}
	if !startsWithAny(first, known) {
		add(SeverityError, "mermaid-directive", "Mermaid diagram must start with a known directive (flowchart, sequenceDiagram, ...)", "body")
	}
	if strings.Contains(s, "subgraph ") && strings.Count(s, "subgraph ") != countStandalone(s, "end") {
		add(SeverityError, "mermaid-subgraph", "unbalanced Mermaid subgraph/end", "body")
	}
}

// validateMermaidBlocks validates fenced mermaid blocks embedded in markdown.
func validateMermaidBlocks(markdown string, add func(Severity, string, string, string)) {
	lines := strings.Split(markdown, "\n")
	in := false
	var buf []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if !in && strings.HasPrefix(t, "```mermaid") {
			in = true
			buf = nil
			continue
		}
		if in && t == "```" {
			in = false
			validateMermaid(strings.Join(buf, "\n"), add)
			continue
		}
		if in {
			buf = append(buf, ln)
		}
	}
}

func validateSEO(c Content, add func(Severity, string, string, string)) {
	meta := c.Metadata
	if meta == nil {
		add(SeverityError, "no-metadata", "SEO content has no metadata", "metadata")
		return
	}
	if d := meta["metaDescription"]; d == "" {
		add(SeverityWarning, "no-meta-desc", "SEO metadata has no meta description", "metadata.metaDescription")
	} else if len(d) > 160 {
		add(SeverityError, "meta-desc-limit", "meta description exceeds 160 characters", "metadata.metaDescription")
	}
	if meta["slug"] == "" {
		add(SeverityWarning, "no-slug", "SEO metadata has no slug", "metadata.slug")
	}
}

func validateVisual(c Content, add func(Severity, string, string, string)) {
	if c.Metadata["aspectRatio"] == "" {
		add(SeverityWarning, "no-aspect", "visual asset has no aspect ratio", "metadata.aspectRatio")
	}
	// Prompt-based assets must not embed literal rendered text expectations.
	if c.Type == TypeThumbnailPrompt && !strings.Contains(strings.ToLower(c.Body), "no text") && strings.TrimSpace(c.Body) != "" {
		add(SeverityWarning, "no-text-guard", "thumbnail prompt should instruct the model to render no literal text", "body")
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func startsWithAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func countStandalone(s, word string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) == word {
			n++
		}
	}
	return n
}
