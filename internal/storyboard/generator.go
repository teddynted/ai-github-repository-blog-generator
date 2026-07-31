package storyboard

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// Generator builds a Storyboard from a technical blog and its Release Context.
// It reuses the shared releasegen.Model port; when Model is nil, generation is
// fully deterministic (narration is drawn from the blog prose).
type Generator struct {
	Model releasegen.Model
	// WordsPerSecond sets narration pacing for timing; <= 0 uses the default.
	WordsPerSecond float64
	Now            func() time.Time
	Logger         *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// Storyboard converts a generated blog post (Milestone 3) and its Release
// Context (Milestone 2) into a structured, scene-by-scene storyboard. Diagram
// references come only from the parsed Mermaid in rctx — architecture is never
// invented.
func (g *Generator) Storyboard(ctx context.Context, post releasegen.BlogPost, rctx *rc.ReleaseContext) (Storyboard, error) {
	if rctx == nil {
		return Storyboard{}, fmt.Errorf("storyboard: release context is required")
	}
	sb := Storyboard{
		SchemaVersion: SchemaVersion,
		Metadata: Metadata{
			Repository:      rctx.Repository.FullName,
			Release:         rctx.Release.Tag,
			SourceBlogTitle: post.Title,
			GeneratedAt:     g.now().Format(time.RFC3339),
		},
	}

	var extraWarnings []string
	sections := extractSections(post.Markdown)
	if len(sections) == 0 {
		// Graceful degradation: scene the blog as a single overview from its body
		// rather than failing on a section-less (small) release.
		sections = syntheticSections(post.Markdown, post.Title)
		if len(sections) == 0 {
			return sb, fmt.Errorf("storyboard: blog has no sections and no body to scene")
		}
		extraWarnings = append(extraWarnings, "blog had no ## sections; storyboarded a single overview scene from the intro")
	}

	// Scope diagrams to the release's OWN diagram (the blog's Mermaid), not the
	// whole-repository set in rctx.Mermaid — which pulls in unrelated README,
	// release-management, and docs diagrams and explodes the animation plan.
	releaseDiagrams := rc.AnalyzeMarkdown(post.Markdown, "release architecture diagram")

	scenes := make([]Scene, 0, len(sections))
	var priorNarration []string // what earlier scenes already said, for de-duplication
	diagramIntroduced := false  // the primary diagram is built once, then recalled
	for i, sec := range sections {
		typ := sceneType(sec.Title)
		sc := Scene{
			SceneNumber: i + 1,
			Title:       sec.Title,
			Type:        typ,
			Objective:   objectiveFor(typ, sec.Title),
		}
		nextTitle := ""
		if i+1 < len(sections) {
			nextTitle = sections[i+1].Title
		}
		// Narration is one (potentially slow) model call per scene; log position so
		// a long CPU-bound run — e.g. Ollama polishing every scene — shows steady
		// scene-by-scene progress rather than going silent between the whole-run
		// start and finish lines.
		if g.Logger != nil && g.Model != nil {
			g.Logger.Info("storyboard narrating scene",
				slog.Int("scene", i+1), slog.Int("of", len(sections)), slog.String("title", sec.Title))
		}
		sc.Narration = g.narration(ctx, sec, typ, nextTitle, strings.Join(priorNarration, " "))
		// Enforce the per-scene timing budget on sentence boundaries so the spoken
		// narration always fits its allocated slot (the voice-over never marks it
		// "over"). Trim before timing and dedup so both see the final words.
		sc.Narration = capitalizeFirst(fitToSceneBudget(sc.Narration, g.rate()))
		if strings.TrimSpace(sc.Narration) != "" {
			priorNarration = append(priorNarration, sc.Narration)
		}
		sc.Duration = planDuration(sc.Narration, g.WordsPerSecond)
		sc.Diagrams = planDiagrams(typ, releaseDiagrams)
		// Build the diagram from scratch only on its first appearance; later
		// architecture scenes recall it instead of rebuilding the same graph.
		repeatDiagram := len(sc.Diagrams) > 0 && diagramIntroduced
		if len(sc.Diagrams) > 0 {
			diagramIntroduced = true
		}
		sc.Code = planCode(sec.Body)
		sc.Camera = planCamera(typ)
		sc.Animations = planAnimations(typ, sc.Diagrams, sc.Code, repeatDiagram)
		sc.Overlays = planOverlays(typ, sec.Title, rctx)
		sc.Assets = planAssets(typ)
		sc.Visuals = planVisuals(typ, sec.Title, sc.Assets, sc.Diagrams, repeatDiagram)
		sc.MusicMood, sc.SoundEffects = planMood(typ)
		scenes = append(scenes, sc)
	}

	// Transitions depend on the following scene, so they run once scenes exist.
	for i := range scenes {
		nextType := ""
		isLast := i == len(scenes)-1
		if !isLast {
			nextType = scenes[i+1].Type
		}
		scenes[i].Transition = planTransition(scenes[i].Type, nextType, isLast)
	}

	sb.Scenes = scenes
	sb.Video = planVideo(scenes, g.rate())
	sb.ContentIntelligence = planIntelligence(scenes, post, rctx, sb.Video)
	sb.Warnings = append(extraWarnings, collectWarnings(sb, rctx)...)

	if g.Logger != nil {
		g.Logger.Info("storyboard generated",
			slog.String("repository", sb.Metadata.Repository),
			slog.String("release", sb.Metadata.Release),
			slog.Int("scenes", len(sb.Scenes)),
			slog.Int("duration_sec", sb.Video.TotalDurationSec),
			slog.Int("warnings", len(sb.Warnings)),
		)
	}
	return sb, nil
}

func (g *Generator) rate() float64 {
	if g.WordsPerSecond > 0 {
		return g.WordsPerSecond
	}
	return defaultWordsPerSecond
}

// planVideo aggregates the scene timings into the whole-video spec.
func planVideo(scenes []Scene, rate float64) VideoSpec {
	total, words := 0, 0
	for _, s := range scenes {
		total += s.Duration.RecommendedSec
		words += wordCount(s.Narration)
	}
	voice := int(float64(words) / rate)
	format, aspect := "long-form", "16:9"
	if total <= 180 {
		format, aspect = "short", "9:16"
	}
	return VideoSpec{
		TargetFormat:         format,
		AspectRatio:          aspect,
		SceneCount:           len(scenes),
		TotalDurationSec:     total,
		VoiceoverDurationSec: voice,
		Pacing:               videoPacing(scenes),
	}
}

func videoPacing(scenes []Scene) string {
	if len(scenes) == 0 {
		return "medium"
	}
	sum := 0
	for _, s := range scenes {
		sum += s.Duration.RecommendedSec
	}
	avg := sum / len(scenes)
	return pacingFor(avg)
}

func collectWarnings(sb Storyboard, rctx *rc.ReleaseContext) []string {
	var w []string
	hasDiagram := false
	for _, s := range sb.Scenes {
		if len(s.Diagrams) > 0 {
			hasDiagram = true
			break
		}
	}
	if !hasDiagram {
		w = append(w, "the release blog has no Mermaid diagram; architecture scenes have no diagram references")
	}
	// Long-form videos target 8–15 minutes; flag when we fall short.
	if sb.Video.TargetFormat == "long-form" && sb.Video.TotalDurationSec < 480 {
		w = append(w, fmt.Sprintf("total runtime %ds is under the 8-minute long-form target; consider a richer blog or the short format", sb.Video.TotalDurationSec))
	}
	return w
}
