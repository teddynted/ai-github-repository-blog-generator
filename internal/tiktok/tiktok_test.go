package tiktok

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/shorts"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// samplePackage builds a representative ReleasePackage whose YouTube Shorts span
// several angles (so TikTok discovery adapts them 1:1).
func samplePackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "widget", FullName: "acme/widget", URL: "https://github.com/acme/widget", Language: "Go"},
		Release:       rc.Release{Tag: "v0.2.0"},
		Architecture:  rc.Architecture{AWSServices: []string{"AWS Lambda", "Amazon SQS"}},
		CommitStats:   rc.CommitStats{Analyzed: 9, Total: 9},
		FileStats:     rc.FileStats{Total: 12},
		Mermaid:       []rc.MermaidDiagram{{Source: "docs/architecture.md", Type: "flowchart"}},
		Technologies:  []rc.Technology{{Name: "Go"}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:                  "widget v0.2.0 delivers a release context builder.",
			ImplementationComplexity: "medium",
			TargetAudience:           "Cloud engineers",
			SEOKeywords:              []string{"go", "aws", "serverless"},
		},
	}
	post := releasegen.BlogPost{Title: "Inside widget v0.2.0", Tags: []string{"go", "aws"}}

	mkShort := func(id int, angle, seed string) shorts.Short {
		return shorts.Short{
			ID: id, Angle: angle, Title: angle + " short", CoreExplanation: seed,
			Source:  shorts.Source{Seed: seed, StoryboardScene: id},
			Visuals: []shorts.Visual{{Kind: "Architecture Diagram", Description: "The diagram", Reference: "docs/architecture.md"}},
		}
	}
	sh := shorts.ShortsCollection{
		SchemaVersion: shorts.SchemaVersion,
		Metadata:      shorts.Metadata{Repository: "acme/widget", Release: "v0.2.0"},
		Shorts: []shorts.Short{
			mkShort(1, "Architecture Reveal", "Each component has a single responsibility."),
			mkShort(2, "Common Mistake", "Don't leak infrastructure types into the domain."),
			mkShort(3, "Developer Tip", "Keep logic behind a small interface."),
			mkShort(4, "Optimization", "The win is in reproducibility."),
		},
	}

	return ReleasePackage{Context: ctx, Blog: post, Shorts: sh}
}

func newGen() *Generator {
	return &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
}

func TestStripReasoning(t *testing.T) {
	// The exact leak seen in Video 3's hook: chain-of-thought embedded mid-text.
	in := `I stopped renaming files by hand — here's why. Wait, that invents a detail. The draft doesn't say what "this" is. I stopped doing this by hand — here's why.`
	got := stripReasoning(in)
	for _, banned := range []string{"Wait, that invents", "the draft doesn't say", "invents a detail"} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(banned)) {
			t.Errorf("reasoning leaked through: %q", got)
		}
	}
	if !strings.Contains(got, "I stopped renaming files by hand") {
		t.Errorf("spoken content was lost: %q", got)
	}
	// Clean narration is returned unchanged.
	clean := "EventBridge Scheduler fires Lambda. Lambda starts and stops EC2."
	if stripReasoning(clean) != clean {
		t.Errorf("clean narration altered: %q", stripReasoning(clean))
	}
	// All-reasoning input never blanks the field.
	if stripReasoning("I should rewrite this.") == "" {
		t.Errorf("must not return empty")
	}
}

func TestTikTokEndToEnd(t *testing.T) {
	pkg := samplePackage()
	col, err := newGen().TikTok(context.Background(), pkg)
	if err != nil {
		t.Fatalf("TikTok: %v", err)
	}

	if col.SchemaVersion != SchemaVersion || col.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", col.Metadata)
	}
	if len(col.Videos) < 2 {
		t.Fatalf("expected multiple videos, got %d", len(col.Videos))
	}
	if col.Metadata.VideoCount != len(col.Videos) {
		t.Errorf("videoCount %d != %d", col.Metadata.VideoCount, len(col.Videos))
	}

	for _, v := range col.Videos {
		if v.Hook == "" || v.Script == "" || v.CTA == "" || v.EngagementPrompt == "" {
			t.Errorf("video %d under-populated", v.ID)
		}
		if len(v.Scenes) == 0 || len(v.Captions) == 0 || len(v.Visuals) == 0 {
			t.Errorf("video %d missing scenes/captions/visuals", v.ID)
		}
		if v.DurationSec < tiktokMinSec || v.DurationSec > tiktokMaxSec {
			t.Errorf("video %d duration %d outside window", v.ID, v.DurationSec)
		}
		if v.RetentionScore <= 0 || v.RetentionScore > 100 {
			t.Errorf("video %d retention %d out of range", v.ID, v.RetentionScore)
		}
	}

	if probs := col.Validate(pkg); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
	blob, err := json.Marshal(col)
	if err != nil || !strings.Contains(string(blob), "\"schemaVersion\":\"1.0.0\"") {
		t.Errorf("marshal: %v", err)
	}
	if col.Metadata.SourceSchemas["shorts"] == "" {
		t.Errorf("source schemas = %+v", col.Metadata.SourceSchemas)
	}
}

func TestDiscoveryAdaptsShorts(t *testing.T) {
	cands := discover(samplePackage(), 6)
	if len(cands) < 4 {
		t.Fatalf("expected the four shorts adapted (+stat), got %d", len(cands))
	}
	// Short angles are mapped to TikTok topics and carry the Short provenance.
	var sawArch, sawShortRef bool
	for _, c := range cands {
		if c.Topic == "Architecture Insight" {
			sawArch = true
		}
		if c.Short > 0 {
			sawShortRef = true
		}
		if c.Short > 0 && collapse(c.Seed) == "" {
			t.Errorf("adapted short %d has no seed", c.Short)
		}
	}
	if !sawArch || !sawShortRef {
		t.Errorf("expected adapted Architecture Insight with Short provenance: arch=%v ref=%v", sawArch, sawShortRef)
	}
}

func TestDiscoveryFallbackWithoutShorts(t *testing.T) {
	pkg := samplePackage()
	pkg.Shorts = shorts.ShortsCollection{} // no shorts → must mine
	// Give it a YouTube chapter to mine.
	cands := discover(pkg, 6)
	// At minimum the statistic candidate is grounded from commit/file stats.
	var sawStat bool
	for _, c := range cands {
		if c.Topic == "Interesting Statistic" && strings.Contains(c.Seed, "9") {
			sawStat = true
		}
	}
	if !sawStat {
		t.Errorf("fallback should still yield a grounded statistic: %+v", cands)
	}
}

func TestFallbackMinesChaptersAndCallouts(t *testing.T) {
	pkg := samplePackage()
	pkg.Shorts = shorts.ShortsCollection{} // force the mining fallback
	pkg.YouTube = youtube.YouTubeScript{
		SchemaVersion: youtube.SchemaVersion,
		Chapters: []youtube.Chapter{
			{
				Number: 2, Title: "Architecture", Type: "architecture",
				Script:           "Event-driven on AWS Lambda and Amazon SQS.",
				StoryboardScenes: []int{2},
				VisualReferences: []string{"Diagram: docs/architecture.md"},
				Callouts: []youtube.Callout{
					{Kind: "Best Practice", Text: "Keep the diagram the single source of truth."},
					{Kind: "Common Mistake", Text: "Don't leak infra types into the domain."},
				},
			},
			{
				Number: 3, Title: "CloudFormation", Type: "cloudformation",
				Script:   "Everything is provisioned as code.",
				Callouts: []youtube.Callout{{Kind: "Warning", Text: "Scope IAM tightly."}},
			},
		},
	}
	col, err := newGen().TikTok(context.Background(), pkg)
	if err != nil {
		t.Fatalf("TikTok: %v", err)
	}
	if len(col.Videos) < 3 {
		t.Fatalf("expected mined videos across topics, got %d", len(col.Videos))
	}
	topics := map[string]bool{}
	for _, v := range col.Videos {
		topics[v.Topic] = true
	}
	// Warning on a CloudFormation chapter maps to "CloudFormation Trick".
	if !topics["CloudFormation Trick"] {
		t.Errorf("expected a CloudFormation Trick from the Warning callout: %v", topics)
	}
	if probs := col.Validate(pkg); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
}

func TestMaxVideosCap(t *testing.T) {
	g := &Generator{MaxVideos: 2, Now: newGen().Now}
	col, _ := g.TikTok(context.Background(), samplePackage())
	if len(col.Videos) != 2 {
		t.Errorf("expected 2 videos (capped), got %d", len(col.Videos))
	}
}

func TestHookByTopic(t *testing.T) {
	if h := hookDraft(topic{Topic: "AWS Tip"}); !strings.Contains(strings.ToLower(h), "aws") {
		t.Errorf("aws-tip hook = %q", h)
	}
	if h := hookDraft(topic{Topic: "Interesting Statistic", Seed: "This release landed 9 commits."}); !strings.Contains(h, "9") {
		t.Errorf("statistic hook should use the seed: %q", h)
	}
}

func TestEngagementByTopic(t *testing.T) {
	if p := engagementPrompt(topic{Topic: "GitHub Automation"}); !strings.Contains(strings.ToLower(p), "automate") {
		t.Errorf("automation prompt = %q", p)
	}
	if p := engagementPrompt(topic{Topic: "Best Practice"}); !strings.Contains(strings.ToLower(p), "production") {
		t.Errorf("best-practice prompt = %q", p)
	}
	// Every topic yields a non-empty prompt.
	if engagementPrompt(topic{Topic: "Whatever"}) == "" {
		t.Error("default engagement prompt must not be empty")
	}
}

func TestDurationAndCaptionTiming(t *testing.T) {
	if d := durationFor(strings.Repeat("word ", 300)); d != tiktokMaxSec {
		t.Errorf("long script should clamp to %d, got %d", tiktokMaxSec, d)
	}
	if d := durationFor("only a few words here"); d != tiktokMinSec {
		t.Errorf("short script should floor at %d, got %d", tiktokMinSec, d)
	}
	caps := planCaptions("we use aws lambda to run go code fast and clean every time", 40)
	if len(caps) == 0 {
		t.Fatal("no captions")
	}
	if caps[0].StartSec != 0 || caps[len(caps)-1].EndSec != 40 {
		t.Errorf("captions don't span the duration: %+v", caps)
	}
	prev := 0
	for _, c := range caps {
		if c.StartSec != prev {
			t.Errorf("caption gap: start %d != prev end %d", c.StartSec, prev)
		}
		prev = c.EndSec
	}
	// A technical term triggers a highlight style.
	var highlighted bool
	for _, c := range caps {
		if c.Style == "highlight" {
			highlighted = true
		}
	}
	if !highlighted {
		t.Error("expected a technical caption to be highlighted (aws/lambda/go)")
	}
}

func TestScenesCoverProblemSolution(t *testing.T) {
	scenes := planScenes(topic{Topic: "Architecture Insight"},
		"Hook.", "Problem here.", "Solution one. Solution two.", "Takeaway.", "Engage?", "CTA.", 45,
		[]Visual{{Kind: "Mermaid Diagram", Description: "The diagram", Reference: "docs/architecture.md"}})
	total := 0
	for _, sc := range scenes {
		total += sc.DurationSec
		if collapse(sc.Narration) == "" || sc.Camera == "" {
			t.Errorf("scene %d under-populated: %+v", sc.Number, sc)
		}
	}
	if total != 45 {
		t.Errorf("scene durations sum to %d, want 45", total)
	}
	if len(scenes) < 4 {
		t.Errorf("expected hook+problem+solution+outro, got %d scenes", len(scenes))
	}
}

func TestHashtagsFocusedAndGrounded(t *testing.T) {
	tags := planHashtags(topic{Topic: "AWS Tip"}, samplePackage())
	if len(tags) == 0 || len(tags) > maxHashtags {
		t.Fatalf("hashtag count %d out of range", len(tags))
	}
	joined := strings.Join(tags, " ")
	if !strings.Contains(joined, "#AWS") || !strings.Contains(joined, "#TechTok") {
		t.Errorf("expected #AWS and #TechTok in %v", tags)
	}
}

func TestCTABrief(t *testing.T) {
	for i, tk := range []string{"Architecture Insight", "Developer Productivity", "AWS Tip"} {
		cta := planCTA(topic{Topic: tk}, i)
		if cta == "" || wordCount(cta) > 14 {
			t.Errorf("CTA for %q not brief: %q", tk, cta)
		}
	}
}

func TestRetentionScore(t *testing.T) {
	// Sweet-spot duration + strong topic + dense captions scores higher than a
	// weak, long one.
	strong := retentionScore(Video{Topic: "Common Mistake", DurationSec: 32,
		Captions: make([]Caption, 8), Visuals: make([]Visual, 4), EngagementPrompt: "x"})
	weak := retentionScore(Video{Topic: "Best Practice", DurationSec: 60, Captions: make([]Caption, 2)})
	if strong <= weak {
		t.Errorf("strong retention %d should exceed weak %d", strong, weak)
	}
	if strong > 100 || weak < 0 {
		t.Errorf("retention out of range: strong=%d weak=%d", strong, weak)
	}
}

func TestMetadata(t *testing.T) {
	col, _ := newGen().TikTok(context.Background(), samplePackage())
	ci := col.ContentIntelligence
	if ci.VideoCount != len(col.Videos) || ci.TotalDurationSec == 0 || ci.AverageRetention == 0 {
		t.Errorf("intelligence = %+v", ci)
	}
	if ci.Audience == "" || len(ci.SEOKeywords) == 0 || len(ci.ProductionNotes) == 0 {
		t.Errorf("collection metadata incomplete: %+v", ci)
	}
	for _, v := range col.Videos {
		if v.SEO.Caption == "" || v.SEO.PostingTime == "" {
			t.Errorf("video %d SEO incomplete: %+v", v.ID, v.SEO)
		}
	}
}

func TestMarkdownRenders(t *testing.T) {
	col, _ := newGen().TikTok(context.Background(), samplePackage())
	md := col.Markdown()
	for _, want := range []string{
		"# TikTok Videos", "## Video 1", "### Script", "### Scene Breakdown",
		"### Captions", "### Visual Suggestions", "**Engagement:**", "**CTA:**",
		"**Hashtags:**", "## Collection Intelligence",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "Sharpened native TikTok line.", nil
}

func TestModelSharpensHookAndScript(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	col, err := g.TikTok(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	if fm.calls < len(col.Videos)*2 {
		t.Errorf("model called %d times, want >= %d", fm.calls, len(col.Videos)*2)
	}
	if col.Videos[0].Hook != "Sharpened native TikTok line." {
		t.Errorf("hook not from model: %q", col.Videos[0].Hook)
	}
}

func TestAdaptedVisualsReuseShortGrounding(t *testing.T) {
	// A TikTok adapted from a Short reuses that Short's grounded visual refs,
	// mapped to TikTok visual kinds.
	pkg := samplePackage()
	vis := planVisuals(topic{Topic: "Architecture Insight", Short: 1}, pkg)
	var sawRef bool
	for _, v := range vis {
		if v.Reference == "docs/architecture.md" {
			sawRef = true
		}
	}
	if !sawRef {
		t.Errorf("adapted visuals should reuse the Short's grounded ref: %+v", vis)
	}
	if mapVisualKind("Mermaid Animation") != "Mermaid Diagram" {
		t.Errorf("visual kind not mapped to TikTok")
	}
}

func TestVisualsNonShortPathGrounded(t *testing.T) {
	// When not adapting a Short, visuals are still grounded (diagram source, repo).
	pkg := samplePackage()
	vis := planVisuals(topic{Topic: "Architecture Insight"}, pkg) // Short == 0
	grounded := groundedRefs(pkg)
	var any bool
	for _, v := range vis {
		if v.Reference != "" && grounded[v.Reference] {
			any = true
		}
	}
	if !any {
		t.Errorf("non-Short visuals should be grounded: %+v", vis)
	}
}

func TestWarningsOnOutOfWindowDuration(t *testing.T) {
	w := collectWarnings([]Video{{ID: 1, Title: "x", DurationSec: 90}})
	if len(w) == 0 || !strings.Contains(w[0], "outside") {
		t.Errorf("expected out-of-window warning, got %v", w)
	}
}

func TestRequiresContext(t *testing.T) {
	if _, err := newGen().TikTok(context.Background(), ReleasePackage{}); err == nil {
		t.Error("expected error for nil context")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	pkg := samplePackage()
	bad := TikTokCollection{
		SchemaVersion: "1.0.0",
		Metadata:      Metadata{VideoCount: 1},
		Videos: []Video{{
			ID: 1, Title: "dup", Hook: "", EngagementPrompt: "", CTA: "", DurationSec: 90,
			Scenes:   []Scene{{Number: 1, Narration: ""}},
			Visuals:  []Visual{{Kind: "X", Reference: "not-grounded"}},
			Captions: nil,
		}},
	}
	probs := bad.Validate(pkg)
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"missing hook", "missing engagement prompt", "missing CTA", "no captions", "outside", "empty narration", "is grounded in the release artifacts"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}

func TestValidateRejectsDuplicates(t *testing.T) {
	pkg := samplePackage()
	col, _ := newGen().TikTok(context.Background(), pkg)
	col.Videos = append(col.Videos, col.Videos[0])
	col.Metadata.VideoCount = len(col.Videos)
	if !strings.Contains(strings.Join(col.Validate(pkg), "\n"), "duplicate") {
		t.Error("expected duplicate TikTok to be rejected")
	}
}
