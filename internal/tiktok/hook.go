package tiktok

import (
	"context"
	"fmt"
	"strings"
)

// hook builds the TikTok-native opener (first 2–3 seconds). It drafts a
// topic-appropriate hook and (when a Model is configured) asks the model to
// sharpen it — never to add facts or exaggerate. Falls back to the draft.
func (g *Generator) hook(ctx context.Context, t topic) string {
	draft := hookDraft(t)
	if g.Model == nil || draft == "" {
		return draft
	}
	if out, err := g.Model.Generate(ctx, hookPrompt(t.Topic, draft)); err == nil {
		if r := stripReasoning(collapse(strings.TrimSpace(out))); r != "" {
			return r
		}
	}
	return draft
}

// hookDraft returns a TikTok-native opening line by topic. Attention-grabbing
// but never misleading.
func hookDraft(t topic) string {
	switch t.Topic {
	case "AWS Tip":
		return "Most AWS developers don't know this."
	case "CloudFormation Trick":
		return "You've probably been writing CloudFormation the hard way."
	case "Architecture Insight":
		return "This one architecture decision changed everything."
	case "GitHub Automation":
		return "This GitHub workflow saved me hours."
	case "Developer Productivity":
		return "I stopped doing this by hand — here's why."
	case "Common Mistake":
		return "You're probably doing this wrong."
	case "Code Optimization", "Performance Improvement":
		return "One small change, a big difference."
	case "Best Practice":
		return "Steal this before your next deploy."
	case "Deployment Strategy":
		return "There's a smarter way to ship this."
	case "AI Workflow":
		return "What if your documentation wrote itself?"
	case "Interesting Statistic":
		if s := firstSentences(t.Seed, 1); s != "" {
			return s
		}
		return "The numbers here surprised me."
	default:
		if s := firstSentences(t.Seed, 1); s != "" {
			return s
		}
		return "Here's something worth knowing."
	}
}

func hookPrompt(topicLabel, draft string) string {
	return fmt.Sprintf(
		"You are writing the opening hook (first 2–3 seconds, spoken) of a fast-paced educational TikTok for "+
			"software engineers. Topic: %s.\n\n"+
			"Rewrite the DRAFT into ONE scroll-stopping spoken line. Native to TikTok, confident, but NOT misleading "+
			"or exaggerated. Use ONLY the facts in the draft — invent nothing. Output only the hook line.\n\nDRAFT:\n%s",
		topicLabel, draft)
}
