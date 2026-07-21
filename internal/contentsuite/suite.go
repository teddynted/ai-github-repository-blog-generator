// Package contentsuite is the single orchestrator that turns one Release Context
// into the complete set of release artifacts (Milestones 3–13) in dependency
// order, wiring the dedicated generator packages in-process. It is the "fan-out"
// that the QA quality gate (H1) found missing: before it, an operator had to run
// ~10 CLIs by hand, threading each stage's output file into the next.
//
// Design:
//   - It composes the EXISTING, tested generator packages (storyboard, voiceover,
//     youtube, shorts, tiktok, visualassets, seo, architecture, linkedin,
//     xthread) — no generation logic is duplicated here.
//   - Every stage is fault-tolerant: a stage that fails (e.g. a storyboard with no
//     sections, or a release with no groundable infrastructure) is recorded in the
//     manifest and the run CONTINUES; one weak input never aborts the whole
//     pipeline. This also resolves the QA edge cases M1/M2 at the orchestration
//     level without weakening any generator's grounding.
//   - It is pure (no file I/O): Run returns a Suite; the caller writes artifacts.
package contentsuite

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/linkedin"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/seo"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/shorts"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/tiktok"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/visualassets"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/xthread"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// SchemaVersion is the suite manifest version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// StageStatus is one stage's outcome.
type StageStatus string

const (
	StageOK      StageStatus = "ok"
	StageFailed  StageStatus = "failed"
	StageSkipped StageStatus = "skipped"
)

// StageResult records how one artifact stage went.
type StageResult struct {
	Name      string      `json:"name"`
	Milestone int         `json:"milestone"`
	Status    StageStatus `json:"status"`
	Artifact  string      `json:"artifact,omitempty"` // output filename when ok
	Error     string      `json:"error,omitempty"`
}

// Manifest correlates a release with every artifact produced for it.
type Manifest struct {
	SchemaVersion string        `json:"schemaVersion"`
	Repository    string        `json:"repository"`
	Release       string        `json:"release"`
	ContextID     string        `json:"contextId"`
	GeneratedAt   time.Time     `json:"generatedAt"`
	Offline       bool          `json:"offline"`
	Stages        []StageResult `json:"stages"`
	Produced      int           `json:"produced"`
	Skipped       int           `json:"skipped"`
	Failed        int           `json:"failed"`
}

// Artifact is one rendered output ready to write.
type Artifact struct {
	Filename string `json:"filename"`
	Kind     string `json:"kind"`
	Markdown string `json:"-"`
}

// Suite is the complete set of artifacts from one release.
type Suite struct {
	Manifest     Manifest
	Blog         releasegen.BlogPost
	Storyboard   storyboard.Storyboard
	VoiceOver    voiceover.VoiceOverScript
	YouTube      youtube.YouTubeScript
	Shorts       shorts.ShortsCollection
	TikTok       tiktok.TikTokCollection
	VisualAssets visualassets.VisualAssetCollection
	SEO          seo.SEOMetadata
	Architecture architecture.ArchitectureCollection
	LinkedIn     linkedin.LinkedInCollection
	XThread      xthread.XThreadCollection
	artifacts    []Artifact
}

// Artifacts returns the rendered outputs for every successful stage, in order.
func (s *Suite) Artifacts() []Artifact { return s.artifacts }

// Orchestrator wires the model + per-generator options. A nil Model runs every
// generator in its deterministic, offline mode (no network) — except the blog,
// which requires a model; supply a pre-generated blog to Run when offline.
type Orchestrator struct {
	Model          releasegen.Model
	Logger         *slog.Logger
	MaxShorts      int
	MaxVideos      int
	MaxAssets      int
	MaxPosts       int
	MaxThreads     int
	PostsPerThread int
}

// Run executes every stage for one release. If blog is nil it is generated first
// (requires a Model); otherwise the supplied blog is used verbatim (the offline
// path). It never returns a fatal error for a single failed stage — inspect the
// manifest for per-stage status.
func (o *Orchestrator) Run(ctx context.Context, rctx *rc.ReleaseContext, blog *releasegen.BlogPost) *Suite {
	s := &Suite{Manifest: Manifest{
		SchemaVersion: SchemaVersion,
		Repository:    rctx.Repository.FullName,
		Release:       rctx.Release.Tag,
		ContextID:     rctx.ContextID,
		GeneratedAt:   time.Now(),
		Offline:       o.Model == nil,
	}}

	// --- M3 Blog (foundational) ---
	if blog != nil {
		s.Blog = *blog
		s.record("blog", 3, StageOK, "01-blog.md", nil, s.Blog.Markdown)
	} else {
		o.stage(s, "blog", 3, "01-blog.md", func() (string, error) {
			b, err := (&releasegen.Generator{Model: o.Model}).Blog(ctx, rctx)
			s.Blog = b
			return b.Markdown, err
		})
	}

	// --- M4 Storyboard (blog + context) ---
	o.stage(s, "storyboard", 4, "02-storyboard.md", func() (string, error) {
		sb, err := (&storyboard.Generator{Model: o.Model}).Storyboard(ctx, s.Blog, rctx)
		s.Storyboard = sb
		return sb.Markdown(), err
	})

	// --- M5 Voice-over (storyboard) ---
	o.stage(s, "voiceover", 5, "03-voiceover.md", func() (string, error) {
		vo, err := (&voiceover.Generator{Model: o.Model}).VoiceOver(ctx, s.Storyboard)
		s.VoiceOver = vo
		return vo.Markdown(), err
	})

	// --- M6 YouTube script (context+blog+storyboard+voiceover) ---
	o.stage(s, "youtube", 6, "04-youtube-script.md", func() (string, error) {
		sc, err := (&youtube.Generator{Model: o.Model}).YouTube(ctx, youtube.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver,
		})
		s.YouTube = sc
		return sc.Markdown(), err
	})

	// --- M7 YouTube Shorts (+youtube) ---
	o.stage(s, "youtube-shorts", 7, "05-youtube-shorts.md", func() (string, error) {
		col, err := (&shorts.Generator{Model: o.Model, MaxShorts: o.MaxShorts}).YouTubeShorts(ctx, shorts.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
		})
		s.Shorts = col
		return col.Markdown(), err
	})

	// --- M8 TikTok (+shorts) ---
	o.stage(s, "tiktok", 8, "06-tiktok.md", func() (string, error) {
		col, err := (&tiktok.Generator{Model: o.Model, MaxVideos: o.MaxVideos}).TikTok(ctx, tiktok.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts,
		})
		s.TikTok = col
		return col.Markdown(), err
	})

	// --- M9 Visual Assets (+tiktok) ---
	o.stage(s, "visual-assets", 9, "07-visual-assets.md", func() (string, error) {
		col, err := (&visualassets.Generator{Model: o.Model, MaxAssets: o.MaxAssets}).VisualAssets(ctx, visualassets.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts, TikTok: s.TikTok,
		})
		s.VisualAssets = col
		return col.Markdown(), err
	})

	// --- M10 SEO metadata (+visual assets) ---
	o.stage(s, "seo-metadata", 10, "08-seo-metadata.md", func() (string, error) {
		m, err := (&seo.Generator{Model: o.Model}).SEO(ctx, seo.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets,
		})
		s.SEO = m
		return m.Markdown(), err
	})

	// --- M11 Architecture diagrams (context+blog+storyboard) ---
	o.stage(s, "architecture", 11, "09-architecture.md", func() (string, error) {
		col, err := (&architecture.Generator{Model: o.Model}).Architecture(ctx, architecture.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard,
		})
		s.Architecture = col
		return col.Markdown(), err
	})

	// --- M12 LinkedIn (everything) ---
	o.stage(s, "linkedin", 12, "10-linkedin.md", func() (string, error) {
		col, err := (&linkedin.Generator{Model: o.Model, MaxPosts: o.MaxPosts}).LinkedIn(ctx, linkedin.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
			Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets, SEO: s.SEO, Architecture: s.Architecture,
		})
		s.LinkedIn = col
		return col.Markdown(), err
	})

	// --- M13 X thread (everything) ---
	o.stage(s, "x-thread", 13, "11-x-thread.md", func() (string, error) {
		col, err := (&xthread.Generator{Model: o.Model, MaxThreads: o.MaxThreads, PostsPerThread: o.PostsPerThread}).XThread(ctx, xthread.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
			Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets, SEO: s.SEO, Architecture: s.Architecture,
		})
		s.XThread = col
		return col.Markdown(), err
	})

	return s
}

// stage runs one generator, recording the outcome and appending the artifact on
// success. A failure is logged and recorded — never fatal.
func (o *Orchestrator) stage(s *Suite, name string, milestone int, filename string, fn func() (string, error)) {
	md, err := fn()
	if err != nil {
		// A deliberate "nothing to do" refusal (no infrastructure to diagram, no
		// Short-worthy moments, no TikTok-worthy topics) is a graceful skip, not a
		// failure — the generator is honouring its grounding, not breaking.
		status := StageFailed
		if isSkip(err) {
			status = StageSkipped
		}
		s.record(name, milestone, status, "", err, "")
		if o.Logger != nil {
			o.Logger.Warn("content stage "+string(status), slog.String("stage", name), slog.Int("milestone", milestone), slog.String("error", err.Error()))
		}
		return
	}
	s.record(name, milestone, StageOK, filename, nil, md)
}

// isSkip reports whether an error is a grounded "nothing worthy to generate"
// refusal — a graceful skip rather than a failure. New generators that add such a
// sentinel are registered here.
func isSkip(err error) bool {
	return errors.Is(err, architecture.ErrNoInfrastructure) ||
		errors.Is(err, shorts.ErrNoMoments) ||
		errors.Is(err, tiktok.ErrNoTopics)
}

func (s *Suite) record(name string, milestone int, status StageStatus, filename string, err error, md string) {
	r := StageResult{Name: name, Milestone: milestone, Status: status, Artifact: filename}
	if err != nil {
		r.Error = err.Error()
	}
	s.Manifest.Stages = append(s.Manifest.Stages, r)
	switch status {
	case StageOK:
		s.Manifest.Produced++
		if filename != "" {
			s.artifacts = append(s.artifacts, Artifact{Filename: filename, Kind: name, Markdown: md})
		}
	case StageSkipped:
		s.Manifest.Skipped++
	case StageFailed:
		s.Manifest.Failed++
	}
}
