package storyboard

import "strings"

// section is a raw ## block of the blog: its heading and the body beneath it.
type section struct {
	Title string
	Body  string
}

// extractSections splits blog Markdown into its second-level (##) sections,
// dropping YAML front matter and the H1 title. Heading boundaries are the
// blog's own logical structure, so scenes map to technical concepts rather than
// arbitrary paragraph breaks.
func extractSections(markdown string) []section {
	body := stripFrontMatter(markdown)
	lines := strings.Split(body, "\n")

	var sections []section
	var cur *section
	var buf []string
	flush := func() {
		if cur != nil {
			cur.Body = strings.TrimSpace(strings.Join(buf, "\n"))
			sections = append(sections, *cur)
		}
		buf = nil
	}
	for _, ln := range lines {
		if strings.HasPrefix(ln, "## ") {
			flush()
			t := strings.TrimSpace(strings.TrimPrefix(ln, "## "))
			cur = &section{Title: cleanInline(t)}
			continue
		}
		if strings.HasPrefix(ln, "# ") { // H1 title — skip
			continue
		}
		if cur != nil {
			buf = append(buf, ln)
		}
	}
	flush()
	return sections
}

// syntheticSections builds a single grounded "overview" section from the blog
// body when it has no ## headings. This lets small releases (a short blog, a
// bug-fix note) still produce a one-scene storyboard instead of failing — using
// the blog's own prose, so it invents nothing. Returns nil when there is no body
// to work with (a genuinely empty blog).
func syntheticSections(markdown, title string) []section {
	body := stripFrontMatter(markdown)
	var kept []string
	for _, ln := range strings.Split(body, "\n") {
		if strings.HasPrefix(ln, "# ") { // drop the H1 title
			continue
		}
		kept = append(kept, ln)
	}
	text := strings.TrimSpace(strings.Join(kept, "\n"))
	if text == "" {
		return nil
	}
	t := cleanInline(strings.TrimSpace(title))
	if t == "" {
		t = "Overview"
	}
	return []section{{Title: t, Body: text}}
}

// stripFrontMatter removes a leading YAML front-matter block (--- … ---).
func stripFrontMatter(md string) string {
	s := strings.TrimLeft(md, "\n")
	if !strings.HasPrefix(s, "---\n") {
		return md
	}
	if i := strings.Index(s[4:], "\n---"); i >= 0 {
		rest := s[4+i+4:]
		return strings.TrimLeft(rest, "\n")
	}
	return md
}

// sceneType classifies a section heading into a canonical scene type.
func sceneType(title string) string {
	t := strings.ToLower(title)
	switch {
	case containsAny(t, "introduction", "intro", "overview") && !strings.Contains(t, "architecture"):
		return "introduction"
	case containsAny(t, "background", "context", "problem", "motivation", "challenge", "why this matters", "why it matters"):
		return "problem"
	case containsAny(t, "constraint", "limitation", "bottleneck"):
		return "constraint"
	case containsAny(t, "architecture diagram", "diagrams"):
		return "diagram"
	case containsAny(t, "architecture", "design", "system"):
		return "architecture"
	case containsAny(t, "cloudformation", "infrastructure", "iac", "aws resources"):
		return "cloudformation"
	case containsAny(t, "repository", "code changes", "commits", "changed files", "walkthrough"):
		return "repository"
	case containsAny(t, "implementation", "how it works", "under the hood", "deep dive"):
		return "implementation"
	case containsAny(t, "solution", "the fix", "the approach", "pre-baked", "pre baked"):
		return "solution"
	case containsAny(t, "decision", "rationale", "why these", "why we chose", "why chosen"):
		return "decisions"
	case containsAny(t, "results", "benefits", "outcomes", "impact", "performance"):
		return "results"
	case containsAny(t, "tradeoff", "trade-off", "trade off"):
		return "tradeoffs"
	case containsAny(t, "enables next", "what this enables", "unlocks", "future work", "roadmap"):
		return "future"
	case containsAny(t, "lessons", "best practice", "takeaway", "how to use", "extend"):
		return "lessons"
	case containsAny(t, "conclusion", "summary", "wrap", "next steps", "what's next"):
		return "conclusion"
	default:
		return "generic"
	}
}

// objectiveFor returns the didactic objective for a scene type.
func objectiveFor(typ, title string) string {
	switch typ {
	case "introduction":
		return "Hook the viewer and frame what the release is about."
	case "problem":
		return "Establish the problem and why this change matters."
	case "constraint":
		return "Pin down the hard engineering constraint the design must satisfy."
	case "solution":
		return "State the core mechanism that resolves the constraint."
	case "architecture":
		return "Explain the system architecture and how components interact."
	case "diagram":
		return "Walk through the architecture diagram visually."
	case "cloudformation":
		return "Show the infrastructure defined as CloudFormation."
	case "repository":
		return "Tour the repository and the code that changed."
	case "implementation":
		return "Explain how the feature was implemented."
	case "decisions":
		return "Explain each design decision and the tradeoff behind it."
	case "results":
		return "Show the outcomes, benefits, and impact."
	case "tradeoffs":
		return "Weigh the costs the design accepts in exchange for its benefits."
	case "future":
		return "Show what this design now makes possible."
	case "lessons":
		return "Share practical guidance and how to use or extend it."
	case "conclusion":
		return "Summarise the takeaways and point to what's next."
	default:
		return "Explain: " + title + "."
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// cleanInline strips common inline markdown from a heading/label.
func cleanInline(s string) string {
	r := strings.NewReplacer("**", "", "`", "", "_", "", "*", "")
	return strings.TrimSpace(r.Replace(s))
}
