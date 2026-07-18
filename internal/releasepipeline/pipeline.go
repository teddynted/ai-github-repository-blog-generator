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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// ContextBuilder builds a Release Context (satisfied by *releasecontext.Builder).
type ContextBuilder interface {
	Build(ctx context.Context, req rc.Request) (*rc.ReleaseContext, error)
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

// Pipeline orchestrates release → context → content → review → publish → notify.
type Pipeline struct {
	Builder   ContextBuilder
	Generator Generator
	// Formats to generate; defaults to blog + release summary when empty.
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
	if p.Builder == nil || p.Generator == nil || p.Reviewer == nil || p.Publisher == nil {
		return Result{}, fmt.Errorf("release pipeline is missing a required stage")
	}

	rctx, err := p.Builder.Build(ctx, req)
	if err != nil {
		return Result{}, fmt.Errorf("build release context: %w", err)
	}
	res := Result{ContextID: rctx.ContextID, Repository: rctx.Repository.FullName, Release: rctx.Release.Tag}

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
