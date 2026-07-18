package voiceover

import (
	"fmt"
	"strings"
)

// Markdown renders the voice-over script as a human-readable narration document
// for narrators, editors, and review.
func (s VoiceOverScript) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Voice-over Script: %s\n\n", firstNonEmpty(s.Metadata.SourceBlogTitle, s.Metadata.Repository))
	fmt.Fprintf(&b, "_%s · release %s · %d scenes · ~%s speaking (%d wpm)_\n\n",
		s.Metadata.Repository, s.Metadata.Release, s.Metadata.SceneCount,
		s.ContentIntelligence.EstimatedSpeakingTime, s.Voice.WordsPerMinute)

	fmt.Fprintf(&b, "**Voice:** %s — _%s_  \n", s.Voice.Style, s.Voice.Persona)
	fmt.Fprintf(&b, "**Language:** %s · **Recommended voice:** %s · **Default pace:** %s\n\n",
		s.Voice.Language, s.Voice.RecommendedVoice, s.Voice.DefaultPace)

	if len(s.Warnings) > 0 {
		b.WriteString("> **Notes:** " + strings.Join(s.Warnings, "; ") + "\n\n")
	}

	for _, sc := range s.Scenes {
		writeScene(&b, sc)
	}

	writeIntelligence(&b, s.ContentIntelligence)
	return b.String()
}

func writeScene(b *strings.Builder, sc Scene) {
	fmt.Fprintf(b, "---\n\n## Scene %d — %s\n\n", sc.SceneNumber, sc.Title)
	fmt.Fprintf(b, "- **Timestamp:** `%s` (%ds allocated, ~%ds spoken%s)\n",
		sc.Timestamp.Label, sc.Duration.AllocatedSec, sc.Duration.EstimatedSpeechSec, fitsSuffix(sc.Duration.Fits))
	fmt.Fprintf(b, "- **Voice:** %s · %s · energy %s · pace %s\n",
		sc.VoiceDirection, sc.Emotion, sc.Energy, sc.Pace)
	fmt.Fprintf(b, "- **Opening cue:** %s\n", sc.OpeningCue)

	fmt.Fprintf(b, "\n> %s\n\n", sc.Narration)

	if len(sc.Emphasis) > 0 {
		fmt.Fprintf(b, "- **Emphasize:** %s\n", joinQuoted(sc.Emphasis))
	}
	if len(sc.Pronunciation) > 0 {
		b.WriteString("- **Pronunciation:**\n")
		for _, p := range sc.Pronunciation {
			fmt.Fprintf(b, "  - **%s** — %s (`say-as: %s`)", p.Term, p.Phonetic, p.SayAs)
			if p.Note != "" {
				fmt.Fprintf(b, " — %s", p.Note)
			}
			b.WriteString("\n")
		}
	}
	if len(sc.Pauses) > 0 {
		b.WriteString("- **Pauses:**\n")
		for _, p := range sc.Pauses {
			fmt.Fprintf(b, "  - [%s, %dms] %s — %s\n", p.Type, p.DurationMs, p.Position, p.Note)
		}
	}
	if len(sc.SyncCues) > 0 {
		b.WriteString("- **Sync:**\n")
		for _, c := range sc.SyncCues {
			fmt.Fprintf(b, "  - [%s] %s", c.Visual, c.Cue)
			if c.Target != "" {
				fmt.Fprintf(b, " (%s)", c.Target)
			}
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(b, "- **Closing cue:** %s\n", sc.ClosingCue)
	fmt.Fprintf(b, "- **Transition:** %s\n", sc.Transition)
	b.WriteString("\n")
}

func writeIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Production Intelligence\n\n")
	fmt.Fprintf(b, "- **Total words:** %d\n", ci.TotalWords)
	fmt.Fprintf(b, "- **Estimated speaking time:** %s (~%d wpm)\n", ci.EstimatedSpeakingTime, ci.AverageWordsPerMinute)
	fmt.Fprintf(b, "- **Estimated recording time:** %s (incl. retakes)\n", ci.EstimatedRecordingTime)
	fmt.Fprintf(b, "- **Reading difficulty:** %s · **Technical density:** %s\n", ci.ReadingDifficulty, ci.TechnicalDensity)
	fmt.Fprintf(b, "- **Voice style:** %s\n", ci.VoiceStyle)
	fmt.Fprintf(b, "- **Recommended TTS voice:** %s · **Language:** %s\n", ci.RecommendedTTSVoice, ci.RecommendedLanguage)
	fmt.Fprintf(b, "- **Unique pronunciations:** %d\n", ci.UniquePronunciations)
	if len(ci.ProductionNotes) > 0 {
		b.WriteString("- **Production notes:**\n")
		for _, n := range ci.ProductionNotes {
			fmt.Fprintf(b, "  - %s\n", n)
		}
	}
	b.WriteString("\n")
}

func fitsSuffix(fits bool) string {
	if fits {
		return ""
	}
	return ", ⚠ over"
}

func joinQuoted(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = "\"" + s + "\""
	}
	return strings.Join(q, ", ")
}
