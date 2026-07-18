package youtube

import (
	"context"
	"fmt"
	"strings"
)

// hook builds the opening 15–30 seconds. It chooses a hook type from the Release
// Context and drafts a grounded line, then (when a Model is configured) asks the
// model to sharpen it — never to add facts. Falls back to the draft on any error.
func (g *Generator) hook(ctx context.Context, pkg ReleasePackage) Hook {
	typ, draft := hookDraft(pkg)
	script := draft
	if g.Model != nil && draft != "" {
		if out, err := g.Model.Generate(ctx, hookPrompt(typ, draft)); err == nil {
			if r := collapse(strings.TrimSpace(out)); r != "" {
				script = r
			}
		}
	}
	dur := clampInt(speakingSeconds(wordCount(script), g.wpm()), hookMinSec, hookMaxSec)
	return Hook{Type: typ, Script: script, DurationSec: dur}
}

// hookDraft picks a hook type and a grounded opening line. It never invents:
// every line is built from the Release Context / summary.
func hookDraft(pkg ReleasePackage) (string, string) {
	c := pkg.Context
	repo := repoName(pkg)
	tag := releaseTag(pkg)
	summary := ""
	if c != nil {
		summary = firstSentences(c.ContentIntelligence.Summary, 1)
	}

	switch {
	case c != nil && c.ContentIntelligence.ImplementationComplexity == "high":
		return "problem", fmt.Sprintf(
			"Shipping %s in %s meant solving a genuinely hard problem — and the way it's built is worth understanding. %s",
			featureName(pkg), repo, summary)
	case c != nil && len(c.ContentIntelligence.ArchitectureHighlights) > 0:
		return "insight", fmt.Sprintf(
			"Here's something most engineers get wrong about this kind of system — and how %s gets it right. %s",
			repo, firstSentences(c.ContentIntelligence.ArchitectureHighlights[0], 1))
	case c != nil && len(c.Architecture.AWSServices) >= 2:
		return "showcase", fmt.Sprintf(
			"In this release we wire up %s into one clean, event-driven pipeline. %s",
			joinAnd(topStrings(c.Architecture.AWSServices, 3)), summary)
	default:
		return "outcome", fmt.Sprintf(
			"By the end of this video you'll understand exactly what shipped in %s %s, and how it was built. %s",
			repo, tag, summary)
	}
}

func hookPrompt(typ, draft string) string {
	return fmt.Sprintf(
		"You are writing the opening hook (15–30 seconds, spoken) for a long-form technical YouTube video.\n"+
			"Hook type: %s.\n\n"+
			"Rewrite the DRAFT into 2–3 punchy, spoken sentences that make an engineer want to keep watching. "+
			"Be confident but NOT clickbait. Use ONLY the facts in the draft — invent nothing. Output only the hook.\n\n"+
			"DRAFT:\n%s", typ, draft)
}

// --- small grounded helpers shared by the framing planners ---

func repoName(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.FullName != "" {
		return pkg.Context.Repository.FullName
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Repository, pkg.VoiceOver.Metadata.Repository, "this project")
}

func releaseTag(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.Tag != "" {
		return pkg.Context.Release.Tag
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Release, pkg.VoiceOver.Metadata.Release)
}

// featureName derives a short, grounded name for what shipped.
func featureName(pkg ReleasePackage) string {
	if pkg.Context != nil {
		if f := pkg.Context.Changelog.Features; len(f) > 0 {
			return f[0]
		}
		if w := pkg.Context.Implementation.WhatChanged; len(w) > 0 {
			return firstSentences(w[0], 1)
		}
	}
	return "this release"
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}
