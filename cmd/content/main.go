// Command content is the local content-development tool. It generates any
// supported artifact from a Release Context fixture using a selectable provider
// (Anthropic, Bedrock, or Ollama), writes the result to an output directory, and
// validates it — all with no GitHub, EC2, EventBridge, SQS, or S3 involvement.
// It mirrors the production pipeline by reusing internal/contentsuite and the
// same generators, so what you iterate on locally is what production runs.
//
// Usage:
//
//	go run ./cmd/content --artifact blog --provider anthropic --context fixtures/v0.3.0.json --output output/
//	go run ./cmd/content --artifact all  --provider ollama    --context fixtures/v0.3.0.json
//	go run ./cmd/content --artifact blog --provider anthropic --context fixtures/v0.3.0.json --dry-run
//	go run ./cmd/content playground
//
// Response caching (.cache/) makes iteration cheap; --no-cache bypasses it.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/aicache"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/airouter"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/anthropic"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/bedrockclaude"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentcheck"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentsuite"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/promptversion"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// Model is the inference port shared by every provider.
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// supportedArtifacts is the full local artifact set (kind → default extension).
var supportedArtifacts = []string{
	"blog", "architecture", "architecture-diagram-spec", "linkedin", "x-thread",
	"storyboard", "voiceover", "youtube", "youtube-shorts", "tiktok",
	"visual-assets", "seo-metadata",
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "playground":
			return playground(args[1:])
		case "validate":
			return validateCmd(args[1:])
		}
	}
	return generate(args)
}

// validateCmd validates already-generated artifacts on disk (no regeneration),
// inferring each artifact's kind from its filename.
func validateCmd(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	outDir := fs.String("output", "output", "directory of generated artifacts to validate (e.g. output/releases/v0.3.0)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	entries, err := os.ReadDir(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", *outDir, err)
		return 1
	}
	failed, n := false, 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		kind := strings.TrimSuffix(name, filepath.Ext(name))
		data, err := os.ReadFile(filepath.Join(*outDir, name))
		if err != nil {
			continue
		}
		r := contentcheck.Validate(kind, string(data))
		fmt.Print(r.String())
		if !r.OK() {
			failed = true
		}
		n++
	}
	if n == 0 {
		fmt.Fprintf(os.Stderr, "no artifacts to validate in %s/\n", *outDir)
		return 1
	}
	if failed {
		return 1
	}
	return 0
}

// options holds the parsed CLI configuration for a generate run.
type options struct {
	artifact, provider, ctxPath, outDir, system, model, ollamaURL, region string
	fromBlog                                                              string
	temperature                                                           float64
	maxTokens                                                             int
	dryRun, verbose, noCache, noHistory, hybrid                           bool
	repoLevel                                                             bool
	repo                                                                  string
	cacheDir                                                              string
	timeout                                                               time.Duration
}

func generate(args []string) int {
	fs := flag.NewFlagSet("content", flag.ContinueOnError)
	o := options{}
	fs.StringVar(&o.artifact, "artifact", "all", "artifact to generate (blog, architecture, …, or 'all')")
	fs.StringVar(&o.provider, "provider", "ollama", "provider: anthropic | bedrock | ollama | claude-code (local: uses your Claude Code subscription via `claude -p`)")
	fs.StringVar(&o.ctxPath, "context", "", "path to a Release Context fixture JSON (required)")
	fs.StringVar(&o.outDir, "output", "output", "output directory")
	fs.Float64Var(&o.temperature, "temperature", 0, "sampling temperature (Anthropic/Bedrock)")
	fs.IntVar(&o.maxTokens, "max-tokens", 0, "max output tokens (Anthropic/Bedrock; 0 = provider default)")
	fs.StringVar(&o.system, "system", "", "system prompt override (Anthropic/Bedrock)")
	fs.BoolVar(&o.dryRun, "dry-run", false, "print the prompt(s) that would be sent; do not call a provider or write output")
	fs.BoolVar(&o.verbose, "verbose", false, "verbose logging")
	fs.BoolVar(&o.noCache, "no-cache", false, "bypass the local response cache")
	fs.BoolVar(&o.noHistory, "no-history", false, "do not archive this run under <output>/releases/<version>/history/<stamp>/")
	fs.BoolVar(&o.hybrid, "hybrid", false, "hybrid routing: premium kinds (blog, architecture, linkedin, x-thread, architecture-diagram-spec) via claude-code, the rest via Ollama (--model / OLLAMA_MODEL, default llama3.2:1b). Best with --artifact all")
	fs.StringVar(&o.fromBlog, "from-blog", "", "generate downstream artifacts from an EXISTING blog.md (skip blog regeneration). Requires --artifact != blog; pair with --artifact all or a specific downstream kind")
	fs.StringVar(&o.cacheDir, "cache-dir", ".cache", "response cache directory")
	fs.StringVar(&o.model, "model", "", "model id/name (provider default when empty)")
	fs.StringVar(&o.ollamaURL, "ollama-url", envOr("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama base URL")
	fs.StringVar(&o.region, "region", envOr("AWS_REGION", "us-east-1"), "AWS region (Bedrock)")
	fs.DurationVar(&o.timeout, "timeout", 15*time.Minute, "overall timeout")
	fs.BoolVar(&o.repoLevel, "repo-level", false, "generate a version-independent, repository-level docs/architecture.md — from the LOCAL working tree (--repo, default) or an existing Release Context fixture (--context); the release version is ignored, never embedded. Deterministic and offline")
	fs.StringVar(&o.repo, "repo", ".", "repository root to read for --repo-level")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// --repo-level builds its own context from the working tree and writes a single
	// version-independent doc — it doesn't use --context or the artifact suite.
	if o.repoLevel {
		return repoLevelRun(o)
	}

	if o.ctxPath == "" {
		fmt.Fprintln(os.Stderr, "error: --context is required (e.g. --context fixtures/v0.3.0.json)")
		return 2
	}
	if o.artifact != "all" && !contains(supportedArtifacts, o.artifact) {
		fmt.Fprintf(os.Stderr, "error: unknown --artifact %q; supported: %s, all\n", o.artifact, strings.Join(supportedArtifacts, ", "))
		return 2
	}

	rctx, err := loadContext(o.ctxPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	return execute(ctx, o, rctx)
}

// repoLevelRun writes a version-independent, repository-level docs/architecture.md.
// Its context comes from the local working tree (default) or, when --context is
// given, from an existing Release Context fixture — in which case the release's
// version/tag is simply ignored by the repo-level renderer (the context itself is
// never mutated). Deterministic, offline, no provider involved.
func repoLevelRun(o options) int {
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	var rctx *rc.ReleaseContext
	var err error
	if o.ctxPath != "" {
		rctx, err = loadContext(o.ctxPath) // render a version-independent doc from a fixture
	} else {
		rctx, err = rc.BuildRepoLevel(ctx, o.repo, "")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: build repo-level context: %v\n", err)
		return 1
	}
	col, err := (&architecture.Generator{Logger: suiteLogger()}).Architecture(ctx, architecture.ReleasePackage{Context: rctx})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: architecture: %v\n", err)
		return 1
	}
	target := repoLevelTarget(o)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if err := os.WriteFile(target, []byte(col.RepoLevelMarkdown()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write %s: %v\n", target, err)
		return 1
	}
	fmt.Printf("wrote %s (%d diagrams, version-independent)\n", target, len(col.Diagrams))
	return 0
}

// repoLevelTarget resolves the --repo-level output path: <repo>/docs/architecture.md
// by default; an --output ending in .md is used verbatim, else treated as a directory.
func repoLevelTarget(o options) string {
	if o.outDir != "" && o.outDir != "output" {
		if strings.HasSuffix(o.outDir, ".md") {
			return o.outDir
		}
		return filepath.Join(o.outDir, "architecture.md")
	}
	return filepath.Join(o.repo, "docs", "architecture.md")
}

func execute(ctx context.Context, o options, rctx *rc.ReleaseContext) int {
	start := time.Now()

	// Build the model. In dry-run we swap in a capturing model so the exact
	// prompts flow through the real generators without calling any provider.
	// In hybrid mode we build a per-kind router (premium via claude-code, the
	// rest via Ollama) so a single `--artifact all` run mixes both providers —
	// the same policy the production worker uses, kept in the Go generators.
	var model Model
	var modelFor func(string) releasegen.Model
	var cache *aicache.Cache
	dry := &captureModel{}
	switch {
	case o.dryRun:
		model = dry
		modelFor = func(string) releasegen.Model { return dry }
		if o.hybrid {
			om := o.model
			if om == "" {
				om = envOr("OLLAMA_MODEL", "llama3.2:1b")
			}
			printHybridPolicy(om)
		}
	case o.hybrid:
		m, mf, err := buildHybrid(o)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: build hybrid: %v\n", err)
			return 1
		}
		model, modelFor = m, mf
	default:
		m, fp, err := buildModel(ctx, o)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: build %s provider: %v\n", o.provider, err)
			return 1
		}
		model = m
		if !o.noCache {
			cache = aicache.New(m, o.cacheDir, fp...)
			model = cache
		}
		modelFor = func(string) releasegen.Model { return model }
	}

	// --from-blog: derive downstream artifacts from an existing blog.md instead
	// of regenerating the (expensive) foundation article.
	var preBlog *releasegen.BlogPost
	if o.fromBlog != "" {
		if o.artifact == "blog" {
			fmt.Fprintln(os.Stderr, "error: --from-blog cannot be used with --artifact blog (nothing to derive)")
			return 1
		}
		bp, err := loadBlogPost(o.fromBlog)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		preBlog = &bp
		fmt.Fprintf(os.Stderr, "using existing blog (%s, %d words) — skipping blog regeneration\n", o.fromBlog, bp.WordCount)
	}

	artifacts, err := produce(ctx, o.artifact, rctx, model, modelFor, preBlog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: generate %s: %v\n", o.artifact, err)
		return 1
	}

	if o.dryRun {
		fmt.Printf("DRY RUN — %d prompt(s) that would be sent to %s:\n\n", len(dry.prompts), o.provider)
		for i, p := range dry.prompts {
			fmt.Printf("──────── prompt %d (%d chars, ~%d input tokens) ────────\n%s\n\n", i+1, len(p), estTokens(p), p)
		}
		return 0
	}

	// Per-release layout: every release version gets its own folder under
	// <output>/releases/<version>/, holding the latest <kind>.<ext> plus a
	// history/<stamp>/ archive — so drafts for different releases never mix.
	releaseDir := filepath.Join(o.outDir, "releases", releaseSlug(rctx.Release.Tag))
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: create output dir: %v\n", err)
		return 1
	}

	// Local versioning: keep <kind>.<ext> as the latest for convenience, and
	// archive every run under <version>/history/<stamp>/ so no draft is lost
	// between iterations (the local counterpart to S3 object versioning).
	histDir := ""
	if !o.noHistory {
		histDir = filepath.Join(releaseDir, "history", runStamp())
		if err := os.MkdirAll(histDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "error: create history dir: %v\n", err)
			return 1
		}
	}

	failed := false
	for _, a := range artifacts {
		filename := a.kind + "." + a.ext
		if err := os.WriteFile(filepath.Join(releaseDir, filename), []byte(a.markdown), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", filename, err)
			return 1
		}
		if histDir != "" {
			if err := os.WriteFile(filepath.Join(histDir, filename), []byte(a.markdown), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error: archive %s: %v\n", filename, err)
				return 1
			}
		}
		report := contentcheck.Validate(a.kind, a.markdown)
		if !report.OK() {
			failed = true
		}
		if o.verbose || !report.OK() {
			fmt.Print(report.String())
		}
	}

	logRun(o, artifacts, cache, releaseDir, histDir, time.Since(start))
	if failed {
		return 1
	}
	return 0
}

// artifact is one produced output.
type artifact struct {
	kind, ext, markdown string
}

// produce generates the requested artifact(s). "blog" is generated standalone
// (the common prompt-iteration path — one model, no downstream chain); anything
// else runs the full suite (which resolves inter-artifact dependencies) and
// selects the requested output(s).
func produce(ctx context.Context, target string, rctx *rc.ReleaseContext, model Model, modelFor func(string) releasegen.Model, blog *releasegen.BlogPost) ([]artifact, error) {
	if target == "blog" {
		post, err := (&releasegen.Generator{Model: modelFor("blog")}).Blog(ctx, rctx)
		if err != nil {
			return nil, err
		}
		return []artifact{{kind: "blog", ext: "md", markdown: post.Markdown}}, nil
	}

	// blog is nil → the orchestrator generates it first; non-nil (--from-blog) →
	// it uses the supplied blog verbatim and only runs the downstream stages.
	orch := &contentsuite.Orchestrator{Model: model, ModelFor: modelFor, Logger: suiteLogger()}
	// A single --artifact runs only that stage plus its real dependencies (not the
	// whole suite); --artifact all runs everything.
	if target != "all" {
		orch.Only = map[string]bool{target: true}
	}
	suite := orch.Run(ctx, rctx, blog)
	var out []artifact
	for _, a := range suite.Artifacts() {
		if target != "all" && a.Kind != target {
			continue
		}
		out = append(out, artifact{kind: a.Kind, ext: extOr(a.Ext), markdown: a.Markdown})
	}
	if len(out) == 0 && target != "all" {
		return nil, fmt.Errorf("artifact %q was not produced (a thin release may skip it)", target)
	}
	return out, nil
}

// loadBlogPost reads an existing blog.md into a BlogPost so the suite can derive
// downstream artifacts from it without regenerating the blog. The whole file is
// the Markdown; the leading YAML front matter (title/description/tags) is parsed
// for the generators that use those fields.
func loadBlogPost(path string) (releasegen.BlogPost, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return releasegen.BlogPost{}, fmt.Errorf("read --from-blog %s: %w", path, err)
	}
	md := string(b)
	if strings.TrimSpace(md) == "" {
		return releasegen.BlogPost{}, fmt.Errorf("--from-blog %s is empty", path)
	}
	bp := releasegen.BlogPost{Markdown: md, WordCount: len(strings.Fields(md))}
	if rest, ok := strings.CutPrefix(md, "---\n"); ok {
		if i := strings.Index(rest, "\n---"); i >= 0 {
			for _, line := range strings.Split(rest[:i], "\n") {
				k, v, found := strings.Cut(line, ":")
				if !found {
					continue
				}
				v = strings.TrimSpace(v)
				switch strings.TrimSpace(k) {
				case "title":
					bp.Title = strings.Trim(v, `"`)
				case "description":
					bp.MetaDescription = strings.Trim(v, `"`)
				case "tags":
					for _, t := range strings.Split(strings.Trim(v, "[]"), ",") {
						if t = strings.TrimSpace(t); t != "" {
							bp.Tags = append(bp.Tags, t)
						}
					}
				}
			}
		}
	}
	return bp, nil
}

func logRun(o options, arts []artifact, cache *aicache.Cache, releaseDir, histDir string, dur time.Duration) {
	fmt.Fprintf(os.Stderr, "\n──────── run summary ────────\n")
	fmt.Fprintf(os.Stderr, "provider:   %s%s\n", o.provider, modelSuffix(o))
	fmt.Fprintf(os.Stderr, "artifact:   %s (%d written)\n", o.artifact, len(arts))
	fmt.Fprintf(os.Stderr, "duration:   %s\n", dur.Round(time.Millisecond))
	if cache != nil {
		h, m := cache.Stats()
		fmt.Fprintf(os.Stderr, "cache:      %d hit / %d miss\n", h, m)
	} else {
		fmt.Fprintf(os.Stderr, "cache:      disabled\n")
	}
	var totalOut int
	for _, a := range arts {
		totalOut += estTokens(a.markdown)
	}
	fmt.Fprintf(os.Stderr, "est tokens: ~%d output\n", totalOut)
	fmt.Fprintf(os.Stderr, "output:     %s/\n", releaseDir)
	if histDir != "" {
		fmt.Fprintf(os.Stderr, "versioned:  %s/\n", histDir)
	}
	prompts := make([]string, 0, len(arts))
	for _, a := range arts {
		prompts = append(prompts, a.kind+"="+promptversion.For(a.kind))
	}
	fmt.Fprintf(os.Stderr, "prompts:    %s\n", strings.Join(prompts, " "))
}

// captureModel records the prompts it is asked to generate and returns a stub,
// so --dry-run reveals exactly what each generator would send.
type captureModel struct{ prompts []string }

func (m *captureModel) Generate(_ context.Context, prompt string) (string, error) {
	m.prompts = append(m.prompts, prompt)
	return "## Introduction\n\nDRY-RUN STUB.\n\n## Conclusion\n\nDRY-RUN STUB.", nil
}

// ollamaOpts builds the shared Ollama client options. --max-tokens, when set,
// overrides the client's default num_predict cap; otherwise the default cap
// (which keeps small models from rambling) applies.
func ollamaOpts(o options) []ollama.Option {
	opts := []ollama.Option{ollama.WithBaseURL(o.ollamaURL)}
	if o.maxTokens > 0 {
		opts = append(opts, ollama.WithNumPredict(o.maxTokens))
	}
	return opts
}

func buildModel(ctx context.Context, o options) (Model, []string, error) {
	switch o.provider {
	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
		}
		model := o.model
		if model == "" {
			model = anthropic.DefaultModel
		}
		cl, err := anthropic.New(anthropic.Config{APIKey: key, Model: model, MaxTokens: o.maxTokens, Temperature: o.temperature, System: o.system})
		if err != nil {
			return nil, nil, err
		}
		return cl, fingerprint("anthropic", model, o.system, o.temperature, o.maxTokens), nil
	case "bedrock":
		if o.model == "" {
			return nil, nil, fmt.Errorf("--model (a Bedrock model id) is required for --provider bedrock")
		}
		cl, err := bedrockclaude.NewFromAWS(ctx, o.region, bedrockclaude.Config{ModelID: o.model, MaxTokens: o.maxTokens, Temperature: o.temperature, System: o.system})
		if err != nil {
			return nil, nil, err
		}
		return cl, fingerprint("bedrock", o.model, o.system, o.temperature, o.maxTokens), nil
	case "ollama":
		model := o.model
		if model == "" {
			model = envOr("OLLAMA_MODEL", "qwen2.5:7b")
		}
		return ollama.New(model, ollamaOpts(o)...), fingerprint("ollama", model, o.system, o.temperature, o.maxTokens), nil
	case "claude-code":
		// Local-dev only: generate via the Claude Code subscription (`claude -p`)
		// instead of Anthropic API credits.
		m, err := newClaudeCodeModel()
		if err != nil {
			return nil, nil, err
		}
		return m, fingerprint("claude-code", "subscription", o.system, o.temperature, o.maxTokens), nil
	default:
		return nil, nil, fmt.Errorf("unknown provider %q (want anthropic, bedrock, ollama, or claude-code)", o.provider)
	}
}

// buildHybrid builds a per-kind router for local hybrid generation: premium
// kinds (blog, architecture, linkedin, x-thread, architecture-diagram-spec) go
// to claude-code, everything else to Ollama. It mirrors the production worker's
// airouter policy (internal/airouter.DefaultRules) so a local `--artifact all`
// run exercises the same routing, with every prompt still owned by the Go
// generators. The returned base Model (Ollama) drives the orchestrator's cheap
// analysis stages; modelFor routes each artifact to its provider.
func buildHybrid(o options) (Model, func(string) releasegen.Model, error) {
	premium, err := newClaudeCodeModel()
	if err != nil {
		return nil, nil, fmt.Errorf("premium provider (claude-code): %w", err)
	}
	ollamaModel := o.model
	if ollamaModel == "" {
		ollamaModel = envOr("OLLAMA_MODEL", "llama3.2:1b")
	}
	var claudeM, ollamaM Model = premium, ollama.New(ollamaModel, ollamaOpts(o)...)
	if !o.noCache {
		claudeM = aicache.New(claudeM, o.cacheDir, fingerprint("claude-code", "subscription", o.system, o.temperature, o.maxTokens)...)
		ollamaM = aicache.New(ollamaM, o.cacheDir, fingerprint("ollama", ollamaModel, o.system, o.temperature, o.maxTokens)...)
	}

	providers := map[string]airouter.Model{
		airouter.ProviderClaude: claudeM,
		airouter.ProviderOllama: ollamaM,
	}
	// A stderr logger surfaces each routing decision live, so a long run shows
	// per-artifact progress ("ai routing decision content_type=blog provider=claude")
	// as it happens rather than going silent until the final summary.
	logger := suiteLogger()
	// Wrap each provider so every generation logs start + finish with elapsed
	// time — otherwise a long run goes silent for minutes between routing lines.
	claudeM = loggingModel{inner: claudeM, label: "claude-code", logger: logger}
	ollamaM = loggingModel{inner: ollamaM, label: "ollama:" + ollamaModel, logger: logger}
	providers[airouter.ProviderClaude] = claudeM
	providers[airouter.ProviderOllama] = ollamaM

	router := airouter.New(providers, airouter.DefaultRules(), airouter.ProviderOllama, airouter.ProviderOllama, logger)
	printHybridPolicy(ollamaModel)
	return ollamaM, func(kind string) releasegen.Model { return router.ModelFor(kind) }, nil
}

// suiteLogger returns the stderr logger used for live run progress (routing
// decisions, stage selection, per-generation timing).
func suiteLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

// loggingModel wraps a Model to log each Generate call's start and finish with
// elapsed time and I/O sizes, so a long hybrid run shows continuous progress
// (per artifact, and per retry) on stderr instead of long silent gaps.
type loggingModel struct {
	inner  Model
	label  string
	logger *slog.Logger
}

func (m loggingModel) Generate(ctx context.Context, prompt string) (string, error) {
	start := time.Now()
	m.logger.Info("generating", "provider", m.label, "prompt_chars", len(prompt))

	// Heartbeat: a single model call is one blocking request with nothing to log
	// mid-flight, so tick every 15s to show the call is still alive during long
	// (minutes-scale) generations.
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				m.logger.Info("… still generating", "provider", m.label,
					"elapsed", time.Since(start).Round(time.Second).String())
			}
		}
	}()

	out, err := m.inner.Generate(ctx, prompt)
	close(done)
	if err != nil {
		m.logger.Warn("generation failed", "provider", m.label,
			"elapsed", time.Since(start).Round(time.Second).String(), "error", err.Error())
		return out, err
	}
	m.logger.Info("generated", "provider", m.label,
		"elapsed", time.Since(start).Round(time.Second).String(), "out_chars", len(out))
	return out, nil
}

// printHybridPolicy writes the per-kind provider routing to stderr so a hybrid
// run (or a --hybrid --dry-run preview) is self-documenting.
func printHybridPolicy(ollamaModel string) {
	rules := airouter.DefaultRules()
	kinds := append([]string{"blog", "architecture", "architecture-diagram-spec", "linkedin", "x-thread"}, airouter.AllKinds...)
	seen := map[string]bool{}
	fmt.Fprintf(os.Stderr, "hybrid routing — premium=claude-code, transforms=ollama:%s\n", ollamaModel)
	for _, k := range kinds {
		if seen[k] {
			continue
		}
		seen[k] = true
		p := rules[k]
		if p == "" {
			p = airouter.ProviderOllama
		}
		fmt.Fprintf(os.Stderr, "  %-26s → %s\n", k, p)
	}
}

func fingerprint(provider, model, system string, temp float64, maxTokens int) []string {
	return []string{provider, model, system, strconv.FormatFloat(temp, 'f', -1, 64), strconv.Itoa(maxTokens)}
}

func loadContext(path string) (*rc.ReleaseContext, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read context: %w", err)
	}
	var rctx rc.ReleaseContext
	if err := json.Unmarshal(raw, &rctx); err != nil {
		return nil, fmt.Errorf("parse Release Context JSON: %w", err)
	}
	return &rctx, nil
}

func modelSuffix(o options) string {
	if o.model != "" {
		return " (" + o.model + ")"
	}
	return ""
}

func extOr(ext string) string {
	if ext == "" {
		return "md"
	}
	return ext
}

// runStamp is a sortable, unique archive folder name: a UTC timestamp plus a
// little entropy so two runs in the same second do not collide.
func runStamp() string {
	var r [2]byte
	_, _ = rand.Read(r[:])
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(r[:])
}

// releaseSlug turns a release tag (e.g. "v0.3.0") into a safe folder name so
// each version's output lives under its own directory. Any character that is
// not alphanumeric, dot, underscore, or dash is replaced with a dash; an empty
// or degenerate tag falls back to "unversioned".
func releaseSlug(tag string) string {
	tag = strings.TrimSpace(tag)
	var b strings.Builder
	for _, r := range tag {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	slug := strings.Trim(b.String(), "-.")
	if slug == "" {
		return "unversioned"
	}
	return slug
}

func estTokens(s string) int { return len(s) / 4 } // rough 4-chars-per-token estimate

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
