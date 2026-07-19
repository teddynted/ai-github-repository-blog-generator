package tiktok

import (
	"fmt"
	"strings"
)

// Markdown renders the collection as a production-ready document: one section
// per TikTok with hook, script, scene breakdown, captions, visual suggestions,
// camera directions, animations, engagement prompt, CTA, and hashtags.
func (col TikTokCollection) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# TikTok Videos: %s\n\n", firstNonEmpty(col.Metadata.SourceBlogTitle, col.Metadata.Repository))
	fmt.Fprintf(&b, "_%s · release %s · %d videos_\n\n",
		col.Metadata.Repository, col.Metadata.Release, col.Metadata.VideoCount)

	if len(col.Warnings) > 0 {
		fmt.Fprintf(&b, "> **Notes:** %s\n\n", strings.Join(col.Warnings, "; "))
	}

	for _, v := range col.Videos {
		writeVideo(&b, v)
	}

	writeIntelligence(&b, col.ContentIntelligence)
	return b.String()
}

func writeVideo(b *strings.Builder, v Video) {
	fmt.Fprintf(b, "---\n\n## Video %d — %s\n\n", v.ID, v.Title)
	fmt.Fprintf(b, "- **Topic:** %s · **Duration:** %s (%ds) · **Retention:** %d/100 · **Words:** %d\n",
		v.Topic, v.Duration, v.DurationSec, v.RetentionScore, v.WordCount)
	fmt.Fprintf(b, "- **Hook:** %s\n", v.Hook)

	fmt.Fprintf(b, "\n### Script\n\n> %s\n\n", v.Script)

	b.WriteString("### Scene Breakdown\n\n")
	for _, sc := range v.Scenes {
		fmt.Fprintf(b, "**Scene %d** (%ds) — _%s · %s · %s_\n", sc.Number, sc.DurationSec, sc.Camera, sc.Animation, sc.Transition)
		fmt.Fprintf(b, "  - Visual: %s\n", sc.Visual)
		if sc.Overlay != "" {
			fmt.Fprintf(b, "  - Overlay: %s\n", sc.Overlay)
		}
		fmt.Fprintf(b, "  - Narration: %s\n\n", sc.Narration)
	}

	if len(v.Captions) > 0 {
		b.WriteString("### Captions (burned-in)\n\n")
		for _, c := range v.Captions {
			style := ""
			if c.Style != "" {
				style = " _(" + c.Style + ")_"
			}
			fmt.Fprintf(b, "- `%s–%s` %s%s\n", mmss(c.StartSec), mmss(c.EndSec), c.Text, style)
		}
		b.WriteString("\n")
	}

	if len(v.Visuals) > 0 {
		b.WriteString("### Visual Suggestions\n\n")
		for _, vis := range v.Visuals {
			fmt.Fprintf(b, "- [%s] %s", vis.Kind, vis.Description)
			if vis.Reference != "" {
				fmt.Fprintf(b, " _(ref: %s)_", vis.Reference)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(v.Camera) > 0 {
		parts := make([]string, len(v.Camera))
		for i, c := range v.Camera {
			parts[i] = fmt.Sprintf("S%d:%s", c.Scene, c.Direction)
		}
		fmt.Fprintf(b, "- **Camera:** %s\n", strings.Join(parts, " · "))
	}
	if len(v.Animations) > 0 {
		parts := make([]string, len(v.Animations))
		for i, a := range v.Animations {
			parts[i] = fmt.Sprintf("S%d:%s", a.Scene, a.Type)
		}
		fmt.Fprintf(b, "- **Animations:** %s\n", strings.Join(parts, " · "))
	}
	fmt.Fprintf(b, "- **Engagement:** %s\n", v.EngagementPrompt)
	fmt.Fprintf(b, "- **CTA:** %s\n", v.CTA)
	if len(v.Hashtags) > 0 {
		fmt.Fprintf(b, "- **Hashtags:** %s\n", strings.Join(v.Hashtags, " "))
	}
	fmt.Fprintf(b, "- **Caption:** %s\n\n", v.SEO.Caption)
}

func writeIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Collection Intelligence\n\n")
	fmt.Fprintf(b, "- **Videos:** %d · **Total:** %s · **Average:** %ds · **Avg retention:** %d/100\n",
		ci.VideoCount, mmss(ci.TotalDurationSec), ci.AverageDurationSec, ci.AverageRetention)
	fmt.Fprintf(b, "- **Audience:** %s · **Difficulty:** %s\n", ci.Audience, ci.Difficulty)
	if len(ci.Topics) > 0 {
		fmt.Fprintf(b, "- **Topics:** %s\n", strings.Join(ci.Topics, ", "))
	}
	if len(ci.SEOKeywords) > 0 {
		fmt.Fprintf(b, "- **SEO keywords:** %s\n", strings.Join(ci.SEOKeywords, ", "))
	}
	fmt.Fprintf(b, "- **Posting cadence:** %s\n", ci.PostingCadence)
	if len(ci.ProductionNotes) > 0 {
		b.WriteString("- **Production notes:**\n")
		for _, n := range ci.ProductionNotes {
			fmt.Fprintf(b, "  - %s\n", n)
		}
	}
	b.WriteString("\n")
}
