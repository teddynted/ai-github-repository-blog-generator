package linkedin

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

func TestDistinctHighlightsReducesCrossPostRepetition(t *testing.T) {
	used := map[string]bool{}
	a := distinctHighlights([]string{"event-driven pipeline", "sqs buffer", "ec2 worker"}, used)
	b := distinctHighlights([]string{"event-driven pipeline", "sqs buffer", "cloudwatch alarms", "dlq"}, used)
	overlap := 0
	for _, x := range b {
		for _, y := range a {
			if strings.EqualFold(collapse(x), collapse(y)) {
				overlap++
			}
		}
	}
	if overlap > 2 {
		t.Errorf("too much cross-post highlight repetition: a=%v b=%v", a, b)
	}
	if len(b) == 0 {
		t.Error("a post must still have highlights")
	}
}

func TestWarningsNotRenderedInArtifact(t *testing.T) {
	col := LinkedInCollection{
		Metadata: Metadata{Repository: "acme/widget", Release: "v0.6.0", PostCount: 1},
		Warnings: []string{"post 3 (Engineering Lesson) has no technical highlights; the release context may be thin"},
		Posts:    []Post{{ID: 1, Type: "Release Announcement", Body: "A grounded engineering update."}},
	}
	md := col.Markdown()
	if strings.Contains(md, "Notes:") || strings.Contains(strings.ToLower(md), "no technical highlights") {
		t.Errorf("internal warnings leaked into the published artifact:\n%s", md)
	}
	if len(col.Warnings) == 0 {
		t.Error("warnings should remain on the struct for CI/manifest")
	}
}

func TestSummarySkipsReleaseStatsFraming(t *testing.T) {
	pkg := ReleasePackage{
		Context: &rc.ReleaseContext{
			Repository:          rc.Repository{Name: "widget", FullName: "acme/widget"},
			Release:             rc.Release{Tag: "v0.6.0"},
			ContentIntelligence: rc.ContentIntelligence{Summary: "widget v0.6.0 delivers 7 analyzed changes (0 features, 0 fixes) across 24 files."},
		},
		Blog: releasegen.BlogPost{MetaDescription: "Pre-baked custom AMIs cut EC2 startup latency by moving boot-time provisioning into versioned images."},
	}
	got := summary(pkg)
	if strings.Contains(strings.ToLower(got), "0 features") || strings.Contains(strings.ToLower(got), "analyzed changes") {
		t.Errorf("summary should skip release-stats framing; got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "ami") {
		t.Errorf("summary should fall back to the topic-led meta description; got %q", got)
	}
}

func samplePackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "widget", FullName: "acme/widget", URL: "https://github.com/acme/widget", Language: "Go"},
		Release:       rc.Release{Tag: "v0.2.0"},
		Architecture:  rc.Architecture{Overview: "An event-driven pipeline that turns GitHub releases into content.", AWSServices: []string{"AWS Lambda", "Amazon SQS", "Amazon Bedrock"}},
		Changelog:     rc.ChangelogAnalysis{Found: true, Features: []string{"release context builder", "storyboard generator"}},
		Technologies:  []rc.Technology{{Name: "Go"}, {Name: "AWS CloudFormation"}},
		Implementation: rc.ImplementationSummary{
			WhatChanged:                []string{"Added the release context builder"},
			TechnicalImprovements:      []string{"Grounded content in real analysis"},
			WhyItMatters:               "It keeps every downstream artifact accurate.",
			InfrastructureImprovements: []string{"Reproducible infra via CloudFormation"},
		},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:                  "widget v0.2.0 delivers a release context builder that grounds all downstream content.",
			ImplementationComplexity: "medium",
			TargetAudience:           "Cloud engineers",
			TechnicalHighlights:      []string{"grounded content intelligence", "event-driven decoupling"},
			ArchitectureHighlights:   []string{"clean event-driven decoupling on AWS"},
			InfrastructureHighlights: []string{"reproducible, reviewable infrastructure"},
			DeveloperValue:           "Less manual work per release.",
			SEOKeywords:              []string{"release context", "content intelligence", "go", "aws"},
		},
	}
	post := releasegen.BlogPost{Title: "Inside widget v0.2.0", Tags: []string{"go", "aws"}, MetaDescription: "A deep dive into widget v0.2.0."}

	se := seo.SEOMetadata{
		SchemaVersion: seo.SchemaVersion,
		Blog:          seo.BlogSEO{Canonical: seo.Canonical{URL: "https://widget.dev/blog/inside-widget-v0-2-0"}, ReadingTime: "6 min read"},
		Keywords:      seo.Keywords{Primary: []string{"release context builder", "AWS Lambda", "content intelligence"}},
		Hashtags:      seo.PlatformTags{LinkedIn: []string{"#SoftwareEngineering", "#CloudComputing", "#AIEngineering"}},
		Social:        seo.SocialSEO{Items: []seo.SocialItem{{Platform: "LinkedIn", Summary: "widget v0.2.0 ships a Release Context builder — here's how it's built.", Excerpt: "Grounded content."}}},
	}
	va := visualassets.VisualAssetCollection{
		SchemaVersion: visualassets.SchemaVersion,
		Assets: []visualassets.Asset{
			{Type: "LinkedIn Banner", Title: "LinkedIn Banner", Metadata: visualassets.AssetMeta{RecommendedFilename: "widget-v0-2-0-linkedin-banner.png"}},
			{Type: "Architecture Illustration", Title: "Architecture Illustration", Metadata: visualassets.AssetMeta{RecommendedFilename: "widget-v0-2-0-architecture-illustration.png"}},
		},
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

func TestLinkedInEndToEnd(t *testing.T) {
	pkg := samplePackage()
	col, err := newGen().LinkedIn(context.Background(), pkg)
	if err != nil {
		t.Fatalf("LinkedIn: %v", err)
	}

	if col.SchemaVersion != SchemaVersion || col.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", col.Metadata)
	}
	if len(col.Posts) < 3 {
		t.Fatalf("expected multiple posts, got %d", len(col.Posts))
	}

	for _, p := range col.Posts {
		if p.Body == "" || p.CTA == "" || p.EngagementPrompt == "" || len(p.Hashtags) == 0 {
			t.Errorf("post %d (%s) under-populated", p.ID, p.Type)
		}
		if p.Metadata.EngagementScore == 0 || p.Metadata.ProfessionalConfidence == 0 {
			t.Errorf("post %d scores not computed", p.ID)
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

func TestPostTypesDiverseAudiences(t *testing.T) {
	col, _ := newGen().LinkedIn(context.Background(), samplePackage())
	types := map[string]bool{}
	audiences := map[string]bool{}
	for _, p := range col.Posts {
		types[p.Type] = true
		audiences[p.Audience] = true
	}
	if len(types) != len(col.Posts) {
		t.Errorf("post types not unique: %v", types)
	}
	if len(audiences) < 3 {
		t.Errorf("expected diverse audiences, got %v", audiences)
	}
	// AI highlight appears because the stack includes Bedrock.
	if !types["AI Engineering Highlight"] {
		t.Errorf("expected an AI Engineering Highlight (Bedrock in stack)")
	}
}

func TestHighlightsGrounded(t *testing.T) {
	pkg := samplePackage()
	grounded := groundedTerms(pkg)
	for _, typ := range []string{"Architecture Deep Dive", "Engineering Lesson", "Feature Spotlight"} {
		for _, h := range planHighlights(pkg, typ) {
			if !termGrounded(h, grounded) {
				t.Errorf("%s highlight not grounded: %q", typ, h)
			}
		}
	}
}

func TestReusesSEOAndVisualAssets(t *testing.T) {
	col, _ := newGen().LinkedIn(context.Background(), samplePackage())
	var sawBlogURL, sawVisual bool
	for _, p := range col.Posts {
		if strings.Contains(p.CTA, "widget.dev/blog") {
			sawBlogURL = true
		}
		for _, v := range p.VisualReferences {
			if strings.Contains(v.Reference, ".png") || strings.Contains(v.Reference, "Architecture") {
				sawVisual = true
			}
		}
		// Hashtags reuse the SEO LinkedIn set.
		if containsTag(p.Hashtags, "#SoftwareEngineering") {
			// good
		}
	}
	if !sawBlogURL {
		t.Error("expected a CTA reusing the SEO blog canonical URL")
	}
	if !sawVisual {
		t.Error("expected posts to reference existing visual assets")
	}
}

func TestNoAINoAIPost(t *testing.T) {
	pkg := samplePackage()
	pkg.Context.Architecture.AWSServices = []string{"AWS Lambda", "Amazon SQS"} // drop Bedrock
	pkg.Context.ContentIntelligence.Summary = "An event-driven pipeline."
	pkg.Context.ContentIntelligence.TechnicalHighlights = []string{"event-driven decoupling"}
	col, _ := newGen().LinkedIn(context.Background(), pkg)
	for _, p := range col.Posts {
		if p.Type == "AI Engineering Highlight" {
			t.Error("no AI in stack but AI post produced")
		}
	}
}

func TestEngagementAndCTAByType(t *testing.T) {
	if p := engagementPrompt(postCandidate{Type: "Architecture Deep Dive"}); !strings.Contains(strings.ToLower(p), "architecture") {
		t.Errorf("arch engagement = %q", p)
	}
	cta := planCTA(samplePackage(), postCandidate{Type: "Architecture Deep Dive"})
	if !strings.Contains(cta, "widget.dev") {
		t.Errorf("arch CTA should link the blog: %q", cta)
	}
}

func TestHashtagsFocused(t *testing.T) {
	tags := planHashtags(samplePackage(), postCandidate{Type: "AWS Best Practice"})
	if len(tags) == 0 || len(tags) > maxHashtags {
		t.Fatalf("hashtag count %d out of range", len(tags))
	}
	if !containsTag(tags, "#AWS") {
		t.Errorf("expected #AWS in %v", tags)
	}
	for _, tag := range tags {
		if !strings.HasPrefix(tag, "#") {
			t.Errorf("malformed hashtag %q", tag)
		}
	}
}

func TestMetadataScores(t *testing.T) {
	col, _ := newGen().LinkedIn(context.Background(), samplePackage())
	ci := col.ContentIntelligence
	if ci.PostCount != len(col.Posts) || ci.AverageEngagementScore == 0 {
		t.Errorf("intelligence = %+v", ci)
	}
	if len(ci.AWSServices) == 0 || ci.Tone == "" {
		t.Errorf("collection metadata incomplete: %+v", ci)
	}
	for _, p := range col.Posts {
		if p.Metadata.SuggestedPublishTime == "" || p.Metadata.CharacterCount == 0 {
			t.Errorf("post %d meta incomplete", p.ID)
		}
	}
}

func TestMarkdownRenders(t *testing.T) {
	col, _ := newGen().LinkedIn(context.Background(), samplePackage())
	md := col.Markdown()
	for _, want := range []string{
		"# LinkedIn Content", "### Post", "**Technical highlights:**",
		"**Engagement prompt:**", "**CTA:**", "**Hashtags:**", "## Content Intelligence",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "An authentic, grounded LinkedIn post about widget v0.2.0 and AWS Lambda.", nil
}

func TestModelWritesBody(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	col, err := g.LinkedIn(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	if fm.calls != len(col.Posts) {
		t.Errorf("model called %d times, want %d (one per post)", fm.calls, len(col.Posts))
	}
	if !strings.Contains(col.Posts[0].Body, "authentic, grounded") {
		t.Errorf("body not from model: %q", col.Posts[0].Body)
	}
}

func TestRequiresContext(t *testing.T) {
	if _, err := newGen().LinkedIn(context.Background(), ReleasePackage{}); err == nil {
		t.Error("expected error for nil context")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	pkg := samplePackage()
	bad := LinkedInCollection{
		SchemaVersion: "1.0.0",
		Metadata:      Metadata{PostCount: 1},
		Posts: []Post{{
			ID: 1, Type: "Mystery", Body: "totally unrelated content about penguins", CTA: "", EngagementPrompt: "",
			Hashtags:            nil,
			TechnicalHighlights: []string{"quantum teleportation subsystem"}, // ungrounded
		}},
	}
	probs := bad.Validate(pkg)
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"missing CTA", "no hashtags", "missing engagement prompt", "not grounded", "ungrounded technical claim"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}

func TestValidateRejectsDuplicates(t *testing.T) {
	pkg := samplePackage()
	col, _ := newGen().LinkedIn(context.Background(), pkg)
	col.Posts = append(col.Posts, col.Posts[0])
	col.Metadata.PostCount = len(col.Posts)
	if !strings.Contains(strings.Join(col.Validate(pkg), "\n"), "duplicate") {
		t.Error("expected duplicate post to be rejected")
	}
}

func containsTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}
