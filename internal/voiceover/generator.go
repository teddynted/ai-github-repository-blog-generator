package voiceover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
)

// Generator turns a Storyboard into a structured, synchronized voice-over script.
// It reuses the shared releasegen.Model port; when Model is nil, generation is
// fully deterministic (narration is taken from the storyboard verbatim). It
// never regenerates the storyboard — it enriches it with narration.
type Generator struct {
	Model releasegen.Model
	// WordsPerMinute sets the narration rate for timing/estimation; <= 0 uses
	// the default (156 wpm), which matches the storyboard's pacing model.
	WordsPerMinute int
	Now            func() time.Time
	Logger         *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

func (g *Generator) wpm() int {
	if g.WordsPerMinute > 0 {
		return g.WordsPerMinute
	}
	return defaultWordsPerMinute
}

// VoiceOver converts a Storyboard (Milestone 4) into a VoiceOverScript. Every
// storyboard scene becomes a narration scene aligned to its timing, camera,
// diagrams, and code. Nothing is invented: narration comes from the storyboard,
// pronunciation/emphasis are extracted from that narration, and sync cues point
// at the storyboard's real visuals.
func (g *Generator) VoiceOver(ctx context.Context, sb storyboard.Storyboard) (VoiceOverScript, error) {
	if len(sb.Scenes) == 0 {
		return VoiceOverScript{}, fmt.Errorf("voiceover: storyboard has no scenes")
	}

	voice := g.voiceProfile()
	script := VoiceOverScript{
		SchemaVersion: SchemaVersion,
		Voice:         voice,
		Metadata: Metadata{
			Repository:       sb.Metadata.Repository,
			Release:          sb.Metadata.Release,
			SourceBlogTitle:  sb.Metadata.SourceBlogTitle,
			GeneratedAt:      g.now().Format(time.RFC3339),
			SceneCount:       len(sb.Scenes),
			TotalDurationSec: sb.Video.TotalDurationSec,
		},
	}

	// Sequential, gap-free timestamps from the storyboard's allocated durations.
	allocations := make([]int, len(sb.Scenes))
	for i, sc := range sb.Scenes {
		allocations[i] = sc.Duration.RecommendedSec
	}
	stamps := timeline(allocations)

	releaseTag := sb.Metadata.Release
	scenes := make([]Scene, len(sb.Scenes))
	uniqueTerms := map[string]bool{}

	for i, sc := range sb.Scenes {
		isFirst := i == 0
		isLast := i == len(sb.Scenes)-1
		dir := directionFor(sc.Type)

		narration := g.narration(ctx, sc.Narration, sc.Title, sc.Type, dir.Direction)
		// Normalize spoken form ("manifest. json" -> "manifest.json", then
		// "an /etc/x" -> "the file /etc/x"). Runs BEFORE the fit-trim below so any
		// word change is counted against the slot rather than overflowing it.
		narration = speakFilePaths(joinFileExtensions(narration))
		// A narration model may refine the base into more words than the scene's
		// slot allows; trim (on sentence boundaries) so the spoken estimate never
		// exceeds the storyboard's allocated duration — the video timeline stays
		// authoritative and no scene is reported "over".
		narration = fitNarrationToBudget(narration, sc.Duration.RecommendedSec, g.wpm())
		// If trimming the refinement had to drop a whole trailing sentence and fell
		// below the slot-sized base narration, use the base instead. The base is
		// derived FROM this scene's allocation, so it fills the slot and fits — this
		// avoids under-filling with dead air when the model overshot the budget.
		if base := speakFilePaths(joinFileExtensions(collapse(sc.Narration))); wordCount(narration) < wordCount(base) {
			narration = base
		}
		narration = expandAbbreviations(capitalizeFirst(narration))
		pron := planPronunciation(narration)
		for _, p := range pron {
			uniqueTerms[lower(p.Term)] = true
		}

		nextType, nextTitle := "", ""
		if !isLast {
			nextType = sb.Scenes[i+1].Type
			nextTitle = sb.Scenes[i+1].Title
		}

		scenes[i] = Scene{
			SceneNumber:    sc.SceneNumber,
			Title:          sc.Title,
			Timestamp:      stamps[i],
			Duration:       planDuration(narration, sc.Duration.RecommendedSec, g.wpm()),
			Pace:           dir.Pace,
			VoiceDirection: dir.Direction,
			Emotion:        dir.Emotion,
			Energy:         dir.Energy,
			OpeningCue:     openingCue(sc.Type, isFirst),
			Narration:      narration,
			Pronunciation:  pron,
			Emphasis:       planEmphasis(narration, releaseTag),
			Pauses: planPauses(pauseInput{
				SceneType:  sc.Type,
				Narration:  narration,
				HasDiagram: len(sc.Diagrams) > 0,
				HasCode:    len(sc.Code) > 0,
				IsLast:     isLast,
			}),
			SyncCues:   planSyncCues(sc),
			Transition: planTransition(sc.Type, nextType, nextTitle, isLast),
			ClosingCue: closingCue(sc.Type, isLast),
			WordCount:  wordCount(narration),
		}
	}

	script.Scenes = scenes
	script.ContentIntelligence = planIntelligence(scenes, voice, len(uniqueTerms))
	script.Warnings = collectWarnings(script)

	if g.Logger != nil {
		g.Logger.Info("voice-over generated",
			slog.String("repository", script.Metadata.Repository),
			slog.String("release", script.Metadata.Release),
			slog.Int("scenes", len(script.Scenes)),
			slog.Int("words", script.ContentIntelligence.TotalWords),
			slog.String("speaking_time", script.ContentIntelligence.EstimatedSpeakingTime),
			slog.Int("warnings", len(script.Warnings)),
		)
	}
	return script, nil
}

// voiceProfile builds the global, TTS-agnostic voice direction for the script.
func (g *Generator) voiceProfile() Voice {
	return Voice{
		Style:            "Professional, educational, conversational",
		Persona:          "an experienced software engineer teaching another engineer",
		Tone:             "warm, precise, encouraging",
		DefaultPace:      "Conversational",
		Language:         "en-US",
		RecommendedVoice: "Neural, en-US, warm and conversational (provider-neutral)",
		WordsPerMinute:   g.wpm(),
	}
}

// collectWarnings surfaces non-fatal production concerns.
func collectWarnings(s VoiceOverScript) []string {
	var w []string
	for _, sc := range s.Scenes {
		if !sc.Duration.Fits {
			w = append(w, fmt.Sprintf(
				"scene %d (%s): narration ~%ds may exceed the allocated %ds — trim or extend the scene",
				sc.SceneNumber, sc.Title, sc.Duration.EstimatedSpeechSec, sc.Duration.AllocatedSec))
		}
	}
	if strings.TrimSpace(s.Metadata.Repository) == "" {
		w = append(w, "storyboard metadata has no repository; the script is missing provenance")
	}
	return w
}
