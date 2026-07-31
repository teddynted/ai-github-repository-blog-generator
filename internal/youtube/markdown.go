package youtube

import (
	"fmt"
	"strings"
)

// Markdown renders the YouTube script as a production-ready document for the
// creator: hook, introduction, per-chapter narration/visuals/demo/callouts/
// timing, conclusion, CTA, and the metadata block.
func (s YouTubeScript) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# YouTube Script: %s\n\n", firstNonEmpty(s.Video.Title, s.Metadata.SourceBlogTitle, s.Metadata.Repository))
	fmt.Fprintf(&b, "_%s · release %s · %s · ~%s · %s · %s_\n\n",
		s.Metadata.Repository, s.Metadata.Release, s.Video.Format,
		s.Video.Duration, s.Video.Audience, s.Video.Difficulty)

	// s.Warnings are internal pipeline QA diagnostics (under-run, thin upstream,
	// fit warnings). They are kept on the struct for JSON/provenance and logged
	// at generation, but MUST NOT surface in the viewer-facing artifact — so they
	// are deliberately not rendered here.

	// Hook
	fmt.Fprintf(&b, "## Hook (`%s`, %s)\n\n", s.Hook.Timestamp.Label, s.Hook.Type)
	fmt.Fprintf(&b, "> %s\n\n", s.Hook.Script)

	// Introduction
	fmt.Fprintf(&b, "## Introduction (`%s`)\n\n", s.Introduction.Timestamp.Label)
	fmt.Fprintf(&b, "> %s\n\n", s.Introduction.Script)
	writeList(&b, "Technologies", s.Introduction.Technologies)
	writeList(&b, "You'll learn", s.Introduction.LearningOutcomes)
	writeList(&b, "Agenda", s.Introduction.Agenda)
	b.WriteString("\n")

	// Chapters
	for _, ch := range s.Chapters {
		writeChapter(&b, ch)
	}

	// Conclusion
	fmt.Fprintf(&b, "---\n\n## Conclusion (`%s`)\n\n", s.Conclusion.Timestamp.Label)
	fmt.Fprintf(&b, "> %s\n\n", s.Conclusion.Script)
	writeList(&b, "What we built", s.Conclusion.WhatWasBuilt)
	writeList(&b, "Key takeaways", s.Conclusion.KeyTakeaways)
	if s.Conclusion.NextRelease != "" {
		fmt.Fprintf(&b, "- **Next:** %s\n", s.Conclusion.NextRelease)
	}
	b.WriteString("\n")

	// CTA
	b.WriteString("---\n\n## Call to Action\n\n")
	fmt.Fprintf(&b, "> %s\n\n", s.CallToAction.Script)
	for _, it := range s.CallToAction.Items {
		fmt.Fprintf(&b, "- **%s** — %s", it.Kind, it.Text)
		if it.URL != "" {
			fmt.Fprintf(&b, " (%s)", it.URL)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	writeMetadata(&b, s)
	return b.String()
}

func writeChapter(b *strings.Builder, ch Chapter) {
	fmt.Fprintf(b, "---\n\n## Chapter %d — %s\n\n", ch.Number, ch.Title)
	fmt.Fprintf(b, "- **Timestamp:** `%s` (target %ds; %d–%ds)\n", ch.Timestamp.Label, ch.Duration.TargetSec, ch.Duration.MinSec, ch.Duration.MaxSec)
	fmt.Fprintf(b, "- **Storyboard scenes:** %s · **Voice-over scenes:** %s\n",
		ints(ch.StoryboardScenes), ints(ch.VoiceOverReferences))

	fmt.Fprintf(b, "\n### Narration\n\n> %s\n\n", ch.Script)

	if len(ch.VisualReferences) > 0 {
		writeList(b, "Visual references", ch.VisualReferences)
	}
	if len(ch.Demonstration) > 0 {
		b.WriteString("**Demonstration:**\n")
		for _, d := range ch.Demonstration {
			fmt.Fprintf(b, "  %d. %s", d.Step, d.Action)
			if d.Detail != "" {
				fmt.Fprintf(b, " — %s", d.Detail)
			}
			b.WriteString("\n")
		}
	}
	if len(ch.Callouts) > 0 {
		b.WriteString("**Callouts:**\n")
		for _, co := range ch.Callouts {
			fmt.Fprintf(b, "  - [%s] %s\n", co.Kind, co.Text)
		}
	}
	if len(ch.Engagement) > 0 {
		for _, e := range ch.Engagement {
			fmt.Fprintf(b, "- 💬 **Engage:** %s\n", e)
		}
	}
	fmt.Fprintf(b, "- **Transition:** %s\n\n", ch.Transition)
}

func writeMetadata(b *strings.Builder, s YouTubeScript) {
	ci := s.ContentIntelligence
	b.WriteString("---\n\n## Production Metadata\n\n")
	fmt.Fprintf(b, "- **Estimated runtime:** %s · **Speaking time:** %s · **Words:** %d\n", ci.EstimatedRuntime, ci.SpeakingTime, ci.WordCount)
	fmt.Fprintf(b, "- **Audience:** %s · **Difficulty:** %s · **Reading level:** %s\n", ci.Audience, ci.Difficulty, ci.ReadingLevel)
	fmt.Fprintf(b, "- **Suggested title:** %s\n", ci.SuggestedTitle)
	writeList(b, "Alternative titles", ci.AlternativeTitles)
	fmt.Fprintf(b, "- **Thumbnail text:** %s\n", ci.SuggestedThumbnail)
	fmt.Fprintf(b, "- **Playlist:** %s\n", ci.SuggestedPlaylist)
	writeList(b, "Technical topics", ci.TechnicalTopics)
	writeList(b, "SEO keywords", ci.SEOKeywords)
	writeList(b, "Tags", ci.SuggestedTags)
	if len(ci.Chapters) > 0 {
		b.WriteString("- **Chapter markers:**\n")
		for _, m := range ci.Chapters {
			fmt.Fprintf(b, "  - `%s` %s\n", m.Timestamp, m.Title)
		}
	}
	if ci.PinnedComment != "" {
		fmt.Fprintf(b, "- **Pinned comment:** %s\n", ci.PinnedComment)
	}
	if ci.SuggestedDescription != "" {
		fmt.Fprintf(b, "\n### Suggested Description\n\n```\n%s\n```\n", ci.SuggestedDescription)
	}
}

func writeList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "- **%s:** %s\n", label, strings.Join(items, "; "))
}

func ints(ns []int) string {
	if len(ns) == 0 {
		return "—"
	}
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = itoa(n)
	}
	return strings.Join(parts, ", ")
}
