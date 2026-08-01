package seo

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
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

func samplePackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "widget", FullName: "acme/widget", URL: "https://github.com/acme/widget", Homepage: "https://widget.dev", Language: "Go"},
		Release:       rc.Release{Tag: "v0.2.0", PublishedAt: "2026-07-18T20:00:00Z"},
		Architecture:  rc.Architecture{Overview: "An event-driven pipeline that turns GitHub releases into content.", AWSServices: []string{"AWS Lambda", "Amazon SQS"}, Components: []rc.ArchitectureComponent{{Name: "Builder"}}},
		CommitStats:   rc.CommitStats{Analyzed: 9, Total: 9},
		FileStats:     rc.FileStats{Total: 12},
		Changelog:     rc.ChangelogAnalysis{Found: true, Features: []string{"release context builder"}},
		Technologies:  []rc.Technology{{Name: "Go"}, {Name: "AWS CloudFormation"}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:                  "widget v0.2.0 delivers a release context builder that grounds all downstream content.",
			ImplementationComplexity: "medium",
			TargetAudience:           "Cloud engineers",
			SEOKeywords:              []string{"release context", "content intelligence", "go"},
			TechnicalHighlights:      []string{"grounded content intelligence"},
			LinkedInPost:             "We shipped a Release Context builder. It grounds every downstream artifact in real analysis.",
		},
	}
	post := releasegen.BlogPost{
		Title:           "Inside widget v0.2.0: Release Context Builder",
		MetaDescription: "A deep dive into widget v0.2.0 and its event-driven Release Context builder on AWS Lambda and Amazon SQS.",
		Tags:            []string{"go", "aws", "serverless"},
		Markdown:        "---\ntitle: x\n---\n# Inside widget\n\nThis release adds a Release Context builder. It grounds all downstream content in real analysis. The system is event-driven on AWS Lambda and Amazon SQS.\n\n## More\n\nThe builder is a pure Go package behind a Sources port, which keeps it testable and accurate.",
		WordCount:       60,
	}
	yt := youtube.YouTubeScript{
		SchemaVersion: youtube.SchemaVersion,
		Metadata:      youtube.Metadata{Repository: "acme/widget", Release: "v0.2.0"},
		Video:         youtube.Video{Title: "Inside widget v0.2.0", Duration: "12:40", DurationSec: 760},
		ContentIntelligence: youtube.Intelligence{
			SuggestedTitle:       "Inside widget v0.2.0: How the Release Context Builder Works",
			AlternativeTitles:    []string{"How widget v0.2.0 Actually Works"},
			SuggestedDescription: "A full walkthrough of widget v0.2.0 and its event-driven architecture.",
			SuggestedTags:        []string{"aws", "go", "serverless", "architecture"},
			SuggestedPlaylist:    "widget — Release Deep Dives",
			SuggestedThumbnail:   "v0.2.0 · AWS Lambda",
			PinnedComment:        "📌 Everything here is generated from the repo's Release Context.",
			Chapters:             []youtube.ChapterMarker{{Timestamp: "00:00", Title: "Intro"}, {Timestamp: "02:40", Title: "Architecture"}},
		},
	}
	sh := shorts.ShortsCollection{
		SchemaVersion: shorts.SchemaVersion,
		Shorts: []shorts.Short{{
			ID: 1, Angle: "Architecture Reveal", Title: "This architecture in under a minute",
			Hook: "This is the whole architecture in one shot.", CoreExplanation: "An event-driven pipeline.",
			CTA: "Watch the full video.", Hashtags: []string{"#AWS", "#SystemDesign"},
			SEO: shorts.SEO{Description: "Architecture in 45s"},
		}},
	}
	tt := tiktok.TikTokCollection{
		SchemaVersion: tiktok.SchemaVersion,
		Videos: []tiktok.Video{{
			ID: 1, Topic: "Architecture Insight", Title: "The architecture nobody explains",
			Hook: "This one decision changed everything.", Takeaway: "Clean boundaries pay off.",
			EngagementPrompt: "Would you design it differently?", CTA: "Full video in bio.",
			Hashtags: []string{"#TechTok", "#AWS"}, SEO: tiktok.SEO{Caption: "Architecture explained fast."},
		}},
	}
	return ReleasePackage{Context: ctx, Blog: post, YouTube: yt, Shorts: sh, TikTok: tt}
}

func newGen() *Generator {
	return &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
}

func TestSEOEndToEnd(t *testing.T) {
	pkg := samplePackage()
	m, err := newGen().SEO(context.Background(), pkg)
	if err != nil {
		t.Fatalf("SEO: %v", err)
	}

	if m.SchemaVersion != SchemaVersion || m.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", m.Metadata)
	}
	// Every channel is populated.
	if m.Blog.Title == "" || m.Blog.MetaDescription == "" || m.Blog.Slug == "" {
		t.Errorf("blog SEO incomplete: %+v", m.Blog)
	}
	if m.YouTube.Title == "" || len(m.YouTube.ChapterTitles) == 0 {
		t.Errorf("youtube SEO incomplete: %+v", m.YouTube)
	}
	if len(m.Shorts.Items) < 2 { // one shorts + one tiktok
		t.Errorf("shorts SEO items = %d", len(m.Shorts.Items))
	}
	if len(m.Social.Items) < 4 {
		t.Errorf("social SEO items = %d", len(m.Social.Items))
	}
	if len(m.Keywords.Primary) == 0 || m.OpenGraph.Title == "" || m.StructuredData.JSONLD == nil {
		t.Errorf("keywords/OG/structured incomplete")
	}
	if m.ContentIntelligence.SEOConfidenceScore == 0 {
		t.Errorf("confidence score not computed")
	}

	if probs := m.Validate(pkg); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
	blob, err := json.Marshal(m)
	if err != nil || !strings.Contains(string(blob), "\"schemaVersion\":\"1.0.0\"") {
		t.Errorf("marshal: %v", err)
	}
}

func TestDedupeTagsAcrossFormats(t *testing.T) {
	in := []string{"amazon-cloudwatch", "amazon cloudwatch", "aws-iam", "AWS IAM", "go", "GitHub Actions", "github-actions"}
	got := dedupeTags(in)
	// One entry per normalized (lowercase, hyphen==space) term.
	if len(got) != 4 {
		t.Fatalf("expected 4 unique tags, got %d: %v", len(got), got)
	}
	// First-occurrence wins.
	if got[0] != "amazon-cloudwatch" || got[1] != "aws-iam" {
		t.Errorf("unexpected order/form: %v", got)
	}
}

func TestDescLeadsWithKeyword(t *testing.T) {
	primary := []string{"aws spot startup", "custom ami"}
	if !descLeadsWithKeyword("Optimizing AWS Spot startup with custom AMIs on EC2.", primary) {
		t.Error("expected lead keyword to be detected")
	}
	// Keyword only appears past the 150-char lead → not front-loaded.
	late := strings.Repeat("x", 160) + " aws spot startup"
	if descLeadsWithKeyword(late, primary) {
		t.Error("keyword past 150 chars should not count as front-loaded")
	}
	if descLeadsWithKeyword("Generic release notes for v0.6.0.", primary) {
		t.Error("no primary keyword present should be false")
	}
}

func TestMainEntityOfPageIsObject(t *testing.T) {
	blog := BlogSEO{Title: "T", MetaDescription: "D", Canonical: Canonical{URL: "https://example.com/blog/x"}}
	sd := planStructuredData(samplePackage(), blog, Keywords{Primary: []string{"aws"}}, "2026-01-01T00:00:00Z")
	me, ok := sd.JSONLD["mainEntityOfPage"].(map[string]interface{})
	if !ok || me["@type"] != "WebPage" || me["@id"] != "https://example.com/blog/x" {
		t.Errorf("mainEntityOfPage should be a WebPage object, got %v", sd.JSONLD["mainEntityOfPage"])
	}
}

func TestValidationReportRendered(t *testing.T) {
	m, _ := newGen().SEO(context.Background(), samplePackage())
	md := m.Markdown()
	for _, want := range []string{
		"## Validation Report", "Blog title length:", "Meta description length:",
		"YouTube title length:", "Duplicate chapter timestamps:", "JSON-LD valid:",
		"Canonical URL present:", "OG/Twitter parity:",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("validation report missing %q", want)
		}
	}
}

func TestDropGenericKeywords(t *testing.T) {
	got := dropGeneric([]string{"aws-iam", "agent", "ai", "this release", "serverless", "release"})
	want := []string{"aws-iam", "serverless"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("dropGeneric = %v, want %v", got, want)
	}
	if isGenericKeyword("AWS Lambda") {
		t.Error("AWS Lambda should not be generic")
	}
}

func TestDedupePlaylists(t *testing.T) {
	in := []string{"teddynted/repo — Release Deep Dives", "Release Deep Dives", "AWS & Cloud Engineering"}
	got := dedupePlaylists(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 playlists after dedup, got %d: %v", len(got), got)
	}
	// The bare "Release Deep Dives" duplicate is gone; the qualified one and the
	// distinct series remain.
	if got[0] != "teddynted/repo — Release Deep Dives" || got[1] != "AWS & Cloud Engineering" {
		t.Errorf("unexpected playlists: %v", got)
	}
}

func TestShortenYouTubeTitle(t *testing.T) {
	long := "Optimizing Spot Startup on AWS: Pre-Baked Custom AMIs for an Event-Driven AI Agent Platform"
	got := shortenYouTubeTitle(long)
	if got != "Optimizing Spot Startup on AWS" {
		t.Errorf("colon title should use the headline, got %q", got)
	}
	if len(got) > YouTubeTitlePref {
		t.Errorf("title %d chars exceeds preferred %d", len(got), YouTubeTitlePref)
	}
	// No colon → word-boundary truncate within the preferred length.
	noColon := strings.Repeat("word ", 30)
	if s := shortenYouTubeTitle(noColon); len(s) > YouTubeTitlePref {
		t.Errorf("no-colon title not truncated: %d chars", len(s))
	}
	// Already short → unchanged.
	if shortenYouTubeTitle("Short title") != "Short title" {
		t.Error("short title should be unchanged")
	}
}

func TestSlugNoDoubleReleaseForFeaturelessRelease(t *testing.T) {
	pkg := samplePackage()
	// Featureless (maintenance) release → featureName returns "this release".
	pkg.Context.Changelog = rc.ChangelogAnalysis{Found: true}
	pkg.Context.Implementation.WhatChanged = nil
	pkg.Blog.Title = "" // force the feature-based path
	_, alts := planSlug(pkg)
	for _, a := range alts {
		if strings.Contains(a, "release-release") {
			t.Errorf("slug still doubles 'release': %q", a)
		}
		if strings.Contains(a, "this-release") {
			t.Errorf("slug contains placeholder 'this release': %q", a)
		}
	}
}

func TestJSONLDIsValidJSON(t *testing.T) {
	m, _ := newGen().SEO(context.Background(), samplePackage())
	md := m.Markdown()
	// Extract the JSON-LD fenced block.
	i := strings.Index(md, "**JSON-LD:**")
	if i < 0 {
		t.Fatal("no JSON-LD block")
	}
	rest := md[i:]
	open := strings.Index(rest, "```json\n")
	if open < 0 {
		t.Fatal("no json fence")
	}
	body := rest[open+len("```json\n"):]
	body = body[:strings.Index(body, "\n```")]

	var ld map[string]any
	if err := json.Unmarshal([]byte(body), &ld); err != nil {
		t.Fatalf("JSON-LD is not valid JSON: %v\n%s", err, body)
	}
	// author/publisher must be nested objects, not Go map[...] strings.
	if a, ok := ld["author"].(map[string]any); !ok || a["@type"] != "Organization" {
		t.Errorf("author not a proper JSON object: %v", ld["author"])
	}
	for _, k := range []string{"@context", "@type", "headline", "description", "keywords", "datePublished", "dateModified", "wordCount"} {
		if _, ok := ld[k]; !ok {
			t.Errorf("JSON-LD missing %q", k)
		}
	}
}

func TestWarningsNeverLeakIntoMarkdown(t *testing.T) {
	m, _ := newGen().SEO(context.Background(), samplePackage())
	m.Warnings = []string{"YouTube title is 91 chars; ≤ 70 is preferred"}
	if md := m.Markdown(); strings.Contains(md, "**Notes:**") || strings.Contains(md, "91 chars") {
		t.Error("internal QA warning leaked into the viewer artifact")
	}
}

func TestHashifyCanonicalAWSCasing(t *testing.T) {
	cases := map[string]string{
		"aws-iam":            "AWSIAM",
		"aws-lambda":         "AWSLambda",
		"amazon-ec2":         "AmazonEC2",
		"amazon-eventbridge": "AmazonEventBridge",
		"amazon-cloudwatch":  "AmazonCloudWatch",
		"event-driven":       "EventDriven",
		"clean architecture": "CleanArchitecture",
	}
	for in, want := range cases {
		if got := hashify(in); got != want {
			t.Errorf("hashify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBlogMetaWithinLimit(t *testing.T) {
	m, _ := newGen().SEO(context.Background(), samplePackage())
	if l := len(m.Blog.MetaDescription); l > BlogDescMax {
		t.Errorf("meta description %d chars exceeds %d", l, BlogDescMax)
	}
	if l := len(m.YouTube.Title); l > YouTubeTitleMax {
		t.Errorf("youtube title %d chars exceeds %d", l, YouTubeTitleMax)
	}
}

func TestSlugGeneration(t *testing.T) {
	slug, alts := planSlug(samplePackage())
	if !validSlug(slug) {
		t.Errorf("primary slug invalid: %q", slug)
	}
	// Deterministic + readable: lowercase, hyphenated, stop words dropped.
	if strings.Contains(slug, " ") || strings.Contains(slug, "the-") {
		t.Errorf("slug not clean: %q", slug)
	}
	for _, a := range alts {
		if !validSlug(a) {
			t.Errorf("alt slug invalid: %q", a)
		}
		if a == slug {
			t.Errorf("alt slug duplicates primary: %q", a)
		}
	}
	// Determinism.
	slug2, _ := planSlug(samplePackage())
	if slug != slug2 {
		t.Errorf("slug not deterministic: %q vs %q", slug, slug2)
	}
}

func TestValidSlug(t *testing.T) {
	cases := map[string]bool{
		"aws-lambda-release-context": true,
		"github-automation":          true,
		"Bad-Caps":                   false,
		"has space":                  false,
		"under_score":                false,
		"-leading":                   false,
		"double--dash":               false,
	}
	for s, want := range cases {
		if got := validSlug(s); got != want {
			t.Errorf("validSlug(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestKeywordsGroundedAndDeduped(t *testing.T) {
	k := planKeywords(samplePackage())
	if len(k.Primary) == 0 || len(k.AWS) == 0 {
		t.Fatalf("keywords under-populated: %+v", k)
	}
	// AWS keywords come straight from the context.
	joined := strings.ToLower(strings.Join(k.AWS, "|"))
	if !strings.Contains(joined, "lambda") {
		t.Errorf("AWS keywords not grounded: %v", k.AWS)
	}
	// No duplicates in primary.
	if hasDup(k.Primary) {
		t.Errorf("primary keywords contain duplicates: %v", k.Primary)
	}
	// Long-tail phrases are grounded.
	if len(k.LongTail) == 0 {
		t.Errorf("no long-tail keywords")
	}
}

func TestHashtagsPerPlatformDeduped(t *testing.T) {
	m, _ := newGen().SEO(context.Background(), samplePackage())
	h := m.Hashtags
	if len(h.YouTube) == 0 || len(h.LinkedIn) == 0 || len(h.X) == 0 {
		t.Errorf("platform hashtags missing: %+v", h)
	}
	if len(h.YouTube) > ytHashtagMax || len(h.X) > xHashtagMax {
		t.Errorf("hashtags exceed platform caps: %+v", h)
	}
	if hasDup(h.All) {
		t.Errorf("merged hashtags contain duplicates: %v", h.All)
	}
	for _, tag := range h.All {
		if !strings.HasPrefix(tag, "#") {
			t.Errorf("malformed hashtag %q", tag)
		}
	}
}

func TestExcerptsLength(t *testing.T) {
	ex := newGen().planExcerpts(context.Background(), samplePackage())
	if wordCount(ex.Short) > 56 {
		t.Errorf("short excerpt too long: %d words", wordCount(ex.Short))
	}
	if wordCount(ex.Medium) > 101 || wordCount(ex.Long) > 201 {
		t.Errorf("excerpts exceed caps: med=%d long=%d", wordCount(ex.Medium), wordCount(ex.Long))
	}
	if ex.Short == "" {
		t.Error("empty short excerpt")
	}
}

func TestOpenGraphAndStructuredData(t *testing.T) {
	m, _ := newGen().SEO(context.Background(), samplePackage())
	if m.OpenGraph.Type != "article" || m.OpenGraph.Twitter.Card != "summary_large_image" {
		t.Errorf("open graph = %+v", m.OpenGraph)
	}
	ld := m.StructuredData.JSONLD
	if ld["@type"] != "TechArticle" || ld["headline"] == "" {
		t.Errorf("json-ld = %+v", ld)
	}
	if m.StructuredData.RSS.Title == "" || m.StructuredData.Sitemap.ChangeFreq == "" {
		t.Errorf("rss/sitemap incomplete: %+v", m.StructuredData)
	}
	// Canonical URL uses the repo homepage.
	if !strings.Contains(m.Blog.Canonical.URL, "widget.dev") {
		t.Errorf("canonical not from homepage: %q", m.Blog.Canonical.URL)
	}
}

func TestReadingTime(t *testing.T) {
	if got := readingMinutes(samplePackage()); got < 1 {
		t.Errorf("reading time = %d, want >= 1", got)
	}
}

func TestConfidenceScore(t *testing.T) {
	m, _ := newGen().SEO(context.Background(), samplePackage())
	if m.ContentIntelligence.SEOConfidenceScore < 70 {
		t.Errorf("expected a healthy confidence score for a complete set, got %d", m.ContentIntelligence.SEOConfidenceScore)
	}
}

func TestMarkdownRenders(t *testing.T) {
	m, _ := newGen().SEO(context.Background(), samplePackage())
	md := m.Markdown()
	for _, want := range []string{
		"# SEO Metadata", "## Blog SEO", "## YouTube SEO", "## Short-form SEO",
		"## Social SEO", "## Keywords", "## Hashtags", "## Open Graph", "## Structured Data",
		"```json", "@type",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "A short, punchy, grounded SEO string about the release.", nil
}

func TestModelPolishesTitlesAndDescriptions(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	m, err := g.SEO(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	// Title + meta description + excerpt = at least 3 calls.
	if fm.calls < 3 {
		t.Errorf("model called %d times, want >= 3", fm.calls)
	}
	// Even model output stays within limits.
	if len(m.Blog.MetaDescription) > BlogDescMax {
		t.Errorf("model meta description exceeds limit: %d", len(m.Blog.MetaDescription))
	}
}

func TestModelOverlongOutputIsTruncated(t *testing.T) {
	long := &longModel{}
	g := &Generator{Model: long, Now: newGen().Now}
	m, err := g.SEO(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Blog.MetaDescription) > BlogDescMax {
		t.Errorf("overlong model meta not truncated: %d chars", len(m.Blog.MetaDescription))
	}
	if l := len(m.Blog.Title); l > 60 {
		t.Errorf("overlong model title not truncated: %d chars", l)
	}
}

type longModel struct{}

func (longModel) Generate(_ context.Context, _ string) (string, error) {
	return strings.Repeat("very long seo output ", 40), nil
}

func TestRequiresContext(t *testing.T) {
	if _, err := newGen().SEO(context.Background(), ReleasePackage{}); err == nil {
		t.Error("expected error for nil context")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	pkg := samplePackage()
	bad := SEOMetadata{
		SchemaVersion: "1.0.0",
		Blog:          BlogSEO{Title: "", MetaDescription: strings.Repeat("x", 200), Slug: "Bad Slug"},
		YouTube:       YouTubeSEO{Title: "", Description: ""},
		Keywords:      Keywords{Primary: []string{"dup", "dup"}},
	}
	probs := bad.Validate(pkg)
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"empty title", "exceeds the 160 limit", "invalid slug", "duplicate primary", "youtube: empty", "shorts: no items", "missing JSON-LD"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}
