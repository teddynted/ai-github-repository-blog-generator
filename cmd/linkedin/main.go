// Command linkedin converts a GitHub Release's generated artifacts into
// professional LinkedIn content (Milestone 12): multiple post variations
// targeting software, cloud, and AI engineering audiences.
//
// It reads a Release Context JSON and an optional blog, builds the content chain
// (blog → … → SEO → architecture) offline or via the Anthropic API, then emits the LinkedIn
// package as Markdown (default) or JSON. It generates content, not published
// posts.
//
// Usage:
//
//	go run ./cmd/linkedin --context ctx.json --blog post.md --offline
//	go run ./cmd/linkedin --context ctx.json --format json --out linkedin.json
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

	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/linkedin"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/localgen"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/seo"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/shorts"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/tiktok"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/visualassets"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("linkedin", flag.ContinueOnError)
	ctxPath := fs.String("context", "", "path to a Release Context JSON file (required)")
	blogPath := fs.String("blog", "", "path to an existing blog Markdown file (else generate)")
	maxPosts := fs.Int("max", 6, "maximum number of posts to generate")
	format := fs.String("format", "md", "output format: md | json")
	model := fs.String("model", "", "model id (Anthropic; provider default when empty)")
	offline := fs.Bool("offline", false, "do not call the model (deterministic content)")
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
		mdl = localgen.Default(*model)
	}

	rctx, err := loadContext(*ctxPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	pkg, err := buildPackage(ctx, rctx, *blogPath, mdl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	col, err := (&linkedin.Generator{Model: mdl, MaxPosts: *maxPosts}).LinkedIn(ctx, pkg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if probs := col.Validate(pkg); len(probs) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d validation issue(s):\n", len(probs))
		for _, p := range probs {
			fmt.Fprintf(os.Stderr, "  - %s\n", p)
		}
	}

	var out string
	if *format == "json" {
		b, _ := json.MarshalIndent(col, "", "  ")
		out = string(b)
	} else {
		out = col.Markdown()
	}
	if err := writeOut(*outPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		return 1
	}
	if *outPath != "" {
		fmt.Fprintf(os.Stderr, "wrote %s (%d posts)\n", *outPath, col.Metadata.PostCount)
	}
	return 0
}

// buildPackage assembles the full content chain, then the LinkedIn inputs.
func buildPackage(ctx context.Context, rctx *rc.ReleaseContext, blogPath string, mdl releasegen.Model) (linkedin.ReleasePackage, error) {
	var zero linkedin.ReleasePackage

	post, err := resolveBlog(ctx, blogPath, rctx, mdl)
	if err != nil {
		return zero, err
	}
	sb, err := (&storyboard.Generator{Model: mdl}).Storyboard(ctx, post, rctx)
	if err != nil {
		return zero, fmt.Errorf("storyboard: %w", err)
	}
	vo, err := (&voiceover.Generator{Model: mdl}).VoiceOver(ctx, sb)
	if err != nil {
		return zero, fmt.Errorf("voiceover: %w", err)
	}
	yt, err := (&youtube.Generator{Model: mdl}).YouTube(ctx, youtube.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo})
	if err != nil {
		return zero, fmt.Errorf("youtube: %w", err)
	}
	sh, err := (&shorts.Generator{Model: mdl}).YouTubeShorts(ctx, shorts.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt})
	if err != nil {
		return zero, fmt.Errorf("shorts: %w", err)
	}
	tt, err := (&tiktok.Generator{Model: mdl}).TikTok(ctx, tiktok.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt, Shorts: sh})
	if err != nil {
		return zero, fmt.Errorf("tiktok: %w", err)
	}
	va, err := (&visualassets.Generator{Model: mdl}).VisualAssets(ctx, visualassets.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt, Shorts: sh, TikTok: tt})
	if err != nil {
		return zero, fmt.Errorf("visualassets: %w", err)
	}
	se, err := (&seo.Generator{Model: mdl}).SEO(ctx, seo.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt, Shorts: sh, TikTok: tt, VisualAssets: va})
	if err != nil {
		return zero, fmt.Errorf("seo: %w", err)
	}
	ar, err := (&architecture.Generator{Model: mdl}).Architecture(ctx, architecture.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb})
	if err != nil {
		// Architecture is optional for LinkedIn (thin contexts may not ground it).
		ar = architecture.ArchitectureCollection{}
	}

	return linkedin.ReleasePackage{
		Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt,
		Shorts: sh, TikTok: tt, VisualAssets: va, SEO: se, Architecture: ar,
	}, nil
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
