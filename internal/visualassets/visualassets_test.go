package visualassets

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/shorts"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/tiktok"
)

func samplePackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "widget", FullName: "acme/widget", URL: "https://github.com/acme/widget", Language: "Go"},
		Release:       rc.Release{Tag: "v0.2.0"},
		Architecture: rc.Architecture{
			Overview:    "An event-driven pipeline that turns GitHub releases into content.",
			AWSServices: []string{"AWS Lambda", "Amazon SQS", "Amazon EventBridge"},
		},
		CommitStats:  rc.CommitStats{Analyzed: 9, Total: 9},
		FileStats:    rc.FileStats{Total: 12},
		Changelog:    rc.ChangelogAnalysis{Found: true, Features: []string{"release context builder"}},
		Mermaid:      []rc.MermaidDiagram{{Source: "docs/architecture.md", Type: "flowchart"}},
		Technologies: []rc.Technology{{Name: "Go"}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:                  "widget v0.2.0 delivers a release context builder.",
			ImplementationComplexity: "medium",
			TargetAudience:           "Cloud engineers",
			SEOKeywords:              []string{"go", "aws", "serverless"},
		},
	}
	post := releasegen.BlogPost{Title: "Inside widget v0.2.0", Tags: []string{"go", "aws"}}
	sh := shorts.ShortsCollection{
		SchemaVersion: shorts.SchemaVersion,
		Shorts:        []shorts.Short{{ID: 1, Angle: "Architecture Reveal"}},
	}
	tt := tiktok.TikTokCollection{
		SchemaVersion: tiktok.SchemaVersion,
		Videos:        []tiktok.Video{{ID: 1, Topic: "Architecture Insight"}},
	}
	return ReleasePackage{Context: ctx, Blog: post, Shorts: sh, TikTok: tt}
}

func newGen() *Generator {
	return &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
}

func TestVisualAssetsEndToEnd(t *testing.T) {
	pkg := samplePackage()
	col, err := newGen().VisualAssets(context.Background(), pkg)
	if err != nil {
		t.Fatalf("VisualAssets: %v", err)
	}

	if col.SchemaVersion != SchemaVersion || col.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", col.Metadata)
	}
	if len(col.Assets) < 5 {
		t.Fatalf("expected many assets, got %d", len(col.Assets))
	}
	if col.Metadata.AssetCount != len(col.Assets) {
		t.Errorf("assetCount %d != %d", col.Metadata.AssetCount, len(col.Assets))
	}

	// Branding is populated and applied to every asset.
	if len(col.Branding.PrimaryColors) == 0 || col.Branding.VisualTone == "" {
		t.Errorf("branding not populated: %+v", col.Branding)
	}

	for _, a := range col.Assets {
		if a.Prompt == "" || a.Platform == "" || a.AspectRatio == "" {
			t.Errorf("asset %d (%s) under-populated", a.ID, a.Type)
		}
		if a.Style.Style == "" && a.Style.Composition == "" {
			t.Errorf("asset %d missing style", a.ID)
		}
		if len(a.References) == 0 {
			t.Errorf("asset %d not grounded", a.ID)
		}
		// Prompts never embed literal text.
		if !strings.Contains(strings.ToLower(a.Prompt), "no text") {
			t.Errorf("asset %d prompt should instruct no literal text: %q", a.ID, a.Prompt)
		}
	}

	if probs := col.Validate(pkg); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
	blob, err := json.Marshal(col)
	if err != nil || !strings.Contains(string(blob), "\"schemaVersion\":\"1.0.0\"") {
		t.Errorf("marshal: %v", err)
	}
}

func TestDiscoveryCoversPlatformsAndConditionals(t *testing.T) {
	pkg := samplePackage()
	cands := discover(pkg, 20)
	types := map[string]bool{}
	for _, c := range cands {
		types[c.Type] = true
	}
	for _, want := range []string{
		"YouTube Thumbnail", "GitHub Social Card", "LinkedIn Banner", "X Image",
		"Blog Header", "Architecture Illustration", "AWS Workflow Diagram",
		"YouTube Shorts Cover", "TikTok Cover", "Release Card",
	} {
		if !types[want] {
			t.Errorf("discovery missing %q", want)
		}
	}
}

func TestDiscoverySkipsCoversWithoutArtifacts(t *testing.T) {
	pkg := samplePackage()
	pkg.Shorts = shorts.ShortsCollection{}
	pkg.TikTok = tiktok.TikTokCollection{}
	for _, c := range discover(pkg, 20) {
		if c.Type == "YouTube Shorts Cover" || c.Type == "TikTok Cover" {
			t.Errorf("cover %q should be skipped when the artifact is absent", c.Type)
		}
	}
}

func TestArchitectureSkippedWithoutArchitecture(t *testing.T) {
	pkg := samplePackage()
	pkg.Context.Architecture = rc.Architecture{}
	pkg.Context.Mermaid = nil
	col, err := newGen().VisualAssets(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range col.Assets {
		if a.Type == "Architecture Illustration" || a.Type == "AWS Workflow Diagram" {
			t.Errorf("architecture asset %q should be skipped", a.Type)
		}
	}
	if len(col.Warnings) == 0 {
		t.Error("expected a warning about skipped architecture illustrations")
	}
}

func TestBrandingAWSAccent(t *testing.T) {
	pkg := samplePackage()
	if b := planBranding(pkg); b.AccentColors[0] != "#FF9900" {
		t.Errorf("AWS release should use AWS-orange accent, got %v", b.AccentColors)
	}
	pkg.Context.Architecture.AWSServices = nil
	if b := planBranding(pkg); b.AccentColors[0] == "#FF9900" {
		t.Errorf("non-AWS release should not use AWS orange, got %v", b.AccentColors)
	}
}

func TestThumbnailPromptGrounded(t *testing.T) {
	pkg := samplePackage()
	c := candidate{Type: "YouTube Thumbnail", Platform: "YouTube", AspectRatio: "16:9", Category: catThumbnail,
		Subject: "an event-driven pipeline", Focus: "release architecture", References: []string{"widget"}}
	b := planBranding(pkg)
	style := styleFor(c, b)
	prompt := buildPrompt(c, style, b, textPlaceholders(c))
	if !strings.Contains(prompt, "16:9") {
		t.Errorf("thumbnail prompt missing aspect ratio: %q", prompt)
	}
	if !strings.Contains(strings.ToLower(prompt), "event-driven pipeline") {
		t.Errorf("thumbnail prompt not grounded in subject: %q", prompt)
	}
	if !strings.Contains(strings.ToLower(prompt), "no text") {
		t.Errorf("thumbnail prompt should forbid literal text: %q", prompt)
	}
}

func TestNegativePromptsByCategory(t *testing.T) {
	pkg := samplePackage()
	ill := planNegative(candidate{Category: catIllustration, Focus: "AWS service flow"}, pkg)
	if !strings.Contains(ill, "distorted diagrams") {
		t.Errorf("illustration negatives missing diagram guard: %q", ill)
	}
	// Unrelated AWS services (not in the context) are excluded.
	if !strings.Contains(ill, "unrelated AWS services") {
		t.Errorf("expected unrelated-AWS exclusion: %q", ill)
	}
	// Universal negatives always present.
	if !strings.Contains(ill, "no watermarks") {
		t.Errorf("universal negatives missing: %q", ill)
	}
}

func TestMetadataAndFilename(t *testing.T) {
	pkg := samplePackage()
	meta := planAssetMeta(candidate{Type: "YouTube Thumbnail", Category: catThumbnail, Focus: "architecture"}, pkg)
	if meta.VisualComplexity != "medium" || meta.RecommendedFilename == "" {
		t.Errorf("meta = %+v", meta)
	}
	if !strings.HasSuffix(meta.RecommendedFilename, ".png") || !strings.Contains(meta.RecommendedFilename, "widget") {
		t.Errorf("filename not grounded: %q", meta.RecommendedFilename)
	}
	// Illustration is high-complexity.
	if planAssetMeta(candidate{Type: "Architecture Illustration", Category: catIllustration}, pkg).VisualComplexity != "high" {
		t.Error("illustration should be high complexity")
	}
}

func TestMarkdownRenders(t *testing.T) {
	col, _ := newGen().VisualAssets(context.Background(), samplePackage())
	md := col.Markdown()
	for _, want := range []string{
		"# Visual Assets", "## Brand Guidelines", "### Prompt", "### Negative Prompt",
		"### Composition Notes", "## Collection Intelligence", "render no text",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "A polished, vivid, grounded image prompt with no text rendered, 16:9.", nil
}

func TestModelPolishesPrompt(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	col, err := g.VisualAssets(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	if fm.calls != len(col.Assets) {
		t.Errorf("model called %d times, want %d (one per asset)", fm.calls, len(col.Assets))
	}
	if !strings.Contains(col.Assets[0].Prompt, "polished") {
		t.Errorf("prompt not taken from model: %q", col.Assets[0].Prompt)
	}
}

func TestRequiresContext(t *testing.T) {
	if _, err := newGen().VisualAssets(context.Background(), ReleasePackage{}); err == nil {
		t.Error("expected error for nil context")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	pkg := samplePackage()
	bad := VisualAssetCollection{
		SchemaVersion: "1.0.0",
		Metadata:      Metadata{AssetCount: 1},
		Assets: []Asset{{
			ID: 1, Type: "Mystery", Platform: "", AspectRatio: "",
			Prompt:     "A render featuring Amazon Redshift and Amazon SageMaker.", // ungrounded services
			References: []string{"not-grounded"},
			Style:      Style{},
		}},
	}
	probs := bad.Validate(pkg)
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"missing platform", "missing aspect ratio", "missing style guidance", "no reference is grounded", "unsupported AWS service"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}

func TestValidateAllowsGroundedAWS(t *testing.T) {
	// A prompt naming a service the release DOES use must not be flagged.
	pkg := samplePackage()
	used := contextAWSSet(pkg)
	if got := ungroundedAWSMentions("An AWS Lambda and Amazon SQS workflow.", used); len(got) != 0 {
		t.Errorf("grounded services flagged as unsupported: %v", got)
	}
}

func TestValidateRejectsDuplicates(t *testing.T) {
	pkg := samplePackage()
	col, _ := newGen().VisualAssets(context.Background(), pkg)
	col.Assets = append(col.Assets, col.Assets[0])
	col.Metadata.AssetCount = len(col.Assets)
	if !strings.Contains(strings.Join(col.Validate(pkg), "\n"), "duplicate") {
		t.Error("expected duplicate asset to be rejected")
	}
}
