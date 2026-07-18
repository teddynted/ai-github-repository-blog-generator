package shorts

import (
	"context"
	"fmt"
	"strings"
)

// hook builds the first-3-5-seconds opener for a Short. It drafts a grounded,
// angle-appropriate hook and (when a Model is configured) asks the model to
// sharpen it — never to add facts. Falls back to the draft on any error.
func (g *Generator) hook(ctx context.Context, c candidate) string {
	draft := hookDraft(c)
	if g.Model == nil || draft == "" {
		return draft
	}
	if out, err := g.Model.Generate(ctx, hookPrompt(c.Angle, draft)); err == nil {
		if r := collapse(strings.TrimSpace(out)); r != "" {
			return r
		}
	}
	return draft
}

// hookDraft returns an angle-shaped opening line, seeded by the grounded fact.
func hookDraft(c candidate) string {
	subject := firstSentences(c.Seed, 1)
	switch c.Angle {
	case "Common Mistake":
		return "Most developers get this wrong."
	case "AWS Best Practice":
		return "Here's an AWS best practice most teams skip."
	case "Architecture Reveal":
		return "This is the whole architecture in one shot."
	case "Optimization":
		return "This one change made all the difference."
	case "CloudFormation Tip":
		return "One CloudFormation habit that saves you later."
	case "Developer Tip":
		return "Quick tip you'll actually use."
	case "Code Walkthrough":
		return "Here's how this actually works under the hood."
	case "Demo Highlight":
		return "Watch this run end to end."
	case "Lesson Learned":
		return "Here's what shipping this taught us."
	case "Interesting Statistic":
		if subject != "" {
			return subject
		}
		return "The numbers here are wild."
	default:
		if subject != "" {
			return subject
		}
		return "Here's something worth knowing."
	}
}

func hookPrompt(angle, draft string) string {
	return fmt.Sprintf(
		"You are writing the opening hook (first 3–5 seconds, spoken) of a fast-paced technical YouTube Short.\n"+
			"Angle: %s.\n\n"+
			"Rewrite the DRAFT into ONE punchy spoken line that stops the scroll. Confident, NOT clickbait. "+
			"Use ONLY the facts in the draft — invent nothing. Output only the hook line.\n\nDRAFT:\n%s",
		angle, draft)
}
