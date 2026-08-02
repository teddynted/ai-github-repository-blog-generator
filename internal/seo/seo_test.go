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

// topicPackage builds a minimal package for a given engineering topic so keyword
// extraction can be asserted against real search intent.
func topicPackage(feature string, seoKeywords, highlights, awsServices, blogTags []string) ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "platform", FullName: "acme/platform", Language: "Go"},
		Release:       rc.Release{Tag: "v0.6.0"},
		Architecture:  rc.Architecture{Overview: feature, AWSServices: awsServices},
		Changelog:     rc.ChangelogAnalysis{Found: true, Features: []string{feature}},
		Technologies:  []rc.Technology{{Name: "Go"}, {Name: "AWS CloudFormation"}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:             feature,
			SEOKeywords:         seoKeywords,
			TechnicalHighlights: highlights,
		},
	}
	return ReleasePackage{Context: ctx, Blog: releasegen.BlogPost{Title: feature, Tags: blogTags}}
}

func TestPrimaryKeywordsPreferTopicOverServiceInventory_AMI(t *testing.T) {
	// "Replacing EC2 UserData provisioning with custom AMIs" — the primary
	// keywords must be the topic, not the detected service inventory.
	pkg := topicPackage(
		"Pre-baked AMIs cut EC2 startup time for AI agents by replacing UserData provisioning",
		[]string{"AWS Custom AMI", "EC2 startup optimization", "pre-baked AMI"},
		[]string{"immutable machine images"},
		[]string{"Amazon EC2", "AWS IAM", "Amazon CloudWatch", "AWS Lambda", "Amazon EventBridge"},
		[]string{"aws", "ec2"},
	)
	k := planKeywords(pkg)

	primaryText := strings.ToLower(strings.Join(k.Primary, " | "))
	for _, concept := range []string{"ami", "startup"} {
		if !strings.Contains(primaryText, concept) {
			t.Errorf("primary missing core concept %q; got %v", concept, k.Primary)
		}
	}
	for _, reject := range []string{"AWS IAM", "Amazon CloudWatch", "AWS Lambda"} {
		if containsFold(k.Primary, reject) {
			t.Errorf("primary should not contain service inventory %q; got %v", reject, k.Primary)
		}
	}
	if c, n := countAWSServiceNames(k.Primary, lowerSet(awsServices(pkg))), len(k.Primary); c*2 > n {
		t.Errorf("AWS services are %d of %d primary keywords (>50%%): %v", c, n, k.Primary)
	}
	// Services are demoted to Secondary, not dropped.
	if !containsFold(k.Secondary, "AWS Lambda") {
		t.Errorf("expected AWS Lambda in secondary; got %v", k.Secondary)
	}
}

func TestPrimaryKeywordsKeepCentralService_Bedrock(t *testing.T) {
	// "Hybrid Claude + Bedrock inference routing" — Bedrock IS a service but the
	// article is about it, so it stays primary; incidental Lambda/S3 do not.
	pkg := topicPackage(
		"Hybrid Claude and Bedrock inference routing across providers",
		[]string{"AI inference routing", "Amazon Bedrock", "Claude integration"},
		nil,
		[]string{"AWS Lambda", "Amazon S3", "Amazon Bedrock"},
		[]string{"ai", "bedrock"},
	)
	k := planKeywords(pkg)

	for _, want := range []string{"AI inference routing", "Amazon Bedrock", "Claude integration"} {
		if !containsFold(k.Primary, want) {
			t.Errorf("primary missing %q; got %v", want, k.Primary)
		}
	}
	for _, reject := range []string{"AWS Lambda", "Amazon S3"} {
		if containsFold(k.Primary, reject) {
			t.Errorf("primary should not contain incidental service %q; got %v", reject, k.Primary)
		}
	}
	if c, n := countAWSServiceNames(k.Primary, lowerSet(awsServices(pkg))), len(k.Primary); c*2 > n {
		t.Errorf("AWS services are %d of %d primary keywords (>50%%): %v", c, n, k.Primary)
	}
}

func TestPrimaryKeywordsRejectNoiseTokens(t *testing.T) {
	pkg := topicPackage(
		"Custom AMIs speed up EC2 startup",
		[]string{"EC2 startup optimization", "Go 1.22", "AWS SDK for Go", "aws", "platform", "custom AMI"},
		nil,
		[]string{"Amazon EC2"},
		[]string{"go", "aws"},
	)
	k := planKeywords(pkg)
	for _, reject := range []string{"Go 1.22", "AWS SDK for Go", "aws", "platform"} {
		if containsFold(k.Primary, reject) {
			t.Errorf("primary must not contain noise token %q; got %v", reject, k.Primary)
		}
	}
	// The real search-intent terms survive and lead.
	if !containsFold(k.Primary, "EC2 startup optimization") {
		t.Errorf("expected search-intent keyword to survive; got %v", k.Primary)
	}
}

func TestAboutExcludesIncidentalServices(t *testing.T) {
	pkg := topicPackage(
		"Pre-baked AMIs cut EC2 startup time",
		[]string{"AWS Custom AMI", "EC2 startup optimization", "pre-baked AMI"},
		nil,
		[]string{"Amazon EC2", "AWS IAM", "Amazon CloudWatch"},
		nil,
	)
	m, err := newGen().SEO(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	about := jsonldAbout(m.StructuredData.JSONLD)
	joined := strings.ToLower(strings.Join(about, "|"))
	if strings.Contains(joined, "iam") || strings.Contains(joined, "cloudwatch") {
		t.Errorf("about leaked incidental services: %v", about)
	}
	if !strings.Contains(joined, "ami") && !strings.Contains(joined, "startup") {
		t.Errorf("about does not reflect the topic: %v", about)
	}
}

func TestYouTubeSocialInheritsTopic(t *testing.T) {
	pkg := topicPackage(
		"Pre-baked Custom AMIs for faster EC2 startup",
		[]string{"EC2 startup optimization", "Custom AMI", "pre-baked AMI"},
		nil,
		[]string{"Amazon EC2", "AWS IAM"},
		nil,
	)
	m, err := newGen().SEO(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	// Thumbnail text carries the topic.
	if !sharesToken(m.YouTube.ThumbnailText, strings.Join(m.Keywords.Primary, " ")) {
		t.Errorf("thumbnail text does not reflect topic: %v", m.YouTube.ThumbnailText)
	}
	// Description leads with the topic keyword.
	if !descLeadsWithKeyword(m.YouTube.Description, m.Keywords.Primary) {
		t.Errorf("description does not lead with topic: %q", m.YouTube.Description)
	}
	// Hashtags do not surface the incidental IAM service.
	for _, h := range m.YouTube.Hashtags {
		if strings.Contains(strings.ToLower(h), "iam") {
			t.Errorf("hashtags leaked incidental service: %v", m.YouTube.Hashtags)
		}
	}
}

func TestValidationRejectsNoiseAndTopicDrift(t *testing.T) {
	pkg := samplePackage()
	m, _ := newGen().SEO(context.Background(), pkg)
	// Inject a noisy primary keyword and a topic-drifted about.
	m.Keywords.Primary = append([]string{"aws"}, m.Keywords.Primary...)
	m.StructuredData.JSONLD["about"] = []string{"Amazon QuantumLedger", "Unrelated Thing"}
	m.Blog.Title = "Pre-Baked Custom AMIs for Faster EC2 Startup"
	probs := strings.Join(m.Validate(pkg), " | ")
	if !strings.Contains(probs, "generic/repo/version/SDK token") {
		t.Errorf("expected noisy-primary failure; got: %s", probs)
	}
	if !strings.Contains(probs, "about shares no topic term") {
		t.Errorf("expected about/title drift failure; got: %s", probs)
	}
}

// amiPackage reproduces the real contaminated AMI-startup release: a long repo
// name whose fragments leak ("designing", "an"), and junk SEO keywords/tags
// ("changelog", "aws lambda (go runtime)", "aws sdk for go v2", "AWS IAM").
func amiPackage() ReleasePackage {
	junk := []string{
		"AWS Custom AMI", "EC2 startup optimization", "pre-baked AMI",
		"AWS Spot Instance optimization",
		"an", "changelog", "designing", "aws lambda (go runtime)",
		"aws sdk for go v2", "AWS IAM",
	}
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "designing-an-ai-agent-platform-on-aws", FullName: "teddynted/designing-an-ai-agent-platform-on-aws", Language: "Go"},
		Release:       rc.Release{Tag: "v0.6.0"},
		Architecture:  rc.Architecture{Overview: "Pre-baked custom AMIs remove UserData provisioning from EC2 boot to speed up Spot startup.", AWSServices: []string{"Amazon EC2", "AWS IAM", "Amazon CloudWatch", "AWS Lambda", "Amazon EventBridge", "AWS CloudFormation"}},
		Changelog:     rc.ChangelogAnalysis{Found: true, Features: []string{"Pre-baked AMIs speed up AWS Spot startup by baking dependencies into the image"}},
		Technologies:  []rc.Technology{{Name: "Go"}, {Name: "AWS CloudFormation"}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:     "Pre-baked custom AMIs speed up AWS Spot startup.",
			SEOKeywords: junk,
		},
	}
	post := releasegen.BlogPost{
		Title:           "Pre-Baked AMIs to Speed Up AWS Spot Startup",
		MetaDescription: "Versioned pre-baked custom AMIs move EC2 provisioning into image creation for faster, deterministic Spot startup.",
		Tags:            []string{"aws", "ec2", "an", "changelog"},
	}
	yt := youtube.YouTubeScript{
		SchemaVersion:       youtube.SchemaVersion,
		Metadata:            youtube.Metadata{Repository: "teddynted/designing-an-ai-agent-platform-on-aws", Release: "v0.6.0"},
		ContentIntelligence: youtube.Intelligence{SuggestedTags: junk, SuggestedThumbnail: "AWS IAM / AN / V0.6.0"},
	}
	return ReleasePackage{Context: ctx, Blog: post, YouTube: yt}
}

func TestAMIScenarioNoContamination(t *testing.T) {
	pkg := amiPackage()
	m, err := newGen().SEO(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"an", "changelog", "designing", "aws lambda (go runtime)", "aws sdk for go v2", "aws iam"}

	contains := func(list []string, term string) bool { return containsFold(list, term) }

	// Primary keywords: topic only, no contamination.
	for _, b := range banned {
		if contains(m.Keywords.Primary, b) {
			t.Errorf("primary contains banned token %q: %v", b, m.Keywords.Primary)
		}
	}
	if !strings.Contains(strings.ToLower(strings.Join(m.Keywords.Primary, " | ")), "ami") {
		t.Errorf("primary missing AMI topic concept: %v", m.Keywords.Primary)
	}

	// YouTube tags / JSON-LD keywords / blog keywords: no junk substrings.
	fields := map[string][]string{
		"youtube.tags":  m.YouTube.Tags,
		"blog.keywords": m.Blog.Keywords,
		"youtube.kw":    m.YouTube.Keywords,
	}
	for name, list := range fields {
		joined := strings.ToLower(strings.Join(list, " | "))
		for _, bad := range []string{"changelog", "designing", "runtime", "(go runtime)", "sdk", " an "} {
			if strings.Contains(" "+joined+" ", bad) {
				t.Errorf("%s leaked %q: %v", name, bad, list)
			}
		}
	}

	// JSON-LD about: concept-level, no incidental services.
	about := strings.ToLower(strings.Join(jsonldAbout(m.StructuredData.JSONLD), " | "))
	if strings.Contains(about, "iam") || strings.Contains(about, "cloudwatch") {
		t.Errorf("about leaked incidental services: %s", about)
	}

	// Thumbnail: no IAM / AN / repo fragments.
	if bad := offTopicThumbnailToken(m.YouTube.ThumbnailText, pkg, m.Blog.Title); bad != "" {
		t.Errorf("thumbnail off-topic token %q: %v", bad, m.YouTube.ThumbnailText)
	}
	if strings.Contains(strings.ToLower(strings.Join(m.YouTube.ThumbnailText, " ")), "iam") {
		t.Errorf("thumbnail contains IAM: %v", m.YouTube.ThumbnailText)
	}

	// Alt slugs: topic-aligned, no aws-iam.
	for _, s := range m.Blog.AlternativeSlugs {
		if strings.Contains(s, "iam") {
			t.Errorf("alt slug off-topic: %q", s)
		}
	}

	// Canonical: template placeholder when no homepage.
	if !strings.HasPrefix(m.Blog.Canonical.URL, "{{site_url}}/") {
		t.Errorf("canonical placeholder missing: %q", m.Blog.Canonical.URL)
	}

	// The clean artifact must validate.
	if probs := m.Validate(pkg); len(probs) != 0 {
		t.Errorf("clean AMI artifact failed validation: %v", probs)
	}
}

func TestValidationFailsOnReintroducedJunk(t *testing.T) {
	pkg := amiPackage()
	m, _ := newGen().SEO(context.Background(), pkg)
	// Re-introduce the exact junk the pipeline must never emit.
	m.Keywords.Primary = append([]string{"an"}, m.Keywords.Primary...)
	if probs := m.Validate(pkg); len(probs) == 0 {
		t.Error("validation should fail when 'an' is a primary keyword")
	}
	m2, _ := newGen().SEO(context.Background(), pkg)
	m2.Keywords.Primary = []string{"changelog"}
	if probs := m2.Validate(pkg); len(probs) == 0 {
		t.Error("validation should fail when 'changelog' is a primary keyword")
	}
}

// spotPackage reproduces the "Optimizing Spot Startup on AWS: Pre-Baked Custom
// AMIs …" article, whose SEO keywords are polluted with platform concepts
// (GitHub Actions, Go Modules, Infrastructure as Code, Event-Driven Architecture)
// and a non-central service (AWS IAM).
func spotPackage() ReleasePackage {
	seokw := []string{
		"AWS Custom AMI", "EC2 startup optimization", "pre-baked AMI", "EC2 UserData optimization",
		"GitHub Actions", "Go Modules", "Infrastructure as Code", "Event-Driven Architecture", "AWS IAM",
	}
	ctx := &rc.ReleaseContext{
		SchemaVersion:       "1.0.0",
		Repository:          rc.Repository{Name: "designing-an-ai-agent-platform-on-aws", FullName: "teddynted/designing-an-ai-agent-platform-on-aws", Language: "Go"},
		Release:             rc.Release{Tag: "v0.6.0"},
		Architecture:        rc.Architecture{Overview: "Pre-baked custom AMIs move EC2 UserData provisioning into image creation to cut Spot startup latency.", AWSServices: []string{"Amazon EC2", "AWS IAM", "AWS Lambda", "Amazon CloudWatch", "Amazon EventBridge"}},
		Changelog:           rc.ChangelogAnalysis{Found: true, Features: []string{"Pre-baked custom AMIs cut EC2 Spot startup latency"}},
		Technologies:        []rc.Technology{{Name: "Go"}, {Name: "AWS CloudFormation"}},
		ContentIntelligence: rc.ContentIntelligence{Summary: "Pre-baked custom AMIs cut EC2 Spot startup latency.", SEOKeywords: seokw},
	}
	post := releasegen.BlogPost{
		Title:           "Optimizing Spot Startup on AWS: Pre-Baked Custom AMIs for an Event-Driven AI Agent Platform",
		MetaDescription: "Replace boot-time EC2 UserData provisioning with versioned pre-baked custom AMIs to cut Spot startup latency.",
		Tags:            []string{"aws", "ec2", "github-actions", "go-modules"},
	}
	yt := youtube.YouTubeScript{SchemaVersion: youtube.SchemaVersion, ContentIntelligence: youtube.Intelligence{SuggestedTags: seokw}}
	return ReleasePackage{Context: ctx, Blog: post, YouTube: yt}
}

func TestPlatformConceptsRejectedFromPrimaryAndAbout(t *testing.T) {
	pkg := spotPackage()
	m, err := newGen().SEO(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"GitHub Actions", "github-actions", "Go Modules", "Infrastructure as Code", "Event-Driven Architecture", "AWS IAM"}
	for _, b := range banned {
		if containsFold(m.Keywords.Primary, b) {
			t.Errorf("primary must not contain platform concept %q: %v", b, m.Keywords.Primary)
		}
	}
	// Primary keywords represent the article's optimization topic (mined from the
	// title/meta): they carry the core AMI/startup/UserData concepts.
	primaryText := strings.ToLower(strings.Join(m.Keywords.Primary, " | "))
	for _, concept := range []string{"ami", "startup", "userdata"} {
		if !strings.Contains(primaryText, concept) {
			t.Errorf("primary missing core concept %q: %v", concept, m.Keywords.Primary)
		}
	}
	if len(m.Keywords.Primary) > 5 {
		t.Errorf("primary exceeds 5: %v", m.Keywords.Primary)
	}
	// At least two primary keywords overlap the article title (topic alignment).
	if !sharesToken(m.Keywords.Primary, m.Blog.Title) {
		t.Errorf("primary keywords do not overlap the title: %v", m.Keywords.Primary)
	}
	// about is topic-aligned: no platform concepts, no incidental IAM.
	about := strings.ToLower(strings.Join(jsonldAbout(m.StructuredData.JSONLD), " | "))
	for _, bad := range []string{"github actions", "go modules", "infrastructure as code", "event-driven architecture", "iam"} {
		if strings.Contains(about, bad) {
			t.Errorf("about leaked %q: %s", bad, about)
		}
	}
	if !strings.Contains(about, "ami") || !strings.Contains(about, "ec2") {
		t.Errorf("about not topic-aligned: %s", about)
	}
	// Thumbnail reflects the primary topic.
	if !sharesToken(m.YouTube.ThumbnailText, strings.Join(m.Keywords.Primary, " ")) {
		t.Errorf("thumbnail not topic-aligned: %v", m.YouTube.ThumbnailText)
	}
	// Tags are topic-led (≤50% AWS services) on both channels.
	for name, tags := range map[string][]string{"blog": m.Blog.Tags, "youtube": m.YouTube.Tags} {
		if n := len(tags); n > 0 && countAWSServiceNames(tags, lowerSet(awsServices(pkg)))*2 > n {
			t.Errorf("%s tags are majority AWS services: %v", name, tags)
		}
	}
	if probs := m.Validate(pkg); len(probs) != 0 {
		t.Errorf("clean spot-startup artifact failed validation: %v", probs)
	}
}

// serviceOnlySEOPackage mirrors the real designing-v0.6.0 fixture: the upstream
// SEO keyword list is ENTIRELY service inventory + junk, so primary keywords must
// be mined from the article title/meta, not fall back to the service list.
func serviceOnlySEOPackage() ReleasePackage {
	junkSEO := []string{"agent", "ai", "amazon cloudwatch", "amazon ec2", "amazon eventbridge", "amazon s3", "an", "aws", "aws iam", "aws lambda", "aws lambda (go runtime)", "aws sdk for go v2", "changelog", "designing", "event-driven architecture", "github actions", "go", "go modules", "infrastructure as code"}
	ctx := &rc.ReleaseContext{
		SchemaVersion:       "1.0.0",
		Repository:          rc.Repository{Name: "designing-an-ai-agent-platform-on-aws", FullName: "teddynted/designing-an-ai-agent-platform-on-aws", Language: "Go"},
		Release:             rc.Release{Tag: "v0.6.0"},
		Architecture:        rc.Architecture{Overview: "An event-driven, AWS-native system on AWS IAM, AWS Lambda, Amazon CloudWatch, Amazon EC2, Amazon EventBridge.", AWSServices: []string{"AWS IAM", "AWS Lambda", "Amazon CloudWatch", "Amazon EC2", "Amazon EventBridge", "Amazon S3"}},
		ContentIntelligence: rc.ContentIntelligence{Summary: "release v0.6.0 delivers 7 analyzed changes across 24 files.", SEOKeywords: junkSEO},
	}
	post := releasegen.BlogPost{
		Title:           "Optimizing Spot Startup on AWS: Pre-Baked Custom AMIs for an Event-Driven AI Agent Platform",
		MetaDescription: "How an event-driven AI agent platform on AWS trades boot-time UserData provisioning for versioned, pre-baked custom AMIs to make on-demand EC2 startup fast.",
		Tags:            []string{"aws-iam", "aws-lambda", "amazon-cloudwatch", "amazon-ec2", "amazon-eventbridge", "amazon-s3", "github-actions"},
	}
	return ReleasePackage{Context: ctx, Blog: post}
}

func TestPrimaryMinedFromTitleWhenSEOKeywordsAreServiceOnly(t *testing.T) {
	pkg := serviceOnlySEOPackage()
	m, err := newGen().SEO(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	// Primary must NOT be the service inventory (the pre-fix failure mode).
	for _, svc := range []string{"aws-iam", "aws iam", "aws-lambda", "aws lambda", "amazon-cloudwatch", "amazon cloudwatch"} {
		if containsFold(m.Keywords.Primary, svc) {
			t.Errorf("primary fell back to service inventory %q: %v", svc, m.Keywords.Primary)
		}
	}
	// It must carry the article's optimization concepts, mined from the title/meta.
	primaryText := strings.ToLower(strings.Join(m.Keywords.Primary, " | "))
	for _, concept := range []string{"ami", "startup"} {
		if !strings.Contains(primaryText, concept) {
			t.Errorf("primary missing core concept %q: %v", concept, m.Keywords.Primary)
		}
	}
	// At most 50% of primary may be AWS service names.
	if c, n := countAWSServiceNames(m.Keywords.Primary, lowerSet(awsServices(pkg))), len(m.Keywords.Primary); c*2 > n {
		t.Errorf("primary is majority services (%d/%d): %v", c, n, m.Keywords.Primary)
	}
	// about + thumbnail carry the topic, not the service inventory.
	about := strings.ToLower(strings.Join(jsonldAbout(m.StructuredData.JSONLD), " | "))
	if strings.Contains(about, "iam") || strings.Contains(about, "cloudwatch") {
		t.Errorf("about leaked service inventory: %s", about)
	}
	if !sharesToken(m.YouTube.ThumbnailText, m.Blog.Title) {
		t.Errorf("thumbnail not topic-aligned: %v", m.YouTube.ThumbnailText)
	}
	if probs := m.Validate(pkg); len(probs) != 0 {
		t.Errorf("clean artifact failed validation: %v", probs)
	}
}

func TestKeyphrasesFromText(t *testing.T) {
	pkg := serviceOnlySEOPackage()
	got := keyphrasesFromText(pkg.Blog.Title+". "+pkg.Blog.MetaDescription, pkg)
	joined := strings.ToLower(strings.Join(got, " | "))
	// Extracted phrases must be clean (no interior connectors, no services).
	for _, bad := range []string{" on aws", "aws iam", "aws lambda", " for an", " to make"} {
		if strings.Contains(" "+joined+" ", bad) {
			t.Errorf("keyphrase not clean, leaked %q: %v", bad, got)
		}
	}
	if !strings.Contains(joined, "ami") || !strings.Contains(joined, "startup") {
		t.Errorf("keyphrases missing topic anchors: %v", got)
	}
}

func TestTagsAreLowercaseHyphenSlugs(t *testing.T) {
	m, err := newGen().SEO(context.Background(), serviceOnlySEOPackage())
	if err != nil {
		t.Fatal(err)
	}
	for name, tags := range map[string][]string{"blog": m.Blog.Tags, "youtube": m.YouTube.Tags} {
		for _, tag := range tags {
			if tag != strings.ToLower(tag) {
				t.Errorf("%s tag not lowercase: %q", name, tag)
			}
			if strings.ContainsAny(tag, " …_") {
				t.Errorf("%s tag has space/ellipsis/underscore: %q", name, tag)
			}
			if !validSlug(tag) {
				t.Errorf("%s tag is not a valid slug: %q", name, tag)
			}
		}
	}
}

func TestChaptersDedupedByTimestamp(t *testing.T) {
	pkg := samplePackage()
	pkg.YouTube.ContentIntelligence.Chapters = []youtube.ChapterMarker{
		{Timestamp: "00:00", Title: "Intro"},
		{Timestamp: "04:29", Title: "Conclusion"},
		{Timestamp: "04:29", Title: "Conclusion"},
	}
	m, err := newGen().SEO(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range m.YouTube.ChapterTitles {
		if c.Timestamp == "04:29" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("expected exactly one 04:29 chapter, got %d: %v", n, m.YouTube.ChapterTitles)
	}
	if hasDuplicateChapters(m.YouTube.ChapterTitles) {
		t.Error("generated chapters still contain a duplicate timestamp")
	}
}

func TestDuplicateChapterTimestampsFailValidation(t *testing.T) {
	pkg := spotPackage()
	m, _ := newGen().SEO(context.Background(), pkg)
	m.YouTube.ChapterTitles = []ChapterTitle{{Timestamp: "02:40", Title: "A"}, {Timestamp: "02:40", Title: "B"}}
	if probs := strings.Join(m.Validate(pkg), " | "); !strings.Contains(probs, "duplicate chapter timestamps") {
		t.Errorf("duplicate chapter timestamps must fail validation; got: %s", probs)
	}
}

func TestPlatformConceptAllowedWhenInHeadline(t *testing.T) {
	// If the article headline IS about the concept, it may be a primary keyword.
	pkg := spotPackage()
	pkg.Blog.Title = "Event-Driven Architecture on AWS: A Practical Guide"
	if isPlatformConcept("Event-Driven Architecture", pkg) {
		t.Error("a concept named in the headline should be allowed as primary")
	}
	if !isPlatformConcept("GitHub Actions", pkg) {
		t.Error("a concept absent from the headline should be rejected")
	}
}

func TestIsAWSServiceNameWholeStringOnly(t *testing.T) {
	det := lowerSet([]string{"Amazon EC2", "AWS Lambda"})
	if !isAWSServiceName("Amazon EC2", det) || !isAWSServiceName("aws iam", nil) {
		t.Error("expected service names to be recognized")
	}
	// Topic phrases that merely contain a service token are NOT services.
	for _, topic := range []string{"EC2 startup optimization", "AWS Custom AMI", "pre-baked AMI"} {
		if isAWSServiceName(topic, det) {
			t.Errorf("%q wrongly classified as an AWS service", topic)
		}
	}
}

func TestTopicClustersAreDomainDerived(t *testing.T) {
	pkg := topicPackage(
		"Pre-baked AMIs cut EC2 startup time via an event-driven pipeline",
		[]string{"EC2 startup optimization"}, nil,
		[]string{"Amazon EC2", "Amazon EventBridge"}, nil)
	got := topicClusters(pkg)
	joined := strings.ToLower(strings.Join(got, "|"))
	// Must reflect the actual domain, not this project's hardcoded topics.
	if strings.Contains(joined, "content generation") || strings.Contains(joined, "github release automation") {
		t.Errorf("clusters leaked hardcoded project topics: %v", got)
	}
	if !strings.Contains(joined, "startup optimization") && !strings.Contains(joined, "event-driven") {
		t.Errorf("clusters do not reflect the release domain: %v", got)
	}
}

func TestSanitizeModelTextDropsCommentary(t *testing.T) {
	if got := sanitizeModelText("The draft is weak because there are no facts here.", "Fallback Title"); got != "Fallback Title" {
		t.Errorf("commentary leaked: %q", got)
	}
	if got := sanitizeModelText("  Faster EC2 Startup with Custom AMIs  ", "fb"); got != "Faster EC2 Startup with Custom AMIs" {
		t.Errorf("clean output altered: %q", got)
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
