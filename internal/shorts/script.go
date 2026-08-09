package shorts

import (
	"context"
	"fmt"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/terminology"
)

// script builds the core explanation and takeaway for a Short from the grounded
// seed, then assembles the full spoken script (hook → core → takeaway → CTA).
// When a Model is configured it polishes the full script into energetic,
// concise narration — never adding facts. Falls back to the deterministic
// assembly.
func (g *Generator) script(ctx context.Context, c candidate, hook, cta string) (core, takeaway, full string) {
	core = coreExplanation(c)
	takeaway = takeawayFor(c)
	full = assembleScript(hook, core, takeaway, cta)

	if g.Model != nil && collapse(core) != "" {
		if out, err := g.Model.Generate(ctx, scriptPrompt(c.Angle, hook, core, takeaway, cta)); err == nil {
			if r := collapse(strings.TrimSpace(out)); r != "" {
				full = r
			}
		}
	}
	return core, takeaway, full
}

// coreExplanation turns the grounded seed into 1–2 crisp explanatory sentences.
func coreExplanation(c candidate) string {
	seed := firstSentences(c.Seed, 2)
	if seed == "" {
		return ""
	}
	switch c.Angle {
	case "Interesting Statistic":
		return seed
	case "Architecture Reveal":
		return "The key idea: " + lowerFirst(seed)
	default:
		return seed
	}
}

// takeawayFor is the one-line lesson to leave the viewer with.
func takeawayFor(c candidate) string {
	switch c.Angle {
	case "Common Mistake":
		return "Avoid that trap and your future self will thank you."
	case "AWS Best Practice", "CloudFormation Tip":
		return "Small habit, big payoff."
	case "Optimization":
		return "Measure it, and the win is obvious."
	case "Architecture Reveal":
		return "Clean boundaries make the whole thing easy to reason about."
	case "Code Walkthrough":
		return "Readable code beats clever code every time."
	case "Lesson Learned":
		return "Worth remembering on your next build."
	default:
		return "Simple, but it makes a real difference."
	}
}

func assembleScript(hook, core, takeaway, cta string) string {
	parts := []string{hook, core, takeaway, cta}
	kept := parts[:0]
	for _, p := range parts {
		if collapse(p) != "" {
			kept = append(kept, collapse(p))
		}
	}
	return strings.Join(kept, " ")
}

func scriptPrompt(angle, hook, core, takeaway, cta string) string {
	return fmt.Sprintf(
		"You are scripting a 30–60 second technical YouTube Short (angle: %s).\n\n"+
			"Weave these beats into ONE energetic, concise spoken script (about 90–130 words): \n"+
			"HOOK: %s\nCORE: %s\nTAKEAWAY: %s\nCTA: %s\n\n"+
			"Fast-paced, one idea, spoken English. Use ONLY the facts above — invent nothing."+evergreenRule+" "+terminology.Base().PromptClause()+" Output only the script.",
		angle, hook, core, takeaway, cta)
}
