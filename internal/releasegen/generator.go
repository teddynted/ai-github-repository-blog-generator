// Package releasegen is the Content Generation engine (Milestone 3). It turns a
// releasecontext.ReleaseContext — the canonical, AI-ready analysis of a
// repository at one release — into publication-ready content across formats
// (blog post, release summary, docs, LinkedIn, YouTube Shorts, TikTok, SEO).
//
// It depends only on a Model port (local inference today via Ollama, Amazon
// Bedrock later), so generation is decoupled from any specific LLM and every
// prompt is grounded strictly in the provided context.
package releasegen

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// DefaultMaxPromptBytes bounds the grounding block so a prompt fits the model.
const DefaultMaxPromptBytes = 24000

const truncationMarker = "\n\n[context truncated to fit the model budget]\n"

// Model is the inference port. *ollama.Client satisfies it; a Bedrock adapter
// will too.
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Format identifies a content output type.
type Format string

const (
	FormatBlog           Format = "blog"
	FormatReleaseSummary Format = "release-summary"
	FormatDocumentation  Format = "documentation"
)

// AllFormats is the default set produced by GenerateAll when none are given.
//
// releasegen deliberately covers only the long-form written formats (blog,
// release summary, documentation). The social/video/SEO artifacts (LinkedIn,
// YouTube Shorts, TikTok, SEO metadata) have dedicated, richer generators in
// internal/linkedin, internal/shorts, internal/tiktok and internal/seo — those
// are the single source of truth, orchestrated by internal/contentsuite. Do not
// re-add prompt-only versions of them here (that was the M21 QA "H2" duplication).
var AllFormats = []Format{
	FormatBlog, FormatReleaseSummary, FormatDocumentation,
}

// Asset is one generated piece of content.
type Asset struct {
	Format Format `json:"format"`
	Title  string `json:"title,omitempty"`
	Body   string `json:"body"`
}

// Generator produces content from a ReleaseContext via the Model.
type Generator struct {
	Model          Model
	MaxPromptBytes int
	Logger         *slog.Logger
}

// spec is the varying part of a per-format prompt.
type spec struct {
	role        string
	instruction string
	closing     string
}

var specs = map[Format]spec{
	FormatBlog: {
		role:        "You are a senior software engineer writing a technical blog post.",
		instruction: "Write a clear, accurate, publication-ready technical blog post in GitHub-flavoured Markdown about this software release. Follow the suggested outline where useful. Explain what changed, how it works, and why it matters.",
		closing:     "Produce only the Markdown blog post, starting with a single top-level title.",
	},
	FormatReleaseSummary: {
		role:        "You are preparing a concise GitHub Release summary.",
		instruction: "Write a concise, scannable release summary in GitHub-flavoured Markdown, grouping changes into Features, Fixes, and Breaking Changes where present.",
		closing:     "Produce only the Markdown release summary.",
	},
	FormatDocumentation: {
		role:        "You are a technical writer updating project documentation.",
		instruction: "Write or update documentation in GitHub-flavoured Markdown that reflects the changes in this release. Focus on new capabilities and how to use them.",
		closing:     "Produce a Markdown documentation page.",
	},
}

// Generate produces a single asset of the given format from the context.
func (g *Generator) Generate(ctx context.Context, format Format, rctx *rc.ReleaseContext) (Asset, error) {
	sp, ok := specs[format]
	if !ok {
		return Asset{}, fmt.Errorf("unknown content format %q", format)
	}
	if g.Model == nil {
		return Asset{}, fmt.Errorf("generator has no model configured")
	}
	prompt := g.buildPrompt(sp, rctx)
	out, err := g.Model.Generate(ctx, prompt)
	if err != nil {
		return Asset{}, fmt.Errorf("generate %s: %w", format, err)
	}
	asset := Asset{Format: format, Title: suggestTitle(format, rctx), Body: strings.TrimSpace(out)}
	if g.Logger != nil {
		g.Logger.Info("content generated",
			slog.String("format", string(format)),
			slog.String("repository", rctx.Repository.FullName),
			slog.String("release", rctx.Release.Tag),
			slog.Int("bytes", len(asset.Body)),
		)
	}
	return asset, nil
}

// GenerateAll produces every requested format (or AllFormats when none given).
// It attempts every format and aggregates failures, so one bad format never
// discards the assets that succeeded.
func (g *Generator) GenerateAll(ctx context.Context, rctx *rc.ReleaseContext, formats ...Format) ([]Asset, error) {
	if len(formats) == 0 {
		formats = AllFormats
	}
	var assets []Asset
	var errs []string
	for _, f := range formats {
		a, err := g.Generate(ctx, f, rctx)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		assets = append(assets, a)
	}
	if len(errs) > 0 {
		return assets, fmt.Errorf("%d of %d formats failed: %s", len(errs), len(formats), strings.Join(errs, "; "))
	}
	return assets, nil
}

func (g *Generator) buildPrompt(sp spec, rctx *rc.ReleaseContext) string {
	max := g.MaxPromptBytes
	if max <= 0 {
		max = DefaultMaxPromptBytes
	}
	ground := safeTruncate(contextBlock(rctx), max)
	var b strings.Builder
	b.WriteString(sp.role)
	b.WriteString("\n\n")
	b.WriteString(sp.instruction)
	b.WriteString("\n\nGround every statement strictly in the Release Context below. Do not invent facts, versions, or features that are not present.\n\n")
	b.WriteString("=== RELEASE CONTEXT ===\n")
	b.WriteString(ground)
	b.WriteString("\n=== END RELEASE CONTEXT ===\n\n")
	b.WriteString(sp.closing)
	return b.String()
}

// suggestTitle offers a title for formats that benefit from one, reusing the
// content-intelligence blog titles the context already computed.
func suggestTitle(format Format, rctx *rc.ReleaseContext) string {
	switch format {
	case FormatBlog:
		if len(rctx.ContentIntelligence.BlogTitles) > 0 {
			return rctx.ContentIntelligence.BlogTitles[0]
		}
	case FormatReleaseSummary:
		if rctx.Release.Name != "" {
			return rctx.Release.Name
		}
		return rctx.Release.Tag
	}
	return ""
}

func safeTruncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= len(truncationMarker) {
		return s[:max]
	}
	return s[:max-len(truncationMarker)] + truncationMarker
}
