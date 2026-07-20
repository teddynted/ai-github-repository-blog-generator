package xthread

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/seo"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/visualassets"
)

func samplePackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "widget", FullName: "acme/widget", URL: "https://github.com/acme/widget", Language: "Go"},
		Release:       rc.Release{Tag: "v0.2.0"},
		Architecture:  rc.Architecture{Overview: "An event-driven pipeline that turns GitHub releases into content.", AWSServices: []string{"AWS Lambda", "Amazon SQS", "Amazon Bedrock"}},
		Changelog:     rc.ChangelogAnalysis{Found: true, Features: []string{"release context builder", "storyboard generator"}},
		Technologies:  []rc.Technology{{Name: "Go"}},
		Implementation: rc.ImplementationSummary{
			WhatChanged:                []string{"Added the release context builder"},
			TechnicalImprovements:      []string{"Grounded content in real analysis"},
			WhyItMatters:               "It keeps every downstream artifact accurate.",
			InfrastructureImprovements: []string{"Reproducible infra via CloudFormation"},
			DeveloperExperience:        []string{"One command generates the whole content set"},
		},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:                  "widget v0.2.0 delivers a release context builder that grounds all downstream content.",
			ImplementationComplexity: "medium",
			TargetAudience:           "Cloud engineers",
			TechnicalHighlights:      []string{"grounded content intelligence", "event-driven decoupling"},
			ArchitectureHighlights:   []string{"clean event-driven decoupling on AWS"},
			InfrastructureHighlights: []string{"reproducible, reviewable infrastructure"},
			SEOKeywords:              []string{"release context", "content intelligence", "go", "aws"},
		},
	}
	post := releasegen.BlogPost{
		Title:    "Inside widget v0.2.0",
		Tags:     []string{"go", "aws"},
		Markdown: "# Inside widget\n\nThis release adds a Release Context builder.\n\n```go\nfunc (b *Builder) Build() error { return assemble() }\n```\n\n```mermaid\nflowchart TD\nA-->B\n```\n",
	}
	se := seo.SEOMetadata{
		SchemaVersion: seo.SchemaVersion,
		Blog:          seo.BlogSEO{Canonical: seo.Canonical{URL: "https://widget.dev/blog/inside-widget-v0-2-0"}},
		Keywords:      seo.Keywords{Primary: []string{"release context builder", "AWS Lambda"}},
		Hashtags:      seo.PlatformTags{X: []string{"#DevOps", "#AWS"}},
	}
	va := visualassets.VisualAssetCollection{
		SchemaVersion: visualassets.SchemaVersion,
		Assets:        []visualassets.Asset{{Type: "X Image", Title: "X Image", Metadata: visualassets.AssetMeta{RecommendedFilename: "widget-v0-2-0-x-image.png"}}},
	}
	ar := architecture.ArchitectureCollection{
		SchemaVersion: architecture.SchemaVersion,
		Diagrams:      []architecture.Diagram{{ID: 1, Type: "High-Level Architecture", Title: "widget v0.2.0 — High-Level AWS Architecture"}},
	}
	return ReleasePackage{Context: ctx, Blog: post, SEO: se, VisualAssets: va, Architecture: ar}
}

func newGen() *Generator {
	return &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
}

func TestXThreadEndToEnd(t *testing.T) {
	pkg := samplePackage()
	col, err := newGen().XThread(context.Background(), pkg)
	if err != nil {
		t.Fatalf("XThread: %v", err)
	}

	if col.SchemaVersion != SchemaVersion || col.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", col.Metadata)
	}
	if len(col.Threads) < 3 {
		t.Fatalf("expected multiple threads, got %d", len(col.Threads))
	}

	for _, th := range col.Threads {
		if len(th.Posts) < minPosts || th.CTA == "" || th.EngagementPrompt == "" || len(th.Hashtags) == 0 || len(th.KeyTakeaways) == 0 {
			t.Errorf("thread %d (%s) under-populated", th.ID, th.Type)
		}
		// Every post respects the character limit and is non-empty.
		for _, p := range th.Posts {
			if p.Content == "" {
				t.Errorf("thread %d post %d empty", th.ID, p.Index)
			}
			if runeLen(p.Content) > MaxPostChars {
				t.Errorf("thread %d post %d = %d chars > %d", th.ID, p.Index, runeLen(p.Content), MaxPostChars)
			}
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

func TestConfigurableThreadLength(t *testing.T) {
	for _, n := range []int{3, 7, 10} {
		g := &Generator{PostsPerThread: n, Now: newGen().Now}
		col, _ := g.XThread(context.Background(), samplePackage())
		if col.Metadata.PostsPerThread != n {
			t.Errorf("postsPerThread = %d, want %d", col.Metadata.PostsPerThread, n)
		}
		for _, th := range col.Threads {
			if len(th.Posts) > n {
				t.Errorf("thread %d has %d posts, want <= %d", th.ID, len(th.Posts), n)
			}
		}
	}
	// Out-of-range lengths clamp.
	g := &Generator{PostsPerThread: 99, Now: newGen().Now}
	col, _ := g.XThread(context.Background(), samplePackage())
	if col.Metadata.PostsPerThread != maxPostsPerThread {
		t.Errorf("length not clamped: %d", col.Metadata.PostsPerThread)
	}
}

func TestThreadTypesDiverse(t *testing.T) {
	col, _ := newGen().XThread(context.Background(), samplePackage())
	types := map[string]bool{}
	for _, th := range col.Threads {
		types[th.Type] = true
	}
	if len(types) != len(col.Threads) {
		t.Errorf("thread types not unique: %v", types)
	}
	if !types["AI Engineering Insights"] {
		t.Errorf("expected AI Engineering Insights (Bedrock in stack)")
	}
}

func TestCodeSnippetGroundedNotFabricated(t *testing.T) {
	pkg := samplePackage()
	snip := extractSnippet(pkg)
	if !strings.Contains(snip, "Builder") {
		t.Errorf("expected grounded snippet from the blog, got %q", snip)
	}
	// A blog with no code yields no snippet (never fabricated).
	pkg.Blog.Markdown = "# No code here\n\nJust prose."
	if s := extractSnippet(pkg); s != "" {
		t.Errorf("expected no snippet, got %q", s)
	}
	// A code-focused thread (Feature Breakdown / Implementation Deep Dive)
	// carries the grounded snippet on one of its posts.
	col, _ := newGen().XThread(context.Background(), samplePackage())
	var found bool
	for _, th := range col.Threads {
		for _, p := range th.Posts {
			if strings.Contains(p.CodeSnippet, "Builder") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a code-focused thread to carry the grounded snippet")
	}
}

func TestKeyTakeawaysGrounded(t *testing.T) {
	pkg := samplePackage()
	grounded := groundedTerms(pkg)
	for _, typ := range []string{"Architecture Walkthrough", "Engineering Lessons Learned", "Feature Breakdown"} {
		for _, k := range planTakeaways(pkg, typ) {
			if !termGrounded(k, grounded) {
				t.Errorf("%s takeaway not grounded: %q", typ, k)
			}
		}
	}
}

func TestReusesSEOAndVisuals(t *testing.T) {
	col, _ := newGen().XThread(context.Background(), samplePackage())
	var sawURL, sawVisual bool
	for _, th := range col.Threads {
		if strings.Contains(th.CTA, "widget.dev") || strings.Contains(th.CTA, "github.com/acme/widget") {
			sawURL = true
		}
		for _, v := range th.VisualReferences {
			if v.Reference != "" {
				sawVisual = true
			}
		}
	}
	if !sawURL {
		t.Error("expected CTAs reusing the SEO/repo URLs")
	}
	if !sawVisual {
		t.Error("expected threads to reference existing visual assets")
	}
}

func TestHashtagsMinimal(t *testing.T) {
	tags := planHashtags(samplePackage(), threadCandidate{Type: "AWS Best Practices"})
	if len(tags) == 0 || len(tags) > maxHashtags {
		t.Fatalf("hashtag count %d out of range (max %d)", len(tags), maxHashtags)
	}
	for _, tag := range tags {
		if !strings.HasPrefix(tag, "#") {
			t.Errorf("malformed hashtag %q", tag)
		}
	}
}

func TestEngagementAndCTAByType(t *testing.T) {
	if p := engagementPrompt(threadCandidate{Type: "Architecture Walkthrough"}); !strings.Contains(strings.ToLower(p), "architecture") {
		t.Errorf("arch engagement = %q", p)
	}
	cta := planCTA(samplePackage(), threadCandidate{Type: "Architecture Walkthrough"})
	if !strings.Contains(cta, "widget.dev") {
		t.Errorf("arch CTA should link the blog: %q", cta)
	}
}

func TestMetadataScores(t *testing.T) {
	col, _ := newGen().XThread(context.Background(), samplePackage())
	ci := col.ContentIntelligence
	if ci.ThreadCount != len(col.Threads) || ci.AverageEngagementScore == 0 {
		t.Errorf("intelligence = %+v", ci)
	}
	for _, th := range col.Threads {
		if th.Metadata.SuggestedPostingTime == "" || th.Metadata.TotalCharacters == 0 {
			t.Errorf("thread %d meta incomplete", th.ID)
		}
	}
}

func TestMarkdownRenders(t *testing.T) {
	col, _ := newGen().XThread(context.Background(), samplePackage())
	md := col.Markdown()
	for _, want := range []string{
		"# X Threads", "**Post 1/", "**Key takeaways:**", "**Engagement prompt:**",
		"**CTA:**", "**Hashtags:**", "## Content Intelligence",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "🚀 widget v0.2.0 is out — a grounded thread on AWS Lambda and the release context builder. 🧵", nil
}

func TestModelSharpensHook(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	col, err := g.XThread(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	if fm.calls != len(col.Threads) { // one hook per thread
		t.Errorf("model called %d times, want %d", fm.calls, len(col.Threads))
	}
	if !strings.Contains(col.Threads[0].Posts[0].Content, "grounded thread") {
		t.Errorf("hook not from model: %q", col.Threads[0].Posts[0].Content)
	}
}

func TestModelOverlongHookTruncated(t *testing.T) {
	g := &Generator{Model: longModel{}, Now: newGen().Now}
	col, _ := g.XThread(context.Background(), samplePackage())
	if l := runeLen(col.Threads[0].Posts[0].Content); l > MaxPostChars {
		t.Errorf("overlong hook not truncated: %d chars", l)
	}
}

type longModel struct{}

func (longModel) Generate(_ context.Context, _ string) (string, error) {
	return strings.Repeat("very long hook post ", 40), nil
}

func TestRequiresContext(t *testing.T) {
	if _, err := newGen().XThread(context.Background(), ReleasePackage{}); err == nil {
		t.Error("expected error for nil context")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	pkg := samplePackage()
	bad := XThreadCollection{
		SchemaVersion: "1.0.0",
		Metadata:      Metadata{ThreadCount: 1},
		Threads: []Thread{{
			ID: 1, Type: "Mystery", CTA: "", EngagementPrompt: "", Hashtags: nil, KeyTakeaways: []string{"quantum teleportation module"},
			Posts: []ThreadPost{{Index: 1, Content: strings.Repeat("x", 300)}}, // over the char limit
		}},
	}
	probs := bad.Validate(pkg)
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"missing CTA", "no hashtags", "missing engagement prompt", "exceeds the 280 limit", "ungrounded technical claim"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}

func TestValidateRejectsDuplicates(t *testing.T) {
	pkg := samplePackage()
	col, _ := newGen().XThread(context.Background(), pkg)
	col.Threads = append(col.Threads, col.Threads[0])
	col.Metadata.ThreadCount = len(col.Threads)
	if !strings.Contains(strings.Join(col.Validate(pkg), "\n"), "duplicate") {
		t.Error("expected duplicate thread to be rejected")
	}
}
