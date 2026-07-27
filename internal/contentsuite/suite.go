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
	"sync"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/archspec"
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
	DiagramSpec  archspec.Spec
	artifacts    []Artifact
}

// Artifacts returns the rendered outputs for every successful stage, in order.
func (s *Suite) Artifacts() []Artifact { return s.artifacts }

// Orchestrator wires the model + per-generator options. A nil Model runs every
// generator in its deterministic, offline mode (no network) — except the blog,
// which requires a model; supply a pre-generated blog to Run when offline.
type Orchestrator struct {
	Model releasegen.Model
	// ModelFor, when set, selects the model per content kind (Hybrid AI Routing):
	// premium model for high-value artifacts, local model for commodity ones. The
	// argument is the stage name ("blog", "architecture", "seo-metadata", …). When
	// nil, Model is used for every stage — fully backward compatible.
	ModelFor       func(kind string) releasegen.Model
	Logger         *slog.Logger
	MaxShorts      int
	MaxVideos      int
	MaxAssets      int
	MaxPosts       int
	MaxThreads     int
	PostsPerThread int
}

// model returns the model for a stage: the router's per-kind choice when
// ModelFor is set, otherwise the single Model.
func (o *Orchestrator) model(kind string) releasegen.Model {
	if o.ModelFor != nil {
		return o.ModelFor(kind)
	}
	return o.Model
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
		s.record(stageOutcome{name: "blog", milestone: 3, filename: "01-blog.md", status: StageOK, md: s.Blog.Markdown})
	} else {
		s.record(o.run("blog", 3, "01-blog.md", func() (string, error) {
			b, err := (&releasegen.Generator{Model: o.model("blog")}).Blog(ctx, rctx)
			s.Blog = b
			return b.Markdown, err
		}))
	}

	// --- M4 Storyboard (blog + context) ---
	s.record(o.run("storyboard", 4, "02-storyboard.md", func() (string, error) {
		sb, err := (&storyboard.Generator{Model: o.model("storyboard")}).Storyboard(ctx, s.Blog, rctx)
		s.Storyboard = sb
		return sb.Markdown(), err
	}))

	// --- M5 Voice-over (storyboard) ---
	s.record(o.run("voiceover", 5, "03-voiceover.md", func() (string, error) {
		vo, err := (&voiceover.Generator{Model: o.model("voiceover")}).VoiceOver(ctx, s.Storyboard)
		s.VoiceOver = vo
		return vo.Markdown(), err
	}))

	// --- M11 Architecture (context+blog+storyboard) runs CONCURRENTLY with the
	// M6→M10 chain: it depends only on the blog + storyboard (both ready), not on
	// the YouTube→SEO chain. Its result is recorded in canonical position (after
	// SEO) so the manifest/artifact order stays deterministic regardless of timing.
	var wg sync.WaitGroup
	var archOut stageOutcome
	wg.Add(1)
	go func() {
		defer wg.Done()
		archOut = o.run("architecture", 11, "09-architecture.md", func() (string, error) {
			col, err := (&architecture.Generator{Model: o.model("architecture")}).Architecture(ctx, architecture.ReleasePackage{
				Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard,
			})
			s.Architecture = col
			return col.Markdown(), err
		})
	}()

	// --- M6 YouTube script (context+blog+storyboard+voiceover) ---
	s.record(o.run("youtube", 6, "04-youtube-script.md", func() (string, error) {
		sc, err := (&youtube.Generator{Model: o.model("youtube")}).YouTube(ctx, youtube.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver,
		})
		s.YouTube = sc
		return sc.Markdown(), err
	}))

	// --- M7 YouTube Shorts (+youtube) ---
	s.record(o.run("youtube-shorts", 7, "05-youtube-shorts.md", func() (string, error) {
		col, err := (&shorts.Generator{Model: o.model("youtube-shorts"), MaxShorts: o.MaxShorts}).YouTubeShorts(ctx, shorts.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
		})
		s.Shorts = col
		return col.Markdown(), err
	}))

	// --- M8 TikTok (+shorts) ---
	s.record(o.run("tiktok", 8, "06-tiktok.md", func() (string, error) {
		col, err := (&tiktok.Generator{Model: o.model("tiktok"), MaxVideos: o.MaxVideos}).TikTok(ctx, tiktok.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts,
		})
		s.TikTok = col
		return col.Markdown(), err
	}))

	// --- M9 Visual Assets (+tiktok) ---
	s.record(o.run("visual-assets", 9, "07-visual-assets.md", func() (string, error) {
		col, err := (&visualassets.Generator{Model: o.model("visual-assets"), MaxAssets: o.MaxAssets}).VisualAssets(ctx, visualassets.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts, TikTok: s.TikTok,
		})
		s.VisualAssets = col
		return col.Markdown(), err
	}))

	// --- M10 SEO metadata (+visual assets) ---
	s.record(o.run("seo-metadata", 10, "08-seo-metadata.md", func() (string, error) {
		m, err := (&seo.Generator{Model: o.model("seo-metadata")}).SEO(ctx, seo.ReleasePackage{
			Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets,
		})
		s.SEO = m
		return m.Markdown(), err
	}))

	// Join the architecture branch and record it at its canonical position (11).
	wg.Wait()
	s.record(archOut)

	// --- M12 LinkedIn and M13 X thread both depend on everything above but NOT on
	// each other, so they run concurrently; results are recorded in canonical order.
	var liOut, xtOut stageOutcome
	wg.Add(2)
	go func() {
		defer wg.Done()
		liOut = o.run("linkedin", 12, "10-linkedin.md", func() (string, error) {
			col, err := (&linkedin.Generator{Model: o.model("linkedin"), MaxPosts: o.MaxPosts}).LinkedIn(ctx, linkedin.ReleasePackage{
				Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
				Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets, SEO: s.SEO, Architecture: s.Architecture,
			})
			s.LinkedIn = col
			return col.Markdown(), err
		})
	}()
	go func() {
		defer wg.Done()
		xtOut = o.run("x-thread", 13, "11-x-thread.md", func() (string, error) {
			col, err := (&xthread.Generator{Model: o.model("x-thread"), MaxThreads: o.MaxThreads, PostsPerThread: o.PostsPerThread}).XThread(ctx, xthread.ReleasePackage{
				Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
				Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets, SEO: s.SEO, Architecture: s.Architecture,
			})
			s.XThread = col
			return col.Markdown(), err
		})
	}()
	wg.Wait()
	s.record(liOut)
	s.record(xtOut)

	// --- M14 AWS Architecture Diagram Specification (context + blog topic) ---
	// A separate, renderer-facing artifact (NOT appended to the blog): a
	// repository-grounded spec a downstream pipeline turns into an AWS diagram.
	// Skipped gracefully when the repository has no groundable AWS evidence.
	s.record(o.run("architecture-diagram-spec", 14, "12-architecture-diagram-spec.md", func() (string, error) {
		spec, err := (&archspec.Generator{Model: o.model("architecture-diagram-spec"), Logger: o.Logger}).Spec(ctx, archspec.ReleasePackage{
			Context: rctx, Blog: s.Blog,
		})
		s.DiagramSpec = spec
		return spec.Markdown(), err
	}))

	return s
}

// stageOutcome is the computed result of one stage, before it is recorded into
// the manifest. Separating computation from recording lets independent stages run
// concurrently while the manifest and artifact list stay in canonical order.
type stageOutcome struct {
	name      string
	milestone int
	filename  string
	status    StageStatus
	err       error
	md        string
}

// run executes one generator and returns its outcome WITHOUT touching shared
// manifest state, so it is safe to call from concurrent goroutines. A deliberate
// "nothing to do" refusal (no infrastructure to diagram, no Short-worthy moments,
// no TikTok-worthy topics) becomes a graceful skip, not a failure. The caller
// records the outcome (Suite.record) in canonical order.
func (o *Orchestrator) run(name string, milestone int, filename string, fn func() (string, error)) stageOutcome {
	md, err := fn()
	if err != nil {
		status := StageFailed
		if isSkip(err) {
			status = StageSkipped
		}
		if o.Logger != nil {
			o.Logger.Warn("content stage "+string(status), slog.String("stage", name), slog.Int("milestone", milestone), slog.String("error", err.Error()))
		}
		return stageOutcome{name: name, milestone: milestone, status: status, err: err}
	}
	return stageOutcome{name: name, milestone: milestone, filename: filename, status: StageOK, md: md}
}

// isSkip reports whether an error is a grounded "nothing worthy to generate"
// refusal — a graceful skip rather than a failure. New generators that add such a
// sentinel are registered here.
func isSkip(err error) bool {
	return errors.Is(err, architecture.ErrNoInfrastructure) ||
		errors.Is(err, archspec.ErrNoEvidence) ||
		errors.Is(err, shorts.ErrNoMoments) ||
		errors.Is(err, tiktok.ErrNoTopics)
}

// record appends a stage outcome to the manifest (and the artifact list on
// success). It is called only from the orchestrating goroutine, in canonical
// milestone order, so the manifest is deterministic even when stages run
// concurrently.
func (s *Suite) record(o stageOutcome) {
	r := StageResult{Name: o.name, Milestone: o.milestone, Status: o.status, Artifact: o.filename}
	if o.err != nil {
		r.Error = o.err.Error()
	}
	s.Manifest.Stages = append(s.Manifest.Stages, r)
	switch o.status {
	case StageOK:
		s.Manifest.Produced++
		if o.filename != "" {
			s.artifacts = append(s.artifacts, Artifact{Filename: o.filename, Kind: o.name, Markdown: o.md})
		}
	case StageSkipped:
		s.Manifest.Skipped++
	case StageFailed:
		s.Manifest.Failed++
	}
}
