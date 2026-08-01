package tiktok

import (
	"context"
	"fmt"
	"strings"
)

// script builds the problem/solution/takeaway beats from the grounded seed and
// assembles the full spoken script (hook → problem → solution → takeaway →
// engagement → CTA). When a Model is configured it polishes the full script into
// conversational, TikTok-paced narration — never adding facts. Falls back to the
// deterministic assembly.
func (g *Generator) script(ctx context.Context, t topic, hook, engagement, cta string) (problem, solution, takeaway, full string) {
	problem = problemFor(t)
	solution = solutionFor(t)
	takeaway = takeawayFor(t)
	full = assemble(hook, problem, solution, takeaway, engagement, cta)

	if g.Model != nil && collapse(solution) != "" {
		if out, err := g.Model.Generate(ctx, scriptPrompt(t.Topic, hook, problem, solution, takeaway, engagement, cta)); err == nil {
			if r := stripReasoning(collapse(strings.TrimSpace(out))); r != "" {
				full = r
			}
		}
	}
	return problem, solution, takeaway, full
}

// problemFor frames the pain the concept addresses, by topic.
func problemFor(t topic) string {
	switch t.Topic {
	case "Common Mistake":
		return "Here's the trap a lot of teams fall into."
	case "CloudFormation Trick", "AWS Tip":
		return "Doing this by hand gets tedious fast."
	case "Architecture Insight":
		return "Most setups tangle these concerns together."
	case "GitHub Automation", "Developer Productivity":
		return "Doing this manually every release is a time sink."
	case "Code Optimization", "Performance Improvement":
		return "The naive approach quietly costs you."
	default:
		return "There's an easier way than the obvious one."
	}
}

// solutionFor is the grounded core: what the release actually does.
func solutionFor(t topic) string {
	seed := firstSentences(t.Seed, 2)
	if seed == "" {
		return ""
	}
	return "Here's the fix: " + lowerFirst(seed)
}

func takeawayFor(t topic) string {
	switch t.Topic {
	case "Common Mistake":
		return "Skip the trap and it just works."
	case "Architecture Insight":
		return "Clean boundaries pay off every single time."
	case "Code Optimization", "Performance Improvement":
		return "Measure it and the win is obvious."
	case "AWS Tip", "CloudFormation Trick", "Best Practice":
		return "Small habit, big payoff."
	default:
		return "Simple, but it makes a real difference."
	}
}

func assemble(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if collapse(p) != "" {
			kept = append(kept, collapse(p))
		}
	}
	return strings.Join(kept, " ")
}

func scriptPrompt(topicLabel, hook, problem, solution, takeaway, engagement, cta string) string {
	return fmt.Sprintf(
		"You are scripting a 20–60 second educational TikTok for software engineers (topic: %s).\n\n"+
			"Weave these beats into ONE conversational, fast-paced spoken script (about 70–120 words):\n"+
			"HOOK: %s\nPROBLEM: %s\nSOLUTION: %s\nTAKEAWAY: %s\nENGAGEMENT: %s\nCTA: %s\n\n"+
			"Native to TikTok, technically accurate, one concept. Use ONLY the facts above — invent nothing. "+
			"Output only the script.",
		topicLabel, hook, problem, solution, takeaway, engagement, cta)
}
