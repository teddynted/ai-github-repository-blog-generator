// Command blog turns a Release Context (the JSON produced by the
// /release-context endpoint, Milestone 2) into content via the local model.
//
// It reads a ReleaseContext JSON from a file or stdin, calls the releasegen
// engine against a local Ollama server, and writes the result to stdout or a
// file. The default format is a long-form technical blog post (Milestone 3);
// --format selects any other supported format.
//
// Usage:
//
//	go run ./cmd/blog --context ctx.json                 # blog post to stdout
//	go run ./cmd/blog --context ctx.json --out post.md   # to a file
//	cat ctx.json | go run ./cmd/blog --format linkedin   # a LinkedIn post
//	go run ./cmd/blog --context ctx.json --model qwen2.5:7b --ollama http://127.0.0.1:11434
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("blog", flag.ContinueOnError)
	ctxPath := fs.String("context", "-", "path to a Release Context JSON file ('-' for stdin)")
	format := fs.String("format", "blog", "content format: blog | release-summary | documentation | linkedin | youtube-shorts | tiktok | seo-metadata")
	model := fs.String("model", envOr("OLLAMA_MODEL", "qwen2.5:7b"), "Ollama model to use")
	ollamaURL := fs.String("ollama", envOr("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama base URL")
	outPath := fs.String("out", "", "output file (default: stdout)")
	timeout := fs.Duration("timeout", 5*time.Minute, "generation timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	raw, err := readInput(*ctxPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read context: %v\n", err)
		return 1
	}
	var rctx rc.ReleaseContext
	if err := json.Unmarshal(raw, &rctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: parse Release Context JSON: %v\n", err)
		return 1
	}

	gen := &releasegen.Generator{Model: ollama.New(*model, ollama.WithBaseURL(*ollamaURL))}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	out, err := generate(ctx, gen, releasegen.Format(*format), &rctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if err := writeOutput(*outPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		return 1
	}
	if *outPath != "" {
		fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", *outPath, len(out))
	}
	return 0
}

// generate produces the requested format. The blog format uses the dedicated
// long-form Blog() path (SEO front matter + diagrams); others use Generate().
func generate(ctx context.Context, gen *releasegen.Generator, format releasegen.Format, rctx *rc.ReleaseContext) (string, error) {
	if format == releasegen.FormatBlog {
		post, err := gen.Blog(ctx, rctx)
		if err != nil {
			return "", err
		}
		return post.Markdown, nil
	}
	asset, err := gen.Generate(ctx, format, rctx)
	if err != nil {
		return "", err
	}
	return asset.Body, nil
}

func readInput(path string) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func writeOutput(path, content string) error {
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
