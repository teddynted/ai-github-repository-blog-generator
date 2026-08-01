// Command govern runs a piece of generated content through the review-and-
// approval workflow (Milestone 14): deterministic validation, AI-assisted
// (optional) quality review grounded in the Release Context, scoring, and — with
// --auto-approve — the full approve → publish lifecycle. It prints the governance
// report (Markdown) or the full workflow state (JSON).
//
// Usage:
//
//	# Review a blog file, grounded in a release context, offline:
//	go run ./cmd/govern --content post.md --type "Technical Blog" --context ctx.json --offline
//
//	# Drive the whole lifecycle and emit JSON:
//	go run ./cmd/govern --content post.md --context ctx.json --auto-approve --format json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/governance"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/localgen"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("govern", flag.ContinueOnError)
	contentPath := fs.String("content", "", "path to the content file to review (required)")
	ctxPath := fs.String("context", "", "path to a Release Context JSON file (for grounding terms)")
	typ := fs.String("type", "Technical Blog", "content type (e.g. \"Technical Blog\", \"X Thread\", \"SEO Metadata\")")
	titleFlag := fs.String("title", "", "content title (default: derived from the file)")
	autoApprove := fs.Bool("auto-approve", false, "drive the full approve → publish lifecycle")
	format := fs.String("format", "md", "output format: md | json")
	model := fs.String("model", "", "model id (Anthropic; provider default when empty)")
	offline := fs.Bool("offline", false, "do not call the model (deterministic review only)")
	minScore := fs.Int("min-score", 80, "minimum overall score to pass review")
	outPath := fs.String("out", "", "output file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *contentPath == "" {
		fmt.Fprintln(os.Stderr, "error: --content is required")
		return 2
	}

	body, err := os.ReadFile(*contentPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read content: %v\n", err)
		return 1
	}

	grounding := governance.Grounding{}
	if *ctxPath != "" {
		grounding, err = groundingFromContext(*ctxPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}

	content := governance.Content{
		ID:        contentID(*contentPath),
		Type:      governance.ContentType(*typ),
		Title:     firstNonEmpty(*titleFlag, deriveTitle(string(body))),
		Body:      string(body),
		Grounding: grounding,
		Author:    "govern-cli",
	}

	var ai governance.AIReviewer
	if !*offline {
		ai = governance.NewModelReviewer(localgen.Default(*model), "anthropic:"+*model)
	}

	cfg := governance.DefaultConfig()
	cfg.MinOverallScore = *minScore
	engine := governance.NewEngine(cfg, nil, ai, time.Now)

	ctx := context.Background()
	state, err := engine.Submit(content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: submit: %v\n", err)
		return 1
	}
	state, err = engine.Review(ctx, content.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: review: %v\n", err)
		return 1
	}

	if *autoApprove && state.Status == governance.StatusPendingApproval {
		if _, err := engine.Approve(ctx, content.ID, governance.Reviewer{Name: "cli-reviewer", Role: governance.RoleReviewer}, true, "auto-approved by CLI"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: approve: %v\n", err)
		}
		if s, err := engine.Publish(ctx, content.ID, "cli-publisher"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: publish: %v\n", err)
			state = s
		} else {
			state = s
		}
	}
	if latest, err := engine.Get(content.ID); err == nil {
		state = latest
	}

	var out string
	if *format == "json" {
		b, _ := json.MarshalIndent(state, "", "  ")
		out = string(b)
	} else {
		out = state.Markdown()
	}
	if err := writeOut(*outPath, out); err != nil {
		fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "status: %s\n", state.Status)
	// Non-zero exit when content did not pass, so CI/n8n can branch on it.
	if state.Status == governance.StatusNeedsRevision || state.Status == governance.StatusRejected {
		return 3
	}
	return 0
}

// groundingFromContext extracts grounded terms from a Release Context JSON.
func groundingFromContext(path string) (governance.Grounding, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return governance.Grounding{}, fmt.Errorf("read context: %w", err)
	}
	var c rc.ReleaseContext
	if err := json.Unmarshal(raw, &c); err != nil {
		return governance.Grounding{}, fmt.Errorf("parse Release Context JSON: %w", err)
	}
	var terms []string
	terms = append(terms, c.Repository.Name)
	terms = append(terms, c.Release.Tag)
	terms = append(terms, c.Architecture.AWSServices...)
	for _, t := range c.Technologies {
		terms = append(terms, t.Name)
	}
	terms = append(terms, c.Changelog.Features...)
	terms = append(terms, c.ContentIntelligence.TechnicalHighlights...)
	terms = append(terms, c.ContentIntelligence.ArchitectureHighlights...)
	terms = append(terms, c.ContentIntelligence.SEOKeywords...)
	return governance.Grounding{Repository: c.Repository.FullName, Release: c.Release.Tag, Terms: nonEmpty(terms)}, nil
}

func deriveTitle(body string) string {
	for _, ln := range strings.Split(body, "\n") {
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

func contentID(path string) string {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".md")
}

func nonEmpty(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
