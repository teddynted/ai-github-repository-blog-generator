// Command voiceover converts a Storyboard (Milestone 4) into a structured,
// synchronized voice-over script (Milestone 5) — the canonical input for future
// Text-to-Speech pipelines (Amazon Polly, ElevenLabs, OpenAI TTS, Azure Speech,
// Google Cloud TTS) and human narrators.
//
// It reads a storyboard JSON directly, or builds one on the fly from a Release
// Context (+ an optional blog), then emits the voice-over as Markdown (default)
// or JSON.
//
// Usage:
//
//	# From an existing storyboard JSON, fully offline (deterministic narration):
//	go run ./cmd/voiceover --storyboard board.json --offline
//
//	# From a Release Context + a blog file, offline end-to-end:
//	go run ./cmd/voiceover --context ctx.json --blog post.md --offline
//
//	# Generate the storyboard via Ollama first, then the voice-over as JSON:
//	go run ./cmd/voiceover --context ctx.json --format json --out script.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("voiceover", flag.ContinueOnError)
	sbPath := fs.String("storyboard", "", "path to an existing Storyboard JSON file")
	ctxPath := fs.String("context", "", "path to a Release Context JSON file (used when --storyboard is absent)")
	blogPath := fs.String("blog", "", "path to an existing blog Markdown file (else generate via Ollama)")
	format := fs.String("format", "md", "output format: md | json")
	model := fs.String("model", envOr("OLLAMA_MODEL", "qwen2.5:7b"), "Ollama model")
	ollamaURL := fs.String("ollama", envOr("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama base URL")
	offline := fs.Bool("offline", false, "do not call the model (deterministic narration)")
	outPath := fs.String("out", "", "output file (default: stdout)")
	timeout := fs.Duration("timeout", 5*time.Minute, "generation timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *sbPath == "" && *ctxPath == "" {
		fmt.Fprintln(os.Stderr, "error: one of --storyboard or --context is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var model2 releasegen.Model
	if !*offline {
		model2 = ollama.New(*model, ollama.WithBaseURL(*ollamaURL))
	}

	sb, err := resolveStoryboard(ctx, *sbPath, *ctxPath, *blogPath, model2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	gen := &voiceover.Generator{Model: model2}
	script, err := gen.VoiceOver(ctx, sb)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if probs := script.Validate(); len(probs) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d validation issue(s):\n", len(probs))
		for _, p := range probs {
			fmt.Fprintf(os.Stderr, "  - %s\n", p)
		}
	}

	var out string
	if *format == "json" {
		b, _ := json.MarshalIndent(script, "", "  ")
		out = string(b)
	} else {
		out = script.Markdown()
	}
	if err := writeOut(*outPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		return 1
	}
	if *outPath != "" {
		fmt.Fprintf(os.Stderr, "wrote %s (%d scenes, ~%s speaking)\n", *outPath, script.Metadata.SceneCount, script.ContentIntelligence.EstimatedSpeakingTime)
	}
	return 0
}

// resolveStoryboard loads a storyboard JSON, or builds one from a Release
// Context (+ optional blog), reusing the Milestone 4 generator.
func resolveStoryboard(ctx context.Context, sbPath, ctxPath, blogPath string, model releasegen.Model) (storyboard.Storyboard, error) {
	if sbPath != "" {
		data, err := readInput(sbPath)
		if err != nil {
			return storyboard.Storyboard{}, fmt.Errorf("read storyboard: %w", err)
		}
		var sb storyboard.Storyboard
		if err := json.Unmarshal(data, &sb); err != nil {
			return storyboard.Storyboard{}, fmt.Errorf("parse Storyboard JSON: %w", err)
		}
		return sb, nil
	}

	raw, err := readInput(ctxPath)
	if err != nil {
		return storyboard.Storyboard{}, fmt.Errorf("read context: %w", err)
	}
	var rctx rc.ReleaseContext
	if err := json.Unmarshal(raw, &rctx); err != nil {
		return storyboard.Storyboard{}, fmt.Errorf("parse Release Context JSON: %w", err)
	}

	post, err := resolveBlog(ctx, blogPath, &rctx, model)
	if err != nil {
		return storyboard.Storyboard{}, err
	}
	return (&storyboard.Generator{Model: model}).Storyboard(ctx, post, &rctx)
}

// resolveBlog reads an existing blog file or generates one from the context.
func resolveBlog(ctx context.Context, blogPath string, rctx *rc.ReleaseContext, model releasegen.Model) (releasegen.BlogPost, error) {
	if blogPath != "" {
		data, err := readInput(blogPath)
		if err != nil {
			return releasegen.BlogPost{}, fmt.Errorf("read blog: %w", err)
		}
		md := string(data)
		return releasegen.BlogPost{Title: blogTitle(md), Tags: blogTags(md), Markdown: md}, nil
	}
	if model == nil {
		return releasegen.BlogPost{}, fmt.Errorf("no --storyboard/--blog provided and --offline set: nothing to narrate")
	}
	fmt.Fprintln(os.Stderr, "warning: no --blog provided; generating a fresh blog (may differ from other stages). Pass --blog, or use ./cmd/generate-all for one consistent blog across the suite.")
	return (&releasegen.Generator{Model: model}).Blog(ctx, rctx)
}

func blogTitle(md string) string {
	for _, ln := range strings.Split(md, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "title:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "title:")), "\"'")
		}
		if strings.HasPrefix(t, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(t, "# "))
		}
	}
	return ""
}

func blogTags(md string) []string {
	for _, ln := range strings.Split(md, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "tags:") {
			inner := strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "tags:")), "[]")
			var tags []string
			for _, p := range strings.Split(inner, ",") {
				if p = strings.TrimSpace(p); p != "" {
					tags = append(tags, p)
				}
			}
			return tags
		}
	}
	return nil
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func writeOut(path, content string) error {
	if path == "" {
		_, err := os.Stdout.WriteString(content)
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
