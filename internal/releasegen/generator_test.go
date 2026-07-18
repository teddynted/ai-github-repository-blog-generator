package releasegen

import (
	"context"
	"errors"
	"strings"
	"testing"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

type fakeModel struct {
	prompts []string
	reply   func(prompt string) (string, error)
}

func (f *fakeModel) Generate(_ context.Context, prompt string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	if f.reply != nil {
		return f.reply(prompt)
	}
	return "GENERATED CONTENT", nil
}

func sampleContext() *rc.ReleaseContext {
	return &rc.ReleaseContext{
		Repository: rc.Repository{FullName: "acme/widget", Name: "widget", Language: "Go",
			Topics: []string{"aws", "golang"}, Summary: "acme/widget is a project that turns repos into content."},
		Release: rc.Release{Tag: "v0.2.0", Name: "v0.2.0", PreviousTag: "v0.1.0",
			Summary: "Release v0.2.0 published 2026-07-20.", Body: "Release context builder."},
		Changelog: rc.ChangelogAnalysis{Found: true,
			Features: []string{"add release context builder"}, BugFixes: []string{"fix empty changelog"}},
		CommitStats:    rc.CommitStats{Analyzed: 4, ByCategory: map[string]int{"Features": 2, "Bug Fixes": 1}},
		FileStats:      rc.FileStats{Total: 5},
		Implementation: rc.ImplementationSummary{WhyItMatters: "Release v0.2.0 advances the platform.", WhatChanged: []string{"add builder"}},
		Architecture:   rc.Architecture{Overview: "An event-driven, AWS-native system.", AWSServices: []string{"AWS Lambda", "Amazon SQS"}},
		Technologies:   []rc.Technology{{Name: "Go"}, {Name: "AWS Lambda"}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary: "widget v0.2.0 delivers 4 changes.", TargetAudience: "Software engineers",
			BlogTitles: []string{"Inside widget v0.2.0"}, SEOKeywords: []string{"go", "aws"},
			ArticleOutline: []string{"Introduction"}, ImplementationComplexity: "medium",
		},
	}
}

func TestGenerateGroundsPromptInContext(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm}
	asset, err := g.Generate(context.Background(), FormatBlog, sampleContext())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if asset.Format != FormatBlog || asset.Body != "GENERATED CONTENT" {
		t.Errorf("asset = %+v", asset)
	}
	if asset.Title != "Inside widget v0.2.0" {
		t.Errorf("title = %q (should reuse content-intelligence blog title)", asset.Title)
	}
	p := fm.prompts[0]
	for _, want := range []string{"acme/widget", "v0.2.0", "add release context builder", "Do not invent", "You are a senior software engineer"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestGenerateAllProducesEveryFormat(t *testing.T) {
	g := &Generator{Model: &fakeModel{}}
	assets, err := g.GenerateAll(context.Background(), sampleContext())
	if err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}
	if len(assets) != len(AllFormats) {
		t.Fatalf("assets = %d, want %d", len(assets), len(AllFormats))
	}
	seen := map[Format]bool{}
	for _, a := range assets {
		seen[a.Format] = true
	}
	for _, f := range AllFormats {
		if !seen[f] {
			t.Errorf("missing format %q", f)
		}
	}
}

func TestGenerateAllAggregatesErrors(t *testing.T) {
	// The model fails only for the LinkedIn prompt; the rest still succeed.
	fm := &fakeModel{reply: func(p string) (string, error) {
		if strings.Contains(p, "LinkedIn post") {
			return "", errors.New("model overloaded")
		}
		return "ok", nil
	}}
	g := &Generator{Model: fm}
	assets, err := g.GenerateAll(context.Background(), sampleContext())
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	if len(assets) != len(AllFormats)-1 {
		t.Errorf("assets = %d, want %d (partial)", len(assets), len(AllFormats)-1)
	}
	if !strings.Contains(err.Error(), "1 of") {
		t.Errorf("error should report failure count: %v", err)
	}
}

func TestGenerateUnknownFormat(t *testing.T) {
	g := &Generator{Model: &fakeModel{}}
	if _, err := g.Generate(context.Background(), Format("nope"), sampleContext()); err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestGenerateNoModel(t *testing.T) {
	g := &Generator{}
	if _, err := g.Generate(context.Background(), FormatBlog, sampleContext()); err == nil {
		t.Error("expected error when no model is configured")
	}
}

func TestPromptTruncation(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, MaxPromptBytes: 200}
	if _, err := g.Generate(context.Background(), FormatBlog, sampleContext()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fm.prompts[0], "truncated to fit") {
		t.Error("small budget should truncate the grounding block")
	}
}
