package youtube

import (
	"context"
	"fmt"
	"strings"
)

// conclusion builds the closing summary (20–45s): what was built, key takeaways,
// architecture recap, and a preview of the next milestone. Grounded in the
// Release Context; the Model only smooths the prose.
func (g *Generator) conclusion(ctx context.Context, pkg ReleasePackage, chapters []Chapter, startSec int) Conclusion {
	built := whatWasBuilt(pkg)
	takeaways := keyTakeaways(pkg, chapters)
	next := nextRelease(pkg)
	scenes := scenesOfType(pkg, "conclusion")

	draft := conclusionDraft(pkg, built, next)
	script := draft
	if g.Model != nil && draft != "" {
		if out, err := g.Model.Generate(ctx, conclusionPrompt(draft)); err == nil {
			if r := collapse(strings.TrimSpace(out)); r != "" {
				script = r
			}
		}
	}
	dur := clampInt(speakingSeconds(wordCount(script), g.wpm()), conclusionMinSec, conclusionMaxSec)
	return Conclusion{
		Script:           script,
		DurationSec:      dur,
		Timestamp:        stamp(startSec, startSec+dur),
		WhatWasBuilt:     built,
		KeyTakeaways:     takeaways,
		NextRelease:      next,
		StoryboardScenes: scenes,
	}
}

func conclusionDraft(pkg ReleasePackage, built []string, next string) string {
	var b strings.Builder
	// Evergreen: no repo/version in the spoken wrap-up.
	b.WriteString("So that's the walkthrough, end to end.")
	if len(built) > 0 {
		fmt.Fprintf(&b, " We built %s.", joinAnd(topStrings(built, 3)))
	}
	if c := pkg.Context; c != nil && c.Implementation.WhyItMatters != "" {
		fmt.Fprintf(&b, " %s", firstSentences(c.Implementation.WhyItMatters, 1))
	}
	if next != "" {
		fmt.Fprintf(&b, " Next up: %s.", lowerFirst(next))
	}
	b.WriteString(" Thanks for watching.")
	return collapse(b.String())
}

func conclusionPrompt(draft string) string {
	return fmt.Sprintf(
		"You are writing the conclusion (20–45 seconds, spoken) of a long-form technical YouTube video.\n\n"+
			"Rewrite the DRAFT into a satisfying spoken wrap-up that recaps what was built and points to what's next. "+
			"Warm and confident. Use ONLY the facts in the draft — invent nothing."+evergreenRule+" Output only the conclusion.\n\nDRAFT:\n%s",
		draft)
}

func whatWasBuilt(pkg ReleasePackage) []string {
	if c := pkg.Context; c != nil {
		if len(c.Changelog.Features) > 0 {
			return topStrings(c.Changelog.Features, 4)
		}
		if len(c.Implementation.WhatChanged) > 0 {
			return topStrings(c.Implementation.WhatChanged, 4)
		}
	}
	return nil
}

func keyTakeaways(pkg ReleasePackage, chapters []Chapter) []string {
	var out []string
	for _, ch := range chapters {
		for _, co := range ch.Callouts {
			if co.Kind == "Lesson Learned" || co.Kind == "Best Practice" {
				out = append(out, co.Text)
			}
		}
	}
	if c := pkg.Context; c != nil {
		out = append(out, c.ContentIntelligence.TechnicalHighlights...)
	}
	return topStrings(dedupe(out), 5)
}

func nextRelease(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil && len(c.ContentIntelligence.FutureEnhancements) > 0 {
		return firstSentences(c.ContentIntelligence.FutureEnhancements[0], 1)
	}
	return "we keep building the pipeline release by release"
}
