package shorts

import (
	"fmt"
	"strings"
)

// Markdown renders the collection as a production-ready document: one section
// per Short with hook, script, scene breakdown, captions, visuals, animations,
// CTA, and hashtags.
func (col ShortsCollection) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# YouTube Shorts: %s\n\n", firstNonEmpty(col.Metadata.SourceBlogTitle, col.Metadata.Repository))
	fmt.Fprintf(&b, "_%s · release %s · %d shorts_\n\n",
		col.Metadata.Repository, col.Metadata.Release, col.Metadata.ShortCount)

	if len(col.Warnings) > 0 {
		fmt.Fprintf(&b, "> **Notes:** %s\n\n", strings.Join(col.Warnings, "; "))
	}

	for _, s := range col.Shorts {
		writeShort(&b, s)
	}

	writeIntelligence(&b, col.ContentIntelligence)
	return b.String()
}

func writeShort(b *strings.Builder, s Short) {
	fmt.Fprintf(b, "---\n\n## Short %d — %s\n\n", s.ID, s.Title)
	fmt.Fprintf(b, "- **Angle:** %s · **Duration:** %s (%ds) · **Words:** %d\n", s.Angle, s.Duration, s.DurationSec, s.WordCount)
	fmt.Fprintf(b, "- **Hook:** %s\n", s.Hook)

	fmt.Fprintf(b, "\n### Script\n\n> %s\n\n", s.Script)

	b.WriteString("### Scene Breakdown\n\n")
	for _, sc := range s.Scenes {
		fmt.Fprintf(b, "**Scene %d** (%ds) — _%s · %s · %s_\n", sc.Number, sc.DurationSec, sc.Camera, sc.Animation, sc.Transition)
		fmt.Fprintf(b, "  - Visual: %s\n", sc.Visual)
		if sc.Overlay != "" {
			fmt.Fprintf(b, "  - Overlay: %s\n", sc.Overlay)
		}
		fmt.Fprintf(b, "  - Narration: %s\n\n", sc.Narration)
	}

	if len(s.Captions) > 0 {
		b.WriteString("### Captions (burned-in)\n\n")
		for _, c := range s.Captions {
			fmt.Fprintf(b, "- `%s–%s` %s\n", mmss(c.StartSec), mmss(c.EndSec), c.Text)
		}
		b.WriteString("\n")
	}

	if len(s.Visuals) > 0 {
		b.WriteString("### Visuals\n\n")
		for _, v := range s.Visuals {
			fmt.Fprintf(b, "- [%s] %s", v.Kind, v.Description)
			if v.Reference != "" {
				fmt.Fprintf(b, " _(ref: %s)_", v.Reference)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(s.Animations) > 0 {
		parts := make([]string, len(s.Animations))
		for i, a := range s.Animations {
			parts[i] = fmt.Sprintf("S%d:%s", a.Scene, a.Type)
		}
		fmt.Fprintf(b, "- **Animations:** %s\n", strings.Join(parts, " · "))
	}
	fmt.Fprintf(b, "- **CTA:** %s\n", s.CTA)
	if len(s.Hashtags) > 0 {
		fmt.Fprintf(b, "- **Hashtags:** %s\n", strings.Join(s.Hashtags, " "))
	}
	fmt.Fprintf(b, "- **Thumbnail:** %s\n\n", s.SEO.ThumbnailText)
}

func writeIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Collection Intelligence\n\n")
	fmt.Fprintf(b, "- **Shorts:** %d · **Total duration:** %s · **Average:** %ds\n", ci.ShortCount, mmss(ci.TotalDurationSec), ci.AverageDurationSec)
	fmt.Fprintf(b, "- **Audience:** %s · **Difficulty:** %s\n", ci.Audience, ci.Difficulty)
	if len(ci.Topics) > 0 {
		fmt.Fprintf(b, "- **Topics:** %s\n", strings.Join(ci.Topics, ", "))
	}
	if len(ci.SEOKeywords) > 0 {
		fmt.Fprintf(b, "- **SEO keywords:** %s\n", strings.Join(ci.SEOKeywords, ", "))
	}
	fmt.Fprintf(b, "- **Publish cadence:** %s\n", ci.PublishCadence)
	if len(ci.ProductionNotes) > 0 {
		b.WriteString("- **Production notes:**\n")
		for _, n := range ci.ProductionNotes {
			fmt.Fprintf(b, "  - %s\n", n)
		}
	}
	b.WriteString("\n")
}
