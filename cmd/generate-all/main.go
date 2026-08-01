// Command generate-all is the single entry point that turns one Release Context
// into the COMPLETE set of release artifacts (Milestones 3–13) in one run — the
// orchestrator the QA quality gate found missing. It composes internal/contentsuite
// over the dedicated generator packages, writes every artifact to an output
// directory, and emits a manifest.json correlating the release with its outputs.
//
// A single failing stage never aborts the run: it is recorded in the manifest and
// the rest still generate.
//
// Usage:
//
//	# Fully offline/deterministic (no model). Requires a pre-generated blog.
//	go run ./cmd/generate-all --context ctx.json --blog post.md --offline --out ./artifacts
//
//	# Generate everything, blog included, via the Anthropic API.
//	go run ./cmd/generate-all --context ctx.json --out ./artifacts
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentsuite"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/localgen"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("generate-all", flag.ContinueOnError)
	ctxPath := fs.String("context", "", "path to a Release Context JSON file (required)")
	blogPath := fs.String("blog", "", "path to an existing blog Markdown file (required with --offline)")
	outDir := fs.String("out", "artifacts", "output directory for artifacts + manifest.json")
	offline := fs.Bool("offline", false, "do not call any model (deterministic; requires --blog)")
	model := fs.String("model", "", "model id (Anthropic; provider default when empty)")
	timeout := fs.Duration("timeout", 15*time.Minute, "overall generation timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *ctxPath == "" {
		fmt.Fprintln(os.Stderr, "error: --context is required")
		return 2
	}
	if *offline && *blogPath == "" {
		fmt.Fprintln(os.Stderr, "error: --offline requires --blog (the blog generator needs a model)")
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

	orch := &contentsuite.Orchestrator{}
	if !*offline {
		orch.Model = localgen.Default(*model)
	}

	var blog *releasegen.BlogPost
	if *blogPath != "" {
		data, err := os.ReadFile(*blogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read blog: %v\n", err)
			return 1
		}
		md := string(data)
		blog = &releasegen.BlogPost{Title: blogTitle(md), Markdown: md}
	}

	suite := orch.Run(ctx, &rctx, blog)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: create out dir: %v\n", err)
		return 1
	}
	for _, a := range suite.Artifacts() {
		if err := os.WriteFile(filepath.Join(*outDir, a.Filename), []byte(a.Markdown), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", a.Filename, err)
			return 1
		}
	}
	manifest, _ := json.MarshalIndent(suite.Manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(*outDir, "manifest.json"), append(manifest, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write manifest: %v\n", err)
		return 1
	}

	m := suite.Manifest
	fmt.Fprintf(os.Stderr, "generated %d/%d artifacts for %s %s -> %s\n",
		m.Produced, m.Produced+m.Failed, m.Repository, m.Release, *outDir)
	for _, st := range m.Stages {
		mark := "✓"
		if st.Status != contentsuite.StageOK {
			mark = "✗"
		}
		fmt.Fprintf(os.Stderr, "  %s M%-2d %-16s %s\n", mark, st.Milestone, st.Name, st.Error)
	}
	// Exit non-zero only if NOTHING was produced (a real failure); partial
	// generation is a success with recorded per-stage issues.
	if m.Produced == 0 {
		return 1
	}
	return 0
}

// blogTitle extracts the front-matter/H1 title from a blog file.
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
