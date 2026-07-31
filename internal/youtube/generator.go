package youtube

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// Generator assembles a long-form YouTube script from a ReleasePackage. It
// reuses the shared releasegen.Model port; when Model is nil, generation is
// fully deterministic (narration is built from the voice-over script and the
// Release Context). It never regenerates the upstream artifacts.
type Generator struct {
	Model releasegen.Model
	// WordsPerMinute sets the narration rate for timing/estimation; <= 0 uses
	// the long-form default (150 wpm).
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

// YouTube converts a ReleasePackage (Release Context + Blog + Storyboard +
// Voice-over) into a production-ready long-form YouTube script. Chapters map 1:1
// to storyboard scenes; hook, introduction, conclusion, and CTA frame them.
// Nothing is invented — every fact is drawn from the consumed artifacts.
func (g *Generator) YouTube(ctx context.Context, pkg ReleasePackage) (YouTubeScript, error) {
	if pkg.Context == nil {
		return YouTubeScript{}, fmt.Errorf("youtube: release context is required")
	}
	if len(pkg.Storyboard.Scenes) == 0 {
		return YouTubeScript{}, fmt.Errorf("youtube: storyboard has no scenes to build chapters from")
	}

	script := YouTubeScript{
		SchemaVersion: SchemaVersion,
		Metadata: Metadata{
			Repository:      repoName(pkg),
			Release:         releaseTag(pkg),
			SourceBlogTitle: pkg.Blog.Title,
			GeneratedAt:     g.now().Format(time.RFC3339),
			SourceSchemas: map[string]string{
				"releaseContext": pkg.Context.SchemaVersion,
				"storyboard":     pkg.Storyboard.SchemaVersion,
				"voiceOver":      pkg.VoiceOver.SchemaVersion,
			},
		},
	}

	// 1. Hook (pre-roll at 00:00).
	script.Hook = g.hook(ctx, pkg)
	script.Hook.Timestamp = stamp(0, script.Hook.DurationSec)

	// 2. Chapters (1:1 with storyboard scenes), starting after the hook.
	chapters := g.chapters(ctx, pkg, script.Hook.DurationSec)
	script.Chapters = chapters

	// 3. Introduction & Conclusion frame the chapters (references, not extra
	//    runtime — the intro/conclusion scenes are already chapters).
	script.Introduction = g.introduction(ctx, pkg, chapters)
	if len(chapters) > 0 {
		script.Introduction.Timestamp = chapters[0].Timestamp
	}

	runtimeSec := script.Hook.DurationSec
	for _, ch := range chapters {
		runtimeSec += ch.Duration.TargetSec
	}
	// The conclusion timestamp sits on the final (conclusion) chapter if present.
	conclusionStart := script.Hook.DurationSec
	if len(chapters) > 0 {
		conclusionStart = chapters[len(chapters)-1].Timestamp.StartSec
	}
	script.Conclusion = g.conclusion(ctx, pkg, chapters, conclusionStart)
	if len(chapters) > 0 {
		script.Conclusion.Timestamp = chapters[len(chapters)-1].Timestamp
	}

	// 4. Call to action.
	script.CallToAction = g.planCTA(pkg)

	// Runtime must never be shorter than the time it takes to speak every
	// narration block (#4). Chapters are already sized to their words; the
	// intro/conclusion/CTA overlay chapters, so fold their spoken time in too. The
	// result keeps EstimatedRuntime and SpeakingTime consistent instead of the old
	// 4:54-vs-12:40 split.
	if s := speakingSeconds(totalWords(script), g.wpm()); s > runtimeSec {
		runtimeSec = s
	}

	// 5. Video envelope.
	script.Video = Video{
		Title:               suggestedTitle(pkg),
		Format:              "long-form",
		Duration:            mmss(runtimeSec),
		DurationSec:         runtimeSec,
		Audience:            audience(pkg),
		Difficulty:          difficulty(pkg),
		TargetMinRuntimeSec: targetMinRuntimeSec,
		TargetMaxRuntimeSec: targetMaxRuntimeSec,
	}

	// 6. Content intelligence + warnings.
	script.ContentIntelligence = g.planIntelligence(pkg, script)
	script.Warnings = collectWarnings(script)

	if g.Logger != nil {
		g.Logger.Info("youtube script generated",
			slog.String("repository", script.Metadata.Repository),
			slog.String("release", script.Metadata.Release),
			slog.Int("chapters", len(script.Chapters)),
			slog.Int("runtime_sec", script.Video.DurationSec),
			slog.Int("words", script.ContentIntelligence.WordCount),
			slog.Int("warnings", len(script.Warnings)),
		)
	}
	return script, nil
}

func audience(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.ContentIntelligence.TargetAudience != "" {
		return pkg.Context.ContentIntelligence.TargetAudience
	}
	return "Software engineers and cloud practitioners"
}

func difficulty(pkg ReleasePackage) string {
	if pkg.Context != nil {
		switch pkg.Context.ContentIntelligence.ImplementationComplexity {
		case "high":
			return "advanced"
		case "low":
			return "beginner"
		default:
			return "intermediate"
		}
	}
	return "intermediate"
}

// collectWarnings surfaces non-fatal production concerns — notably an under-run
// of the 10-minute long-form target (a thin blog can't be padded).
func collectWarnings(s YouTubeScript) []string {
	var w []string
	if s.Video.DurationSec < s.Video.TargetMinRuntimeSec {
		w = append(w, fmt.Sprintf(
			"runtime %s is under the %d-minute long-form target; the upstream blog/storyboard may be too thin for a full long-form video",
			mmss(s.Video.DurationSec), s.Video.TargetMinRuntimeSec/60))
	}
	if s.Video.DurationSec > s.Video.TargetMaxRuntimeSec {
		w = append(w, fmt.Sprintf(
			"runtime %s exceeds the %d-minute long-form target; consider splitting into a series",
			mmss(s.Video.DurationSec), s.Video.TargetMaxRuntimeSec/60))
	}
	return w
}
