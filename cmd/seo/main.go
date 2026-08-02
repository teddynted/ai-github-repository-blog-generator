// Command seo converts a GitHub Release's generated artifacts — Release Context
// (M2), Technical Blog (M3), Storyboard (M4), Voice-over (M5), YouTube Script
// (M6), YouTube Shorts (M7), TikTok (M8), and Visual Assets (M9) — into
// structured, platform-specific SEO metadata (Milestone 10): blog, YouTube,
// short-form, social, keywords, hashtags, Open Graph, and structured data.
//
// It reads a Release Context JSON and reuses any artifacts you pass, generating
// the rest, then emits the SEO metadata as Markdown (default) or JSON.
//
// Usage:
//
//	# Fully offline from a context + an existing blog (deterministic):
//	go run ./cmd/seo --context ctx.json --blog post.md --offline
//
//	# Generate the whole chain via the Anthropic API, then the SEO as JSON:
//	go run ./cmd/seo --context ctx.json --format json --out seo.json
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
	fs := flag.NewFlagSet("seo", flag.ContinueOnError)
	ctxPath := fs.String("context", "", "path to a Release Context JSON file (required)")
	blogPath := fs.String("blog", "", "path to an existing blog Markdown file (else generate)")
	sbPath := fs.String("storyboard", "", "path to an existing Storyboard JSON file (else generate)")
	voPath := fs.String("voiceover", "", "path to an existing Voice-over JSON file (else generate)")
	ytPath := fs.String("youtube", "", "path to an existing YouTube Script JSON file (else generate)")
	shPath := fs.String("shorts", "", "path to an existing YouTube Shorts JSON file (else generate)")
	ttPath := fs.String("tiktok", "", "path to an existing TikTok JSON file (else generate)")
	format := fs.String("format", "md", "output format: md | json")
	model := fs.String("model", "", "model id (Anthropic; provider default when empty)")
	offline := fs.Bool("offline", false, "do not call the model (deterministic metadata)")
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

	pkg, err := buildPackage(ctx, rctx, sources{blog: *blogPath, sb: *sbPath, vo: *voPath, yt: *ytPath, sh: *shPath, tt: *ttPath}, mdl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	m, err := (&seo.Generator{Model: mdl}).SEO(ctx, pkg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if probs := m.Validate(pkg); len(probs) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d validation issue(s):\n", len(probs))
		for _, p := range probs {
			fmt.Fprintf(os.Stderr, "  - %s\n", p)
		}
	}

	var out string
	if *format == "json" {
		b, _ := json.MarshalIndent(m, "", "  ")
		out = string(b)
	} else {
		out = m.Markdown()
	}
	if err := writeOut(*outPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		return 1
	}
	if *outPath != "" {
		fmt.Fprintf(os.Stderr, "wrote %s (SEO confidence %d/100)\n", *outPath, m.ContentIntelligence.SEOConfidenceScore)
	}
	return 0
}

type sources struct{ blog, sb, vo, yt, sh, tt string }

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

func buildPackage(ctx context.Context, rctx *rc.ReleaseContext, src sources, mdl releasegen.Model) (seo.ReleasePackage, error) {
	post, err := resolveBlog(ctx, src.blog, rctx, mdl)
	if err != nil {
		return seo.ReleasePackage{}, err
	}
	sb, err := resolveStoryboard(ctx, src.sb, post, rctx, mdl)
	if err != nil {
		return seo.ReleasePackage{}, err
	}
	vo, err := resolveVoiceOver(ctx, src.vo, sb, mdl)
	if err != nil {
		return seo.ReleasePackage{}, err
	}
	yt, err := resolveYouTube(ctx, src.yt, rctx, post, sb, vo, mdl)
	if err != nil {
		return seo.ReleasePackage{}, err
	}
	sh, err := resolveShorts(ctx, src.sh, rctx, post, sb, vo, yt, mdl)
	if err != nil {
		return seo.ReleasePackage{}, err
	}
	tt, err := resolveTikTok(ctx, src.tt, rctx, post, sb, vo, yt, sh, mdl)
	if err != nil {
		return seo.ReleasePackage{}, err
	}
	va, err := (&visualassets.Generator{Model: mdl}).VisualAssets(ctx, visualassets.ReleasePackage{
		Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt, Shorts: sh, TikTok: tt,
	})
	if err != nil {
		return seo.ReleasePackage{}, err
	}
	return seo.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt, Shorts: sh, TikTok: tt, VisualAssets: va}, nil
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

func resolveYouTube(ctx context.Context, ytPath string, rctx *rc.ReleaseContext, post releasegen.BlogPost, sb storyboard.Storyboard, vo voiceover.VoiceOverScript, mdl releasegen.Model) (youtube.YouTubeScript, error) {
	if ytPath != "" {
		data, err := readInput(ytPath)
		if err != nil {
			return youtube.YouTubeScript{}, fmt.Errorf("read youtube script: %w", err)
		}
		var yt youtube.YouTubeScript
		if err := json.Unmarshal(data, &yt); err != nil {
			return youtube.YouTubeScript{}, fmt.Errorf("parse YouTube Script JSON: %w", err)
		}
		return yt, nil
	}
	return (&youtube.Generator{Model: mdl}).YouTube(ctx, youtube.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo})
}

func resolveShorts(ctx context.Context, shPath string, rctx *rc.ReleaseContext, post releasegen.BlogPost, sb storyboard.Storyboard, vo voiceover.VoiceOverScript, yt youtube.YouTubeScript, mdl releasegen.Model) (shorts.ShortsCollection, error) {
	if shPath != "" {
		data, err := readInput(shPath)
		if err != nil {
			return shorts.ShortsCollection{}, fmt.Errorf("read shorts: %w", err)
		}
		var sh shorts.ShortsCollection
		if err := json.Unmarshal(data, &sh); err != nil {
			return shorts.ShortsCollection{}, fmt.Errorf("parse YouTube Shorts JSON: %w", err)
		}
		return sh, nil
	}
	return (&shorts.Generator{Model: mdl}).YouTubeShorts(ctx, shorts.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt})
}

func resolveTikTok(ctx context.Context, ttPath string, rctx *rc.ReleaseContext, post releasegen.BlogPost, sb storyboard.Storyboard, vo voiceover.VoiceOverScript, yt youtube.YouTubeScript, sh shorts.ShortsCollection, mdl releasegen.Model) (tiktok.TikTokCollection, error) {
	if ttPath != "" {
		data, err := readInput(ttPath)
		if err != nil {
			return tiktok.TikTokCollection{}, fmt.Errorf("read tiktok: %w", err)
		}
		var tt tiktok.TikTokCollection
		if err := json.Unmarshal(data, &tt); err != nil {
			return tiktok.TikTokCollection{}, fmt.Errorf("parse TikTok JSON: %w", err)
		}
		return tt, nil
	}
	return (&tiktok.Generator{Model: mdl}).TikTok(ctx, tiktok.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt, Shorts: sh})
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
