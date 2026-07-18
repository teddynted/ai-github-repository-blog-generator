package shorts

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// samplePackage builds a representative ReleasePackage with a grounded YouTube
// script whose chapters carry callouts across several angles.
func samplePackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "widget", FullName: "acme/widget", URL: "https://github.com/acme/widget", Language: "Go"},
		Release:       rc.Release{Tag: "v0.2.0"},
		Architecture:  rc.Architecture{AWSServices: []string{"AWS Lambda", "Amazon SQS"}},
		CommitStats:   rc.CommitStats{Analyzed: 9, Total: 9},
		FileStats:     rc.FileStats{Total: 12},
		Changelog:     rc.ChangelogAnalysis{Found: true, Features: []string{"release context builder", "storyboard generator"}},
		Mermaid:       []rc.MermaidDiagram{{Source: "docs/architecture.md", Type: "flowchart", Nodes: []string{"A", "B"}}},
		Technologies:  []rc.Technology{{Name: "Go"}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:                  "widget v0.2.0 delivers a release context builder.",
			ImplementationComplexity: "medium",
			TargetAudience:           "Cloud engineers",
			SEOKeywords:              []string{"go", "aws", "serverless"},
		},
	}
	post := releasegen.BlogPost{Title: "Inside widget v0.2.0", Tags: []string{"go", "aws"}}

	sb := storyboard.Storyboard{
		SchemaVersion: storyboard.SchemaVersion,
		Metadata:      storyboard.Metadata{Repository: "acme/widget", Release: "v0.2.0"},
		Scenes: []storyboard.Scene{
			{SceneNumber: 2, Type: "architecture", Diagrams: []storyboard.DiagramRef{{Source: "docs/architecture.md"}}},
		},
	}
	vo := voiceover.VoiceOverScript{SchemaVersion: voiceover.SchemaVersion}

	yt := youtube.YouTubeScript{
		SchemaVersion: youtube.SchemaVersion,
		Metadata:      youtube.Metadata{Repository: "acme/widget", Release: "v0.2.0"},
		Chapters: []youtube.Chapter{
			{
				Number: 2, Title: "Architecture", Type: "architecture",
				Script:           "The system is event-driven on AWS Lambda and Amazon SQS.",
				StoryboardScenes: []int{2},
				VisualReferences: []string{"Diagram: docs/architecture.md"},
				Callouts: []youtube.Callout{
					{Kind: "Architecture Decision", Text: "Each component has a single responsibility."},
					{Kind: "Best Practice", Text: "Keep the diagram the single source of truth."},
				},
			},
			{
				Number: 3, Title: "Implementation", Type: "implementation",
				Script:           "The builder is a pure Go package behind a Sources port.",
				StoryboardScenes: []int{3},
				Demonstration:    []youtube.DemoStep{{Step: 1, Action: "Open the file"}},
				Callouts: []youtube.Callout{
					{Kind: "Tip", Text: "Keep logic behind a small interface."},
					{Kind: "Common Mistake", Text: "Don't leak infrastructure types into the domain."},
				},
			},
			{
				Number: 4, Title: "Results", Type: "results",
				Script:           "It grounds every downstream artifact in real analysis.",
				StoryboardScenes: []int{4},
				Callouts:         []youtube.Callout{{Kind: "Performance Note", Text: "The win is in reproducibility."}},
			},
		},
	}

	return ReleasePackage{Context: ctx, Blog: post, Storyboard: sb, VoiceOver: vo, YouTube: yt}
}

func newGen() *Generator {
	return &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
}

func TestShortsEndToEnd(t *testing.T) {
	pkg := samplePackage()
	col, err := newGen().YouTubeShorts(context.Background(), pkg)
	if err != nil {
		t.Fatalf("YouTubeShorts: %v", err)
	}

	if col.SchemaVersion != SchemaVersion || col.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", col.Metadata)
	}
	// Multiple Shorts, each a single idea.
	if len(col.Shorts) < 2 {
		t.Fatalf("expected multiple shorts, got %d", len(col.Shorts))
	}
	if col.Metadata.ShortCount != len(col.Shorts) {
		t.Errorf("shortCount %d != %d", col.Metadata.ShortCount, len(col.Shorts))
	}

	// Every Short is fully populated.
	for _, s := range col.Shorts {
		if s.Hook == "" || s.Script == "" || s.CTA == "" {
			t.Errorf("short %d under-populated", s.ID)
		}
		if len(s.Scenes) == 0 || len(s.Captions) == 0 || len(s.Visuals) == 0 {
			t.Errorf("short %d missing scenes/captions/visuals", s.ID)
		}
		if s.DurationSec < shortMinSec || s.DurationSec > shortMaxSec {
			t.Errorf("short %d duration %d outside window", s.ID, s.DurationSec)
		}
	}

	// Validation passes (grounded visuals, no duplicates).
	if probs := col.Validate(pkg); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
	// JSON round-trips.
	blob, err := json.Marshal(col)
	if err != nil || !strings.Contains(string(blob), "\"schemaVersion\":\"1.0.0\"") {
		t.Errorf("marshal: %v", err)
	}
	// Provenance recorded.
	if col.Metadata.SourceSchemas["youTube"] == "" {
		t.Errorf("source schemas = %+v", col.Metadata.SourceSchemas)
	}
}

func TestDiscoveryIsDiverseAndGrounded(t *testing.T) {
	cands := discover(samplePackage(), 6)
	if len(cands) < 3 {
		t.Fatalf("expected several candidates, got %d", len(cands))
	}
	// Angle diversity: no angle repeats in the first pass selection.
	seen := map[string]bool{}
	for _, c := range cands {
		if seen[c.Angle] {
			t.Errorf("angle %q repeated", c.Angle)
		}
		seen[c.Angle] = true
		if collapse(c.Seed) == "" {
			t.Errorf("candidate %q has no grounded seed", c.Angle)
		}
	}
	// The statistic candidate uses real numbers from the context.
	var stat string
	for _, c := range cands {
		if c.Angle == "Interesting Statistic" {
			stat = c.Seed
		}
	}
	if stat != "" && !strings.Contains(stat, "9") {
		t.Errorf("statistic not grounded in commit count: %q", stat)
	}
}

func TestMaxShortsCap(t *testing.T) {
	g := &Generator{MaxShorts: 2, Now: newGen().Now}
	col, _ := g.YouTubeShorts(context.Background(), samplePackage())
	if len(col.Shorts) != 2 {
		t.Errorf("expected 2 shorts (capped), got %d", len(col.Shorts))
	}
}

func TestHookByAngle(t *testing.T) {
	if h := hookDraft(candidate{Angle: "Common Mistake"}); !strings.Contains(strings.ToLower(h), "wrong") {
		t.Errorf("common-mistake hook = %q", h)
	}
	if h := hookDraft(candidate{Angle: "Interesting Statistic", Seed: "This release landed 9 commits."}); !strings.Contains(h, "9") {
		t.Errorf("statistic hook should use the seed: %q", h)
	}
}

func TestDurationAndCaptionTiming(t *testing.T) {
	// Duration clamps to the 30–60s window.
	if d := durationFor(strings.Repeat("word ", 300)); d != shortMaxSec {
		t.Errorf("long script should clamp to %d, got %d", shortMaxSec, d)
	}
	if d := durationFor("just a few words"); d != shortMinSec {
		t.Errorf("short script should floor at %d, got %d", shortMinSec, d)
	}
	// Captions cover the whole duration without overrunning, in order.
	caps := planCaptions("one two three four five six seven eight nine ten", 40)
	if len(caps) == 0 {
		t.Fatal("no captions")
	}
	if caps[0].StartSec != 0 {
		t.Errorf("first caption starts at %d, want 0", caps[0].StartSec)
	}
	if last := caps[len(caps)-1]; last.EndSec != 40 {
		t.Errorf("last caption ends at %d, want 40", last.EndSec)
	}
	prev := 0
	for _, c := range caps {
		if c.StartSec != prev {
			t.Errorf("caption gap: start %d != prev end %d", c.StartSec, prev)
		}
		prev = c.EndSec
	}
}

func TestScenesNonEmpty(t *testing.T) {
	scenes := planScenes(candidate{Angle: "Architecture Reveal"},
		"Hook line.", "Core one. Core two.", "Takeaway.", "Watch more.", 45,
		[]Visual{{Kind: "Architecture Diagram", Description: "The diagram", Reference: "docs/architecture.md"}})
	if len(scenes) < 3 {
		t.Fatalf("expected >=3 scenes, got %d", len(scenes))
	}
	total := 0
	for _, sc := range scenes {
		if collapse(sc.Narration) == "" || collapse(sc.Visual) == "" || sc.Camera == "" {
			t.Errorf("scene %d under-populated: %+v", sc.Number, sc)
		}
		total += sc.DurationSec
	}
	if total != 45 {
		t.Errorf("scene durations sum to %d, want 45", total)
	}
}

func TestHashtagsFocusedAndGrounded(t *testing.T) {
	tags := planHashtags(candidate{Angle: "AWS Best Practice"}, samplePackage())
	if len(tags) == 0 || len(tags) > maxHashtags {
		t.Fatalf("hashtag count %d out of range", len(tags))
	}
	joined := strings.Join(tags, " ")
	if !strings.Contains(joined, "#AWS") {
		t.Errorf("expected #AWS in %v", tags)
	}
	for _, tag := range tags {
		if !strings.HasPrefix(tag, "#") {
			t.Errorf("malformed hashtag %q", tag)
		}
	}
}

func TestCTABrief(t *testing.T) {
	for i, angle := range []string{"Architecture Reveal", "Developer Tip", "Demo Highlight"} {
		cta := planCTA(candidate{Angle: angle}, i)
		if cta == "" || wordCount(cta) > 14 {
			t.Errorf("CTA for %q not brief: %q", angle, cta)
		}
	}
}

func TestMetadata(t *testing.T) {
	col, _ := newGen().YouTubeShorts(context.Background(), samplePackage())
	ci := col.ContentIntelligence
	if ci.ShortCount != len(col.Shorts) || ci.TotalDurationSec == 0 {
		t.Errorf("intelligence = %+v", ci)
	}
	if ci.Audience == "" || ci.Difficulty == "" || len(ci.SEOKeywords) == 0 {
		t.Errorf("collection metadata incomplete: %+v", ci)
	}
	// Per-Short SEO populated.
	for _, s := range col.Shorts {
		if s.SEO.ThumbnailText == "" || s.SEO.Description == "" {
			t.Errorf("short %d SEO incomplete: %+v", s.ID, s.SEO)
		}
	}
}

func TestMarkdownRenders(t *testing.T) {
	col, _ := newGen().YouTubeShorts(context.Background(), samplePackage())
	md := col.Markdown()
	for _, want := range []string{
		"# YouTube Shorts", "## Short 1", "### Script", "### Scene Breakdown",
		"### Captions", "### Visuals", "**CTA:**", "**Hashtags:**", "## Collection Intelligence",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "Sharpened energetic short-form line.", nil
}

func TestModelSharpensHookAndScript(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	col, err := g.YouTubeShorts(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	// At least a hook + script call per short.
	if fm.calls < len(col.Shorts)*2 {
		t.Errorf("model called %d times, want >= %d", fm.calls, len(col.Shorts)*2)
	}
	if col.Shorts[0].Hook != "Sharpened energetic short-form line." {
		t.Errorf("hook not from model: %q", col.Shorts[0].Hook)
	}
}

func TestRequiresContext(t *testing.T) {
	if _, err := newGen().YouTubeShorts(context.Background(), ReleasePackage{}); err == nil {
		t.Error("expected error for nil context")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	pkg := samplePackage()
	bad := ShortsCollection{
		SchemaVersion: "1.0.0",
		Metadata:      Metadata{ShortCount: 1},
		Shorts: []Short{{
			ID: 1, Title: "dup", Hook: "", CTA: "", DurationSec: 90, // no hook/CTA, too long
			Scenes:   []Scene{{Number: 1, Narration: "", Visual: ""}},
			Visuals:  []Visual{{Kind: "X", Reference: "not-grounded"}},
			Captions: nil,
		}},
	}
	probs := bad.Validate(pkg)
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"missing hook", "missing CTA", "no captions", "outside", "empty narration", "is grounded in the release artifacts"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}

func TestValidateRejectsDuplicates(t *testing.T) {
	pkg := samplePackage()
	col, _ := newGen().YouTubeShorts(context.Background(), pkg)
	// Force a duplicate title.
	col.Shorts = append(col.Shorts, col.Shorts[0])
	col.Metadata.ShortCount = len(col.Shorts)
	if !strings.Contains(strings.Join(col.Validate(pkg), "\n"), "duplicate") {
		t.Error("expected duplicate Short to be rejected")
	}
}
