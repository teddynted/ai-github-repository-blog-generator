// Package contentcheck validates generated content artifacts and produces a
// human-readable report. It is used by the local content CLI to catch prompt
// regressions early: missing sections, too many diagrams, placeholder text,
// hallucination markers, marketing language, and per-artifact rules (SEO length,
// LinkedIn limits, valid Mermaid/SVG). It reuses the intent of the blog
// prompt-quality guards, applied to the produced output rather than the prompt.
package contentcheck

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Severity of a validation finding.
type Severity string

const (
	SeverityError Severity = "error" // fails the artifact
	SeverityWarn  Severity = "warn"  // reported, does not fail
)

// Issue is one validation finding.
type Issue struct {
	Severity Severity
	Message  string
}

// Report is the validation outcome for one artifact.
type Report struct {
	Artifact string
	Issues   []Issue
}

func (r *Report) err(format string, a ...any) {
	r.Issues = append(r.Issues, Issue{SeverityError, fmt.Sprintf(format, a...)})
}
func (r *Report) warn(format string, a ...any) {
	r.Issues = append(r.Issues, Issue{SeverityWarn, fmt.Sprintf(format, a...)})
}

// OK reports whether the artifact passed (no error-severity issues).
func (r Report) OK() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return false
		}
	}
	return true
}

// Errors/Warnings counts by severity.
func (r Report) Errors() int   { return r.count(SeverityError) }
func (r Report) Warnings() int { return r.count(SeverityWarn) }
func (r Report) count(s Severity) int {
	n := 0
	for _, i := range r.Issues {
		if i.Severity == s {
			n++
		}
	}
	return n
}

// String renders a human-readable report.
func (r Report) String() string {
	var b strings.Builder
	status := "✓ PASS"
	if !r.OK() {
		status = "✗ FAIL"
	}
	fmt.Fprintf(&b, "%s  %s (%d error, %d warn)\n", status, r.Artifact, r.Errors(), r.Warnings())
	for _, i := range r.Issues {
		mark := "  •"
		if i.Severity == SeverityError {
			mark = "  ✗"
		} else {
			mark = "  ⚠"
		}
		fmt.Fprintf(&b, "%s %s\n", mark, i.Message)
	}
	return b.String()
}

// forbidden marketing phrases the calm engineering register bans.
var marketingPhrases = []string{
	"exciting milestone", "groundbreaking", "revolutionary", "game-chang",
	"cutting-edge", "next-generation", "industry-leading", "best-in-class",
	"state-of-the-art", "powerful new", "showcases innovation", "revolutionise",
	"revolutionize",
}

// placeholder markers that should never survive into a final artifact.
var placeholderMarkers = []string{
	"todo", "tbd", "fixme", "lorem ipsum", "[insert", "<placeholder>",
	"xxxx", "coming soon", "your text here",
}

// hallucination hedges the prompts forbid (reported as warnings — a human may
// have grounded them).
var hallucinationMarkers = []string{
	"probably", "presumably", "it seems", "the team wanted", "we decided to",
}

var mermaidFence = regexp.MustCompile("(?s)```mermaid.*?```")

// inventedCountRe catches stated counts of repository structure (e.g. "65 files",
// "15 diagrams", "four top-level directories") — stale statistics the article
// must replace with relationships. It targets high-signal structural nouns and
// deliberately excludes "components"/"services" (too easily anaphoric, e.g. "the
// two components scale separately").
var inventedCountRe = regexp.MustCompile(`(?i)\b(\d+|two|three|four|five|six|seven|eight|nine|ten)\s+([a-z][a-z-]*\s+)?(files?|directories|folders|packages|modules|diagrams)\b`)

// genericLedeRe catches abstract technology-teaching sentences the article must
// not contain — it should document THIS repository, not explain AWS/Go/event-
// driven architecture in general.
var genericLedeRe = regexp.MustCompile(`(?i)(event-driven architectures?\s+(decouple|enable|have become|are\b|provide)|\baws provides\s+(services|a |managed|the )|\bgo offers\b|serverless\s+(architectures?\s+)?(is|are)\s+popular|serverless\s+(architectures?\s+)?(have|has)\s+become)`)

// Validate runs the generic checks plus any kind-specific rules and returns a
// report. kind is a content kind ("blog", "seo-metadata", "linkedin", …).
func Validate(kind, content string) Report {
	r := Report{Artifact: kind}
	generic(&r, content)
	switch kind {
	case "blog":
		blog(&r, content)
	case "architecture", "architecture-diagram-spec":
		architecture(&r, content)
	case "architecture-diagram":
		svg(&r, content)
	case "seo-metadata":
		seo(&r, content)
	case "linkedin":
		linkedin(&r, content)
	case "x-thread":
		xthread(&r, content)
	}
	return r
}

func generic(r *Report, c string) {
	if strings.TrimSpace(c) == "" {
		r.err("artifact is empty")
		return
	}
	if !utf8.ValidString(c) {
		r.err("content is not valid UTF-8")
	}
	lc := strings.ToLower(c)
	for _, p := range placeholderMarkers {
		if strings.Contains(lc, p) {
			r.err("placeholder text present: %q", p)
		}
	}
	for _, p := range marketingPhrases {
		if strings.Contains(lc, p) {
			r.err("forbidden marketing phrase: %q", p)
		}
	}
}

func blog(r *Report, c string) {
	// Deterministic front matter.
	for _, fm := range []string{"title:", "description:", "tags:"} {
		if !strings.Contains(c, fm) {
			r.err("front matter missing %q", fm)
		}
	}
	// Required section spine (the deterministic anchors).
	for _, sec := range []string{"## Introduction", "## Conclusion"} {
		if !strings.Contains(c, sec) {
			r.err("required section missing: %q", sec)
		}
	}
	// Diagram ceiling.
	if n := len(mermaidFence.FindAllString(c, -1)); n > 2 {
		r.err("too many Mermaid diagrams: %d (max 2)", n)
	}
	// Invented structural counts — stale statistics; describe relationships instead.
	for _, m := range dedupeMatches(inventedCountRe.FindAllString(c, -1)) {
		r.err("invented repository count %q — describe the relationship, not the number", strings.TrimSpace(m))
	}
	// Generic technology-teaching ledes — the article must document THIS repository.
	for _, m := range dedupeMatches(genericLedeRe.FindAllString(c, -1)) {
		r.err("generic technology lede %q — write about this repository, not AWS/Go in general", strings.TrimSpace(m))
	}

	// Release-centric phrasing (the article must be timeless).
	lc := strings.ToLower(c)
	for _, p := range []string{"in this release", "this release delivers", "this update introduces"} {
		if strings.Contains(lc, p) {
			r.warn("release-centric phrasing: %q (article should be timeless)", p)
		}
	}
	hedges(r, lc)
}

// dedupeMatches lowercases, trims, and de-duplicates regex matches so a repeated
// violation is reported once.
func dedupeMatches(matches []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		k := strings.ToLower(strings.TrimSpace(m))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, m)
	}
	return out
}

func architecture(r *Report, c string) {
	if strings.Count(c, "```mermaid")*2 != strings.Count(c, "```") && strings.Contains(c, "```mermaid") {
		r.warn("unbalanced code fences around Mermaid blocks")
	}
	hedges(r, strings.ToLower(c))
}

func svg(r *Report, c string) {
	if !strings.Contains(c, "<svg") || !strings.Contains(c, "</svg>") {
		r.err("does not look like an SVG document")
	}
}

func seo(r *Report, c string) {
	for _, want := range []string{"title", "description", "keyword"} {
		if !strings.Contains(strings.ToLower(c), want) {
			r.warn("SEO metadata does not mention %q", want)
		}
	}
}

func linkedin(r *Report, c string) {
	// LinkedIn posts hard-cap at 3000 characters; warn if any block is close.
	if len([]rune(c)) > 8000 {
		r.warn("LinkedIn content is very long (%d chars) — check per-post limits", len([]rune(c)))
	}
}

func xthread(r *Report, c string) {
	// X posts cap at 280 characters; a thread that never breaks is suspicious.
	if !strings.Contains(c, "\n") {
		r.warn("X thread has no line breaks — posts may exceed 280 characters")
	}
}

func hedges(r *Report, lc string) {
	for _, h := range hallucinationMarkers {
		if strings.Contains(lc, h) {
			r.warn("possible hallucination hedge: %q", h)
		}
	}
}
