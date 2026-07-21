package shorts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// ErrNoMoments signals that the release has no Short-worthy moments. It is a
// graceful skip (the generator refuses to fabricate content), not a failure —
// callers can errors.Is it to treat the stage as skipped.
var ErrNoMoments = errors.New("shorts: no Short-worthy moments found in the release package")

// Generator mines a ReleasePackage for the best technical moments and plans a
// collection of standalone YouTube Shorts. It reuses the shared releasegen.Model
// port; when Model is nil, generation is fully deterministic (hooks and scripts
// are assembled from grounded seeds). It never regenerates the upstream
// artifacts.
type Generator struct {
	Model releasegen.Model
	// MaxShorts caps how many Shorts to produce; <= 0 uses the default (6).
	MaxShorts int
	Now       func() time.Time
	Logger    *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// YouTubeShorts converts a ReleasePackage into a collection of 30–60 second
// Short plans, each communicating a single grounded technical idea. Discovery,
// timing, scenes, visuals, camera, animation, hashtags, and metadata are
// deterministic; the Model only sharpens hooks, scripts, and captions.
func (g *Generator) YouTubeShorts(ctx context.Context, pkg ReleasePackage) (ShortsCollection, error) {
	if pkg.Context == nil {
		return ShortsCollection{}, fmt.Errorf("shorts: release context is required")
	}

	candidates := discover(pkg, g.MaxShorts)
	if len(candidates) == 0 {
		return ShortsCollection{}, ErrNoMoments
	}

	collection := ShortsCollection{
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
				"youTube":        pkg.YouTube.SchemaVersion,
			},
		},
	}

	shorts := make([]Short, 0, len(candidates))
	for i, c := range candidates {
		shorts = append(shorts, g.buildShort(ctx, pkg, c, i))
	}

	collection.Shorts = shorts
	collection.Metadata.ShortCount = len(shorts)
	collection.ContentIntelligence = g.planIntelligence(pkg, shorts)
	collection.Warnings = collectWarnings(shorts)

	if g.Logger != nil {
		g.Logger.Info("shorts generated",
			slog.String("repository", collection.Metadata.Repository),
			slog.String("release", collection.Metadata.Release),
			slog.Int("shorts", len(shorts)),
			slog.Int("warnings", len(collection.Warnings)),
		)
	}
	return collection, nil
}

// buildShort assembles one Short from a discovered candidate.
func (g *Generator) buildShort(ctx context.Context, pkg ReleasePackage, c candidate, index int) Short {
	hook := g.hook(ctx, c)
	cta := planCTA(c, index)
	core, takeaway, full := g.script(ctx, c, hook, cta)

	dur := durationFor(full)
	visuals := planVisuals(c, pkg)
	scenes := planScenes(c, hook, core, takeaway, cta, dur, visuals)

	return Short{
		ID:              index + 1,
		Title:           c.Title,
		Angle:           c.Angle,
		Duration:        mmss(dur),
		DurationSec:     dur,
		Hook:            hook,
		Script:          full,
		CoreExplanation: core,
		Takeaway:        takeaway,
		Scenes:          scenes,
		Captions:        planCaptions(full, dur),
		Visuals:         visuals,
		Camera:          cameraDirections(scenes),
		Animations:      animations(scenes),
		CTA:             cta,
		Hashtags:        planHashtags(c, pkg),
		SEO:             planSEO(c, pkg, index),
		Source:          Source{Chapter: c.Chapter, StoryboardScene: c.StoryboardScene, Seed: firstSentences(c.Seed, 1)},
		WordCount:       wordCount(full),
	}
}

// cameraDirections lifts the per-scene camera into the Short-level list.
func cameraDirections(scenes []Scene) []CameraDir {
	out := make([]CameraDir, 0, len(scenes))
	for _, sc := range scenes {
		out = append(out, CameraDir{Scene: sc.Number, Direction: sc.Camera})
	}
	return out
}

// animations lifts the per-scene animation into the Short-level sequence.
func animations(scenes []Scene) []Animation {
	out := make([]Animation, 0, len(scenes))
	for i, sc := range scenes {
		out = append(out, Animation{Scene: sc.Number, Type: sc.Animation, Sequence: i + 1})
	}
	return out
}

// collectWarnings surfaces non-fatal concerns per Short.
func collectWarnings(shorts []Short) []string {
	var w []string
	for _, s := range shorts {
		if s.DurationSec < shortMinSec || s.DurationSec > shortMaxSec {
			w = append(w, fmt.Sprintf("short %d (%s): duration %ds is outside the 30–60s window", s.ID, s.Title, s.DurationSec))
		}
	}
	return w
}
