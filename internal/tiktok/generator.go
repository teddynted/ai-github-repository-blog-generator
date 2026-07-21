package tiktok

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// ErrNoTopics signals that the release has no TikTok-worthy topics. It is a
// graceful skip (the generator refuses to fabricate content), not a failure —
// callers can errors.Is it to treat the stage as skipped.
var ErrNoTopics = errors.New("tiktok: no TikTok-worthy topics found in the release package")

// Generator adapts a ReleasePackage into a collection of TikTok-native videos.
// It reuses the shared releasegen.Model port; when Model is nil, generation is
// fully deterministic (hooks and scripts are assembled from grounded seeds). It
// never regenerates the upstream artifacts.
type Generator struct {
	Model releasegen.Model
	// MaxVideos caps how many TikToks to produce; <= 0 uses the default (6).
	MaxVideos int
	Now       func() time.Time
	Logger    *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// TikTok converts a ReleasePackage into a collection of 20–60 second TikTok
// video plans, each teaching a single grounded technical concept. Discovery,
// timing, scenes, captions, visuals, camera, animation, engagement, hashtags,
// and metadata are deterministic; the Model only sharpens hooks, scripts, and
// captions.
func (g *Generator) TikTok(ctx context.Context, pkg ReleasePackage) (TikTokCollection, error) {
	if pkg.Context == nil {
		return TikTokCollection{}, fmt.Errorf("tiktok: release context is required")
	}

	topics := discover(pkg, g.MaxVideos)
	if len(topics) == 0 {
		return TikTokCollection{}, ErrNoTopics
	}

	collection := TikTokCollection{
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
				"shorts":         pkg.Shorts.SchemaVersion,
			},
		},
	}

	videos := make([]Video, 0, len(topics))
	for i, t := range topics {
		videos = append(videos, g.buildVideo(ctx, pkg, t, i))
	}

	collection.Videos = videos
	collection.Metadata.VideoCount = len(videos)
	collection.ContentIntelligence = g.planIntelligence(pkg, videos)
	collection.Warnings = collectWarnings(videos)

	if g.Logger != nil {
		g.Logger.Info("tiktok videos generated",
			slog.String("repository", collection.Metadata.Repository),
			slog.String("release", collection.Metadata.Release),
			slog.Int("videos", len(videos)),
			slog.Int("warnings", len(collection.Warnings)),
		)
	}
	return collection, nil
}

// buildVideo assembles one TikTok from a discovered topic.
func (g *Generator) buildVideo(ctx context.Context, pkg ReleasePackage, t topic, index int) Video {
	hook := g.hook(ctx, t)
	engagement := engagementPrompt(t)
	cta := planCTA(t, index)
	problem, solution, takeaway, full := g.script(ctx, t, hook, engagement, cta)

	dur := durationFor(full)
	visuals := planVisuals(t, pkg)
	scenes := planScenes(t, hook, problem, solution, takeaway, engagement, cta, dur, visuals)

	v := Video{
		ID:               index + 1,
		Title:            t.Title,
		Topic:            t.Topic,
		Duration:         mmss(dur),
		DurationSec:      dur,
		Hook:             hook,
		Script:           full,
		Problem:          problem,
		Solution:         solution,
		Takeaway:         takeaway,
		Scenes:           scenes,
		Captions:         planCaptions(full, dur),
		Visuals:          visuals,
		Camera:           cameraDirections(scenes),
		Animations:       animations(scenes),
		EngagementPrompt: engagement,
		CTA:              cta,
		Hashtags:         planHashtags(t, pkg),
		SEO:              planSEO(t, index),
		Source:           Source{Short: t.Short, Chapter: t.Chapter, StoryboardScene: t.StoryboardScene, Seed: firstSentences(t.Seed, 1)},
		WordCount:        wordCount(full),
	}
	v.RetentionScore = retentionScore(v)
	return v
}

func cameraDirections(scenes []Scene) []CameraDir {
	out := make([]CameraDir, 0, len(scenes))
	for _, sc := range scenes {
		out = append(out, CameraDir{Scene: sc.Number, Direction: sc.Camera})
	}
	return out
}

func animations(scenes []Scene) []Animation {
	out := make([]Animation, 0, len(scenes))
	for i, sc := range scenes {
		out = append(out, Animation{Scene: sc.Number, Type: sc.Animation, Sequence: i + 1})
	}
	return out
}

func collectWarnings(videos []Video) []string {
	var w []string
	for _, v := range videos {
		if v.DurationSec < tiktokMinSec || v.DurationSec > tiktokMaxSec {
			w = append(w, fmt.Sprintf("video %d (%s): duration %ds is outside the 20–60s window", v.ID, v.Title, v.DurationSec))
		}
	}
	return w
}
