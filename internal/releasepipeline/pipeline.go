// Package releasepipeline closes the loop from a GitHub release to published
// content. It composes the Milestone 2 Release Context Builder and the
// Milestone 3 content generator with the platform's existing review, publish,
// and notify stages — so a release becomes reviewed, published Markdown without
// bespoke glue at each call site.
//
// It depends on small ports (all satisfied by existing types), so the whole
// flow is unit-testable end to end with fakes.
package releasepipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentsuite"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/semver"
)

// ErrInvalidVersion is returned when a release tag is not valid SemVer 2.0.0, so
// callers can errors.Is it (the pipeline fails this gate before any generation).
var ErrInvalidVersion = errors.New("releasepipeline: invalid semantic version")

// ContextBuilder builds a Release Context (satisfied by *releasecontext.Builder).
type ContextBuilder interface {
	Build(ctx context.Context, req rc.Request) (*rc.ReleaseContext, error)
}

// Analyzer extracts the structured EngineeringContext (Stage 2 of the content
// pipeline) from the factual context. Optional; when set, the pipeline runs it
// between context-building and generation and attaches the result to the
// ReleaseContext, so every generator is grounded in the engineering reasoning —
// decisions, trade-offs, service choices — rather than the raw facts alone.
// Satisfied by *engineeringanalysis.Analyzer.
type Analyzer interface {
	Analyze(ctx context.Context, rctx *rc.ReleaseContext) (*rc.EngineeringContext, error)
}

// Generator produces content assets (satisfied by *releasegen.Generator via the
// adapter methods below; used through the concrete type for the blog path).
type Generator interface {
	Blog(ctx context.Context, rctx *rc.ReleaseContext) (releasegen.BlogPost, error)
	Generate(ctx context.Context, format releasegen.Format, rctx *rc.ReleaseContext) (releasegen.Asset, error)
}

// Reviewer gates generated assets (satisfied by review.Reviewer).
type Reviewer interface {
	Review(ctx context.Context, assets []generation.Content) (passed []generation.Content, issues []string)
}

// Publisher writes approved assets (satisfied by publish.FilePublisher / S3Publisher).
type Publisher interface {
	Publish(ctx context.Context, repoFullName string, assets []generation.Content) error
}

// Notifier reports the outcome (optional). A thin interface so callers can wrap
// the email/webhook notifiers without this package importing them.
type Notifier interface {
	Notify(ctx context.Context, subject, body string) error
}

// Pipeline orchestrates release → context → analysis → content → review →
// publish → notify.
type Pipeline struct {
	Builder ContextBuilder
	// Analyzer is the optional Stage-2 engineering analysis. When set, its
	// structured output is attached to the context before generation; a failure
	// is non-fatal (generation proceeds on the factual context).
	Analyzer  Analyzer
	Generator Generator
	// Suite, when set, produces the FULL multimedia artifact set (blog, storyboard,
	// voice-over, YouTube, Shorts, TikTok, visual assets, SEO, architecture,
	// LinkedIn, X thread) via internal/contentsuite, so the automated release path
	// emits every artifact — not just the written formats — and runs them all
	// through the same review/publish/notify stages. When nil the pipeline falls
	// back to Formats (below), preserving the original behaviour.
	Suite *contentsuite.Orchestrator
	// Formats to generate when Suite is nil; defaults to blog + release summary.
	Formats   []releasegen.Format
	Reviewer  Reviewer
	Publisher Publisher
	Notifier  Notifier // optional
	Logger    *slog.Logger
}

// Result summarizes a run.
type Result struct {
	ContextID  string
	Repository string
	Release    string
	Generated  int
	Published  int
	Rejected   int
	Issues     []string
}

// defaultFormats is used when Pipeline.Formats is empty.
var defaultFormats = []releasegen.Format{releasegen.FormatBlog, releasegen.FormatReleaseSummary}

// Run executes the full flow for one repository + release.
func (p *Pipeline) Run(ctx context.Context, req rc.Request) (Result, error) {
	start := time.Now()
	// Generation is served by either the full Suite or the Formats path; require
	// one of them, plus the always-needed stages.
	if p.Builder == nil || (p.Generator == nil && p.Suite == nil) || p.Reviewer == nil || p.Publisher == nil {
		return Result{}, fmt.Errorf("release pipeline is missing a required stage")
	}

	// Semantic Version Validation — the first gate. Reject a non-SemVer release
	// tag before doing any context-building or generation work, so an invalid
	// release fails fast and cheaply. Reuses internal/semver (no duplicate logic).
	if !semver.IsValid(req.ReleaseTag) {
		return Result{}, fmt.Errorf("%w: release tag %q is not a valid SemVer 2.0.0 version", ErrInvalidVersion, req.ReleaseTag)
	}

	rctx, err := p.Builder.Build(ctx, req)
	if err != nil {
		return Result{}, fmt.Errorf("build release context: %w", err)
	}
	res := Result{ContextID: rctx.ContextID, Repository: rctx.Repository.FullName, Release: rctx.Release.Tag}

	// Stage 2 — engineering analysis. Extract the structured reasoning and attach
	// it to the context so generation is grounded in it. A failure here is
	// non-fatal by design: the run degrades to the factual context rather than
	// stopping, preserving the "always produce something" contract.
	if p.Analyzer != nil {
		ec, aerr := p.Analyzer.Analyze(ctx, rctx)
		if aerr != nil {
			p.log("engineering analysis failed; generating on factual context",
				slog.String("release", res.Release), slog.String("error", aerr.Error()))
		} else {
			rctx.Engineering = ec
			p.log("engineering analysis attached", slog.String("release", res.Release),
				slog.Bool("empty", ec.IsZero()))
		}
	}

	assets, genErrs := p.generate(ctx, rctx)
	res.Generated = len(assets)
	if len(assets) == 0 {
		p.notify(ctx, res, false, genErrs)
		return res, fmt.Errorf("no content generated: %s", strings.Join(genErrs, "; "))
	}

	passed, issues := p.Reviewer.Review(ctx, assets)
	res.Rejected = len(assets) - len(passed)
	res.Issues = append(issues, genErrs...)
	if len(passed) == 0 {
		p.notify(ctx, res, false, res.Issues)
		return res, fmt.Errorf("all %d generated assets failed review", len(assets))
	}

	if err := p.Publisher.Publish(ctx, rctx.Repository.FullName, passed); err != nil {
		p.notify(ctx, res, false, append(res.Issues, err.Error()))
		return res, fmt.Errorf("publish: %w", err)
	}
	res.Published = len(passed)

	p.log("release content published",
		slog.String("repository", res.Repository), slog.String("release", res.Release),
		slog.Int("generated", res.Generated), slog.Int("published", res.Published),
		slog.Int("rejected", res.Rejected), slog.Duration("duration", time.Since(start)))
	p.notify(ctx, res, true, res.Issues)
	return res, nil
}

// generate produces every configured format as generation.Content, using the
// dedicated long-form Blog path for the blog format. A single format failing is
// collected, not fatal, so the rest still publish.
func (p *Pipeline) generate(ctx context.Context, rctx *rc.ReleaseContext) ([]generation.Content, []string) {
	if p.Suite != nil {
		return p.generateSuite(ctx, rctx)
	}
	formats := p.Formats
	if len(formats) == 0 {
		formats = defaultFormats
	}
	var out []generation.Content
	var errs []string
	for _, f := range formats {
		if f == releasegen.FormatBlog {
			post, err := p.Generator.Blog(ctx, rctx)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", f, err))
				continue
			}
			out = append(out, generation.Content{Kind: generation.Kind(f), Markdown: post.Markdown})
			continue
		}
		asset, err := p.Generator.Generate(ctx, f, rctx)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", f, err))
			continue
		}
		out = append(out, generation.Content{Kind: generation.Kind(f), Markdown: asset.Body})
	}
	return out, errs
}

// generateSuite runs the full content suite and adapts each produced artifact
// into generation.Content for the shared review/publish stages. Skipped and
// failed stages are surfaced as non-fatal issues (recorded in the Result), so a
// thin release still publishes the artifacts it could produce.
func (p *Pipeline) generateSuite(ctx context.Context, rctx *rc.ReleaseContext) ([]generation.Content, []string) {
	suite := p.Suite.Run(ctx, rctx, nil)
	out := make([]generation.Content, 0, len(suite.Artifacts()))
	for _, a := range suite.Artifacts() {
		out = append(out, generation.Content{Kind: generation.Kind(a.Kind), Markdown: a.Markdown})
	}
	var issues []string
	for _, st := range suite.Manifest.Stages {
		if st.Status == contentsuite.StageOK {
			continue
		}
		msg := fmt.Sprintf("%s (M%d): %s", st.Name, st.Milestone, st.Status)
		if st.Error != "" {
			msg += " — " + st.Error
		}
		issues = append(issues, msg)
	}
	if p.Logger != nil {
		p.Logger.Info("content suite generated",
			slog.String("release", rctx.Release.Tag),
			slog.Int("produced", suite.Manifest.Produced),
			slog.Int("skipped", suite.Manifest.Skipped),
			slog.Int("failed", suite.Manifest.Failed))
	}
	return out, issues
}

func (p *Pipeline) notify(ctx context.Context, res Result, ok bool, issues []string) {
	if p.Notifier == nil {
		return
	}
	subject := fmt.Sprintf("Release content published: %s %s", res.Repository, res.Release)
	if !ok {
		subject = fmt.Sprintf("Release content FAILED: %s %s", res.Repository, res.Release)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\nRelease: %s\nContext: %s\n", res.Repository, res.Release, res.ContextID)
	fmt.Fprintf(&b, "Generated: %d, Published: %d, Rejected: %d\n", res.Generated, res.Published, res.Rejected)
	if len(issues) > 0 {
		fmt.Fprintf(&b, "\nIssues:\n- %s\n", strings.Join(issues, "\n- "))
	}
	if err := p.Notifier.Notify(ctx, subject, b.String()); err != nil && p.Logger != nil {
		p.Logger.Warn("notify failed", slog.String("error", err.Error()))
	}
}

func (p *Pipeline) log(msg string, args ...any) {
	if p.Logger != nil {
		p.Logger.Info(msg, args...)
	}
}
