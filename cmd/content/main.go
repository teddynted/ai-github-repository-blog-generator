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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/aicache"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/anthropic"
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
	"blog", "architecture", "linkedin", "x-thread", "storyboard", "voiceover",
	"youtube", "youtube-shorts", "tiktok", "visual-assets", "seo-metadata",
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
	outDir := fs.String("output", "output", "directory of generated artifacts to validate")
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
	temperature                                                           float64
	maxTokens                                                             int
	dryRun, verbose, noCache                                              bool
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
	fs.StringVar(&o.cacheDir, "cache-dir", ".cache", "response cache directory")
	fs.StringVar(&o.model, "model", "", "model id/name (provider default when empty)")
	fs.StringVar(&o.ollamaURL, "ollama-url", envOr("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama base URL")
	fs.StringVar(&o.region, "region", envOr("AWS_REGION", "us-east-1"), "AWS region (Bedrock)")
	fs.DurationVar(&o.timeout, "timeout", 15*time.Minute, "overall timeout")
	if err := fs.Parse(args); err != nil {
		return 2
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

func execute(ctx context.Context, o options, rctx *rc.ReleaseContext) int {
	start := time.Now()

	// Build the model. In dry-run we swap in a capturing model so the exact
	// prompts flow through the real generators without calling any provider.
	var model Model
	var cache *aicache.Cache
	dry := &captureModel{}
	if o.dryRun {
		model = dry
	} else {
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
	}

	artifacts, err := produce(ctx, o.artifact, rctx, model)
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

	if err := os.MkdirAll(o.outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: create output dir: %v\n", err)
		return 1
	}

	failed := false
	for _, a := range artifacts {
		path := filepath.Join(o.outDir, a.kind+"."+a.ext)
		if err := os.WriteFile(path, []byte(a.markdown), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", path, err)
			return 1
		}
		report := contentcheck.Validate(a.kind, a.markdown)
		if !report.OK() {
			failed = true
		}
		if o.verbose || !report.OK() {
			fmt.Print(report.String())
		}
	}

	logRun(o, artifacts, cache, time.Since(start))
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
func produce(ctx context.Context, target string, rctx *rc.ReleaseContext, model Model) ([]artifact, error) {
	if target == "blog" {
		post, err := (&releasegen.Generator{Model: model}).Blog(ctx, rctx)
		if err != nil {
			return nil, err
		}
		return []artifact{{kind: "blog", ext: "md", markdown: post.Markdown}}, nil
	}

	orch := &contentsuite.Orchestrator{Model: model, ModelFor: func(string) releasegen.Model { return model }}
	suite := orch.Run(ctx, rctx, nil)
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

func logRun(o options, arts []artifact, cache *aicache.Cache, dur time.Duration) {
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
	fmt.Fprintf(os.Stderr, "output:     %s/\n", o.outDir)
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
		return ollama.New(model, ollama.WithBaseURL(o.ollamaURL)), fingerprint("ollama", model, o.system, o.temperature, o.maxTokens), nil
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
