// Command youtube assembles a production-ready long-form YouTube script
// (Milestone 6) from the previously generated artifacts — Release Context (M2),
// Technical Blog (M3), Storyboard (M4), and Voice-over Script (M5).
//
// It reads a Release Context JSON and either reuses existing artifacts (via
// --blog / --storyboard / --voiceover) or builds the missing ones on the fly,
// then emits the YouTube script as Markdown (default) or JSON.
//
// Usage:
//
//	# Fully offline from a context + an existing blog (deterministic):
//	go run ./cmd/youtube --context ctx.json --blog post.md --offline
//
//	# Reuse a pre-built storyboard + voice-over, offline:
//	go run ./cmd/youtube --context ctx.json --storyboard board.json --voiceover vo.json --offline
//
//	# Generate the whole chain via Ollama, then the script as JSON:
//	go run ./cmd/youtube --context ctx.json --format json --out script.json
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
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("youtube", flag.ContinueOnError)
	ctxPath := fs.String("context", "", "path to a Release Context JSON file (required)")
	blogPath := fs.String("blog", "", "path to an existing blog Markdown file (else generate)")
	sbPath := fs.String("storyboard", "", "path to an existing Storyboard JSON file (else generate)")
	voPath := fs.String("voiceover", "", "path to an existing Voice-over JSON file (else generate)")
	format := fs.String("format", "md", "output format: md | json")
	model := fs.String("model", envOr("OLLAMA_MODEL", "qwen2.5:7b"), "Ollama model")
	ollamaURL := fs.String("ollama", envOr("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama base URL")
	offline := fs.Bool("offline", false, "do not call the model (deterministic narration)")
	outPath := fs.String("out", "", "output file (default: stdout)")
	timeout := fs.Duration("timeout", 10*time.Minute, "generation timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *ctxPath == "" {
		fmt.Fprintln(os.Stderr, "error: --context is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var mdl releasegen.Model
	if !*offline {
		mdl = ollama.New(*model, ollama.WithBaseURL(*ollamaURL))
	}

	rctx, err := loadContext(*ctxPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	pkg, err := buildPackage(ctx, rctx, *blogPath, *sbPath, *voPath, mdl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	script, err := (&youtube.Generator{Model: mdl}).YouTube(ctx, pkg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if probs := script.Validate(len(pkg.Storyboard.Scenes)); len(probs) > 0 {
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
		fmt.Fprintf(os.Stderr, "wrote %s (%d chapters, ~%s)\n", *outPath, len(script.Chapters), script.Video.Duration)
	}
	return 0
}

func loadContext(path string) (*rc.ReleaseContext, error) {
	raw, err := readInput(path)
	if err != nil {
		return nil, fmt.Errorf("read context: %w", err)
	}
	var rctx rc.ReleaseContext
	if err := json.Unmarshal(raw, &rctx); err != nil {
		return nil, fmt.Errorf("parse Release Context JSON: %w", err)
	}
	return &rctx, nil
}

// buildPackage assembles the ReleasePackage, reusing any artifacts provided on
// the command line and generating the rest from the Release Context.
func buildPackage(ctx context.Context, rctx *rc.ReleaseContext, blogPath, sbPath, voPath string, mdl releasegen.Model) (youtube.ReleasePackage, error) {
	post, err := resolveBlog(ctx, blogPath, rctx, mdl)
	if err != nil {
		return youtube.ReleasePackage{}, err
	}

	sb, err := resolveStoryboard(ctx, sbPath, post, rctx, mdl)
	if err != nil {
		return youtube.ReleasePackage{}, err
	}

	vo, err := resolveVoiceOver(ctx, voPath, sb, mdl)
	if err != nil {
		return youtube.ReleasePackage{}, err
	}

	return youtube.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo}, nil
}

func resolveBlog(ctx context.Context, blogPath string, rctx *rc.ReleaseContext, mdl releasegen.Model) (releasegen.BlogPost, error) {
	if blogPath != "" {
		data, err := readInput(blogPath)
		if err != nil {
			return releasegen.BlogPost{}, fmt.Errorf("read blog: %w", err)
		}
		md := string(data)
		return releasegen.BlogPost{Title: blogTitle(md), Tags: blogTags(md), Markdown: md}, nil
	}
	if mdl == nil {
		return releasegen.BlogPost{}, fmt.Errorf("no --blog provided and --offline set: cannot generate a blog")
	}
	fmt.Fprintln(os.Stderr, "warning: no --blog provided; generating a fresh blog (may differ from other stages). Pass --blog, or use ./cmd/generate-all for one consistent blog across the suite.")
	return (&releasegen.Generator{Model: mdl}).Blog(ctx, rctx)
}

func resolveStoryboard(ctx context.Context, sbPath string, post releasegen.BlogPost, rctx *rc.ReleaseContext, mdl releasegen.Model) (storyboard.Storyboard, error) {
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
	return (&storyboard.Generator{Model: mdl}).Storyboard(ctx, post, rctx)
}

func resolveVoiceOver(ctx context.Context, voPath string, sb storyboard.Storyboard, mdl releasegen.Model) (voiceover.VoiceOverScript, error) {
	if voPath != "" {
		data, err := readInput(voPath)
		if err != nil {
			return voiceover.VoiceOverScript{}, fmt.Errorf("read voice-over: %w", err)
		}
		var vo voiceover.VoiceOverScript
		if err := json.Unmarshal(data, &vo); err != nil {
			return voiceover.VoiceOverScript{}, fmt.Errorf("parse Voice-over JSON: %w", err)
		}
		return vo, nil
	}
	return (&voiceover.Generator{Model: mdl}).VoiceOver(ctx, sb)
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
