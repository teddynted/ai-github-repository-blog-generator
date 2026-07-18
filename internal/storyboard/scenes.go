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
	case containsAny(t, "background", "context", "problem", "motivation", "challenge"):
		return "problem"
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
	case containsAny(t, "results", "benefits", "outcomes", "impact", "performance"):
		return "results"
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
	case "results":
		return "Show the outcomes, benefits, and impact."
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
