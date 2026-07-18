package storyboard

import (
	"fmt"
	"strings"
)

// Markdown renders the storyboard as a human-readable production document.
func (s Storyboard) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Storyboard: %s\n\n", s.Metadata.SourceBlogTitle)
	fmt.Fprintf(&b, "_%s · release %s · %d scenes · ~%s (%s, %s)_\n\n",
		s.Metadata.Repository, s.Metadata.Release, s.Video.SceneCount,
		mmss(s.Video.TotalDurationSec), s.Video.TargetFormat, s.Video.AspectRatio)

	if s.ContentIntelligence.SuggestedTitle != "" {
		fmt.Fprintf(&b, "**Suggested title:** %s  \n", s.ContentIntelligence.SuggestedTitle)
	}
	fmt.Fprintf(&b, "**Audience:** %s · **Difficulty:** %s · **Production:** %s · **Animation:** %s\n\n",
		s.ContentIntelligence.Audience, s.ContentIntelligence.Difficulty,
		s.ContentIntelligence.ProductionComplexity, s.ContentIntelligence.AnimationComplexity)

	if len(s.Warnings) > 0 {
		b.WriteString("> **Notes:** " + strings.Join(s.Warnings, "; ") + "\n\n")
	}

	for _, sc := range s.Scenes {
		writeScene(&b, sc)
	}

	if len(s.ContentIntelligence.Chapters) > 0 {
		b.WriteString("---\n\n## Chapters\n\n")
		for _, c := range s.ContentIntelligence.Chapters {
			fmt.Fprintf(&b, "- `%s` %s\n", mmss(c.StartSec), c.Title)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func writeScene(b *strings.Builder, sc Scene) {
	fmt.Fprintf(b, "---\n\n## Scene %d — %s\n\n", sc.SceneNumber, sc.Title)
	fmt.Fprintf(b, "- **Objective:** %s\n", sc.Objective)
	fmt.Fprintf(b, "- **Timing:** %ds recommended (%d–%ds, %s)\n",
		sc.Duration.RecommendedSec, sc.Duration.MinSec, sc.Duration.MaxSec, sc.Duration.Pacing)
	fmt.Fprintf(b, "- **Narration:** %s\n", sc.Narration)
	fmt.Fprintf(b, "- **Visual:** %s\n", sc.Visuals.Description)
	fmt.Fprintf(b, "- **Camera:** %s — %s\n", sc.Camera.Direction, sc.Camera.Notes)

	if len(sc.Animations) > 0 {
		b.WriteString("- **Animation:**\n")
		for _, a := range sc.Animations {
			fmt.Fprintf(b, "  %d. %s", a.Sequence, a.Type)
			if a.Target != "" {
				fmt.Fprintf(b, " → %s", a.Target)
			}
			if a.Notes != "" {
				fmt.Fprintf(b, " — %s", a.Notes)
			}
			b.WriteString("\n")
		}
	}
	if len(sc.Overlays) > 0 {
		b.WriteString("- **Overlays:**\n")
		for _, o := range sc.Overlays {
			fmt.Fprintf(b, "  - [%s] %s\n", o.Kind, o.Text)
		}
	}
	if len(sc.Diagrams) > 0 {
		b.WriteString("- **Diagrams:**\n")
		for _, d := range sc.Diagrams {
			fmt.Fprintf(b, "  - %s (%s) — %s; highlight: %s\n",
				d.Source, d.Type, d.Animation, strings.Join(d.HighlightNodes, ", "))
		}
	}
	if len(sc.Code) > 0 {
		b.WriteString("- **Code:**\n")
		for _, c := range sc.Code {
			fmt.Fprintf(b, "  - [%s] %s\n", c.Language, c.Instruction)
		}
	}
	if len(sc.Assets) > 0 {
		fmt.Fprintf(b, "- **Assets:** %s\n", strings.Join(sc.Assets, ", "))
	}
	fmt.Fprintf(b, "- **Transition:** %s (%.1fs)\n", sc.Transition.Type, sc.Transition.DurationSec)
	if sc.MusicMood != "" {
		fmt.Fprintf(b, "- **Music:** %s\n", sc.MusicMood)
	}
	b.WriteString("\n")
}
