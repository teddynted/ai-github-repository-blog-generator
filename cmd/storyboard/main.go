// Command storyboard converts a technical blog + its Release Context into a
// structured video storyboard (Milestone 4).
//
// It reads a Release Context JSON (from the /release-context endpoint or S3) and
// either an already-generated blog (--blog) or generates one on the fly via
// the Anthropic API, then produces the storyboard as Markdown (default) or JSON.
//
// Usage:
//
//	# From a context + an existing blog file, fully offline (deterministic):
//	go run ./cmd/storyboard --context ctx.json --blog post.md --offline
//
//	# Generate the blog first (needs the Anthropic API), then the storyboard as JSON:
//	go run ./cmd/storyboard --context ctx.json --format json --out board.json
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

	"github.com/teddynted/ai-github-repository-blog-generator/internal/localgen"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("storyboard", flag.ContinueOnError)
	ctxPath := fs.String("context", "", "path to a Release Context JSON file (required)")
	blogPath := fs.String("blog", "", "path to an existing blog Markdown file (else generate via the Anthropic API)")
	format := fs.String("format", "md", "output format: md | json")
	model := fs.String("model", "", "model id (Anthropic; provider default when empty)")
	offline := fs.Bool("offline", false, "do not call the model (deterministic narration)")
	outPath := fs.String("out", "", "output file (default: stdout)")
	timeout := fs.Duration("timeout", 5*time.Minute, "generation timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *ctxPath == "" {
		fmt.Fprintln(os.Stderr, "error: --context is required")
		return 2
	}

	raw, err := os.ReadFile(*ctxPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read context: %v\n", err)
		return 1
	}
	var rctx rc.ReleaseContext
	if err := json.Unmarshal(raw, &rctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: parse Release Context JSON: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var model2 releasegen.Model
	if !*offline {
		model2 = localgen.Default(*model)
	}

	post, err := resolveBlog(ctx, *blogPath, &rctx, model2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	gen := &storyboard.Generator{Model: model2}
	sb, err := gen.Storyboard(ctx, post, &rctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	var out string
	if *format == "json" {
		b, _ := json.MarshalIndent(sb, "", "  ")
		out = string(b)
	} else {
		out = sb.Markdown()
	}
	if err := writeOut(*outPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		return 1
	}
	if *outPath != "" {
		fmt.Fprintf(os.Stderr, "wrote %s (%d scenes, ~%s)\n", *outPath, sb.Video.SceneCount, sb.ContentIntelligence.EstimatedVideoLength)
	}
	return 0
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
		return releasegen.BlogPost{}, fmt.Errorf("no --blog provided and --offline set: nothing to storyboard")
	}
	fmt.Fprintln(os.Stderr, "warning: no --blog provided; generating a fresh blog (may differ from other stages). Pass --blog, or use ./cmd/generate-all for one consistent blog across the suite.")
	return (&releasegen.Generator{Model: model}).Blog(ctx, rctx)
}

// blogTitle extracts the front-matter title, falling back to the first H1.
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

// blogTags extracts a front-matter "tags: [a, b]" line.
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
