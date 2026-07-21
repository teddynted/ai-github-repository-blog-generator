// Command architecture converts a GitHub Release's Release Context (M2) and
// downstream artifacts (Blog M3, Storyboard M4) into production-ready
// architecture diagrams — Mermaid, Graphviz DOT, and SVG — plus PNG export
// metadata (Milestone 11).
//
// It reads a Release Context JSON and an optional blog, then emits the diagrams
// as Markdown (default) or JSON. Every node and edge is grounded in the Release
// Context; nothing is invented.
//
// Usage:
//
//	# Offline from a context (deterministic):
//	go run ./cmd/architecture --context ctx.json --offline
//
//	# With a blog, output JSON:
//	go run ./cmd/architecture --context ctx.json --blog post.md --format json --out arch.json
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("architecture", flag.ContinueOnError)
	ctxPath := fs.String("context", "", "path to a Release Context JSON file (required)")
	blogPath := fs.String("blog", "", "path to an existing blog Markdown file (optional)")
	sbPath := fs.String("storyboard", "", "path to an existing Storyboard JSON file (optional)")
	format := fs.String("format", "md", "output format: md | json")
	model := fs.String("model", envOr("OLLAMA_MODEL", "qwen2.5:7b"), "Ollama model")
	ollamaURL := fs.String("ollama", envOr("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama base URL")
	offline := fs.Bool("offline", false, "do not call the model (deterministic descriptions)")
	svgDir := fs.String("svg-dir", "", "if set, also write each diagram's SVG to this directory")
	outPath := fs.String("out", "", "output file (default: stdout)")
	timeout := fs.Duration("timeout", 5*time.Minute, "generation timeout")
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
	post, err := resolveBlog(*blogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	sb, err := resolveStoryboard(*sbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	pkg := architecture.ReleasePackage{Context: rctx, Blog: post, Storyboard: sb}
	col, err := (&architecture.Generator{Model: mdl}).Architecture(ctx, pkg)
	if errors.Is(err, architecture.ErrNoInfrastructure) {
		// A documentation-only / non-AWS release has nothing to diagram. That is a
		// legitimate skip, not a failure — exit cleanly so a pipeline continues.
		fmt.Fprintln(os.Stderr, "no groundable infrastructure in this release — skipping architecture diagram")
		return 0
	}
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

	if *svgDir != "" {
		if err := writeSVGs(*svgDir, col); err != nil {
			fmt.Fprintf(os.Stderr, "error: write SVGs: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "wrote %d SVG(s) to %s\n", len(col.Diagrams), *svgDir)
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
		fmt.Fprintf(os.Stderr, "wrote %s (%d diagrams)\n", *outPath, col.Metadata.DiagramCount)
	}
	return 0
}

func writeSVGs(dir string, col architecture.ArchitectureCollection) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, d := range col.Diagrams {
		name := fmt.Sprintf("%02d-%s.svg", d.ID, slug(d.Type))
		if err := os.WriteFile(dir+"/"+name, []byte(d.SVG), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func slug(s string) string {
	var b strings.Builder
	prev := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prev = false
		} else if !prev && b.Len() > 0 {
			b.WriteByte('-')
			prev = true
		}
	}
	return strings.Trim(b.String(), "-")
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

func resolveBlog(blogPath string) (releasegen.BlogPost, error) {
	if blogPath == "" {
		return releasegen.BlogPost{}, nil
	}
	data, err := readInput(blogPath)
	if err != nil {
		return releasegen.BlogPost{}, fmt.Errorf("read blog: %w", err)
	}
	md := string(data)
	return releasegen.BlogPost{Title: blogTitle(md), Markdown: md}, nil
}

func resolveStoryboard(sbPath string) (storyboard.Storyboard, error) {
	if sbPath == "" {
		return storyboard.Storyboard{}, nil
	}
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
