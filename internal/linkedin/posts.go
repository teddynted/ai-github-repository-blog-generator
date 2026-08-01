package linkedin

import (
	"context"
	"fmt"
	"strings"
)

// title returns a professional, non-clickbait post title for a candidate.
// title is the per-post title/label — evergreen and type-based, never naming the
// repository or release version.
func title(pkg ReleasePackage, c postCandidate) string {
	switch c.Type {
	case "Release Announcement":
		return "An infrastructure update worth sharing"
	case "Feature Spotlight":
		if s := firstSentences(c.Seed, 1); s != "" && !namesReleaseIdentity(s, pkg) {
			return "A closer look at " + lowerFirst(s)
		}
		return "A closer look at a recent design choice"
	case "Architecture Deep Dive":
		return "How this architecture fits together"
	case "AWS Best Practice":
		return "An AWS pattern worth sharing"
	case "AI Engineering Highlight":
		return "Grounded AI engineering, in practice"
	case "Engineering Lesson":
		return "A lesson from recent infrastructure work"
	case "Developer Productivity Tip":
		return "A small workflow win worth sharing"
	case "Behind-the-Build":
		return "Behind the build"
	case "Performance Improvement":
		return "Reliability work that quietly pays off"
	default:
		return "A few technical notes"
	}
}

// body assembles the LinkedIn post. It builds a grounded, professional draft and
// (when a Model is configured) asks the model to write it in an authentic,
// non-hype voice — never adding facts. Falls back to the draft.
func (g *Generator) body(ctx context.Context, pkg ReleasePackage, c postCandidate, highlights []string, engagement, cta string) string {
	draft := bodyDraft(pkg, c, highlights, engagement, cta)
	if g.Model == nil || strings.TrimSpace(draft) == "" {
		return draft
	}
	if out, err := g.Model.Generate(ctx, bodyPrompt(c, draft)); err == nil {
		if r := strings.TrimSpace(out); r != "" {
			return r
		}
	}
	return draft
}

// bodyDraft builds a grounded, well-structured LinkedIn post.
func bodyDraft(pkg ReleasePackage, c postCandidate, highlights []string, engagement, cta string) string {
	var b strings.Builder
	repo := repoShort(pkg)
	t := tag(pkg)

	// Opening line by type.
	fmt.Fprintf(&b, "%s\n\n", openingLine(c, repo, t))

	// Grounded context — but only when it doesn't name the repository or release
	// version (posts stay evergreen; the substance is carried by the highlights).
	if seed := firstSentences(c.Seed, 2); seed != "" && !namesReleaseIdentity(seed, pkg) {
		fmt.Fprintf(&b, "%s\n\n", seed)
	}

	// Technical highlights as a scannable list.
	if len(highlights) > 0 {
		b.WriteString("What stood out:\n")
		for _, h := range topStrings(highlights, 4) {
			fmt.Fprintf(&b, "• %s\n", collapse(h))
		}
		b.WriteString("\n")
	}

	// Stack line (grounded).
	if stack := topStrings(append(append([]string{}, awsServices(pkg)...), technologies(pkg)...), 5); len(stack) > 0 {
		fmt.Fprintf(&b, "Stack: %s.\n\n", joinAnd(stack))
	}

	// Engagement prompt + CTA.
	if engagement != "" {
		fmt.Fprintf(&b, "%s\n\n", engagement)
	}
	if cta != "" {
		fmt.Fprintf(&b, "%s", cta)
	}
	return strings.TrimRight(b.String(), "\n")
}

// openingLine is a type-appropriate, PROBLEM-FIRST opener. It deliberately does
// NOT name the repository or release version — a LinkedIn post should stand on
// its own and read as an engineering reflection, not a changelog line. (The repo
// link lives in the CTA.) The repo/tag args are retained for signature stability
// but intentionally unused.
func openingLine(c postCandidate, _, _ string) string {
	switch c.Type {
	case "Release Announcement":
		return "Some infrastructure work worth sharing."
	case "Feature Spotlight":
		return "One design choice from recent work I keep coming back to:"
	case "Architecture Deep Dive":
		return "A note on the architecture behind this one."
	case "AWS Best Practice":
		return "Sharing an AWS pattern that's been working well."
	case "AI Engineering Highlight":
		return "Some notes on the AI-engineering side of this work."
	case "Engineering Lesson":
		return "A lesson worth writing down from recent infrastructure work."
	case "Developer Productivity Tip":
		return "Small thing, real time saved:"
	case "Behind-the-Build":
		return "A bit of the story behind how this was built."
	case "Performance Improvement":
		return "Reliability work that quietly pays off:"
	default:
		return "A few technical notes from recent work."
	}
}

func bodyPrompt(c postCandidate, draft string) string {
	return fmt.Sprintf(
		"You are an experienced software engineer writing a LinkedIn post (type: %s) for %s.\n\n"+
			"Rewrite the DRAFT into an authentic, professional LinkedIn post: educational, technically accurate, and "+
			"approachable. Follow these rules:\n"+
			"- OPEN with a hook — a problem, an insight, or an architecture-curiosity question — NOT \"Just shipped …\" or a changelog line.\n"+
			"- Keep the post EVERGREEN: do NOT mention the repository name, the release version, any version number (e.g. v0.6.0), or \"release\"/\"changelog\" framing. A reader should not need to know which repo or release this came from.\n"+
			"- Do NOT frame it by counts (never \"0 features\", \"0 fixes\", \"maintenance changes\"); frame it by the engineering value.\n"+
			"- Make this post distinct to its type (%s): lead with that angle; do not restate the same architecture sentence a reader would see on every post.\n"+
			"- Short paragraphs (1–3 sentences) for mobile readability; keep the bullet highlights (max 4), the engagement question, and the CTA.\n"+
			"- NO marketing hype, NO clickbait, NO exaggerated or performance/latency/cost claims, NO buzzword stuffing.\n"+
			"- Use ONLY the facts in the draft — invent nothing (no metrics, no services, no features not present).\n\n"+
			"Output only the post text.\n\nDRAFT:\n%s",
		c.Type, c.Audience, c.Type, draft)
}
