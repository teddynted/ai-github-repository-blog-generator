package youtube

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
)

// samplePackage builds a representative ReleasePackage covering intro,
// architecture (diagram), implementation (code), results, and conclusion scenes.
func samplePackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "widget", FullName: "acme/widget", URL: "https://github.com/acme/widget", Language: "Go", Homepage: "https://widget.dev/docs"},
		Release:       rc.Release{Tag: "v0.2.0"},
		Architecture: rc.Architecture{
			Overview:    "An event-driven pipeline that turns releases into content.",
			AWSServices: []string{"AWS Lambda", "Amazon SQS", "Amazon EventBridge"},
			Components:  []rc.ArchitectureComponent{{Name: "Builder", Responsibility: "Assembles the release context", Kind: "compute"}},
		},
		RepositoryStructure: rc.RepositoryStructure{
			Layout:   "standard-go",
			Overview: "A single Go module with cmd and internal packages.",
			Directories: []rc.DirectoryInfo{
				{Path: "internal/releasecontext", Responsibility: "Builds the release context", FileCount: 6},
			},
		},
		Changelog: rc.ChangelogAnalysis{Found: true, Features: []string{"release context builder", "storyboard generator"}},
		Implementation: rc.ImplementationSummary{
			WhatChanged:           []string{"Added the release context builder"},
			TechnicalImprovements: []string{"Grounded content in real analysis"},
			WhyItMatters:          "It keeps every downstream artifact accurate.",
		},
		Mermaid: []rc.MermaidDiagram{{Source: "docs/architecture.md", Type: "flowchart", Nodes: []string{"A", "B"}, NodeCount: 2}},
		ContentIntelligence: rc.ContentIntelligence{
			Summary:                  "widget v0.2.0 delivers a release context builder.",
			ImplementationComplexity: "medium",
			TargetAudience:           "Cloud engineers",
			TechnicalHighlights:      []string{"grounded content intelligence"},
			ArchitectureHighlights:   []string{"clean event-driven decoupling"},
			SEOKeywords:              []string{"go", "aws", "serverless"},
			BlogTitles:               []string{"Inside widget v0.2.0"},
			DeveloperValue:           "Less manual work per release.",
			FutureEnhancements:       []string{"voice-over and video generation"},
		},
	}
	post := releasegen.BlogPost{Title: "Inside widget v0.2.0", Tags: []string{"go", "aws"}, Markdown: "# Inside widget v0.2.0"}

	mkScene := func(n int, title, typ, narr string, sec int, diagram, code bool) storyboard.Scene {
		sc := storyboard.Scene{
			SceneNumber: n, Title: title, Type: typ, Narration: narr,
			Duration: storyboard.Duration{RecommendedSec: sec, MinSec: sec - 3, MaxSec: sec + 5, Pacing: "medium"},
			Camera:   storyboard.Camera{Direction: "Static"},
		}
		if diagram {
			sc.Diagrams = []storyboard.DiagramRef{{Source: "docs/architecture.md", Type: "flowchart", HighlightNodes: []string{"A", "B"}}}
		}
		if code {
			sc.Code = []storyboard.CodeRef{{Language: "go", Instruction: "Highlight function"}}
		}
		return sc
	}
	scenes := []storyboard.Scene{
		mkScene(1, "Introduction", "introduction", "Welcome to widget v0.2.0.", 60, false, false),
		mkScene(2, "Architecture", "architecture", "The system is event-driven on AWS Lambda and Amazon SQS.", 120, true, false),
		mkScene(3, "Implementation", "implementation", "The builder is a pure Go package behind a Sources port.", 120, false, true),
		mkScene(4, "Results", "results", "It grounds every downstream artifact in real analysis.", 90, false, false),
		mkScene(5, "Conclusion", "conclusion", "That's widget v0.2.0. Try it today.", 45, false, false),
	}
	total := 0
	for _, s := range scenes {
		total += s.Duration.RecommendedSec
	}
	sb := storyboard.Storyboard{
		SchemaVersion:       storyboard.SchemaVersion,
		Metadata:            storyboard.Metadata{Repository: "acme/widget", Release: "v0.2.0", SourceBlogTitle: "Inside widget v0.2.0"},
		Video:               storyboard.VideoSpec{SceneCount: len(scenes), TotalDurationSec: total},
		Scenes:              scenes,
		ContentIntelligence: storyboard.Intelligence{SuggestedTitle: "Inside widget v0.2.0"},
	}

	voScenes := make([]voiceover.Scene, len(scenes))
	for i, s := range scenes {
		voScenes[i] = voiceover.Scene{SceneNumber: s.SceneNumber, Title: s.Title, Narration: s.Narration, Transition: "Let's continue."}
	}
	vo := voiceover.VoiceOverScript{SchemaVersion: voiceover.SchemaVersion, Scenes: voScenes}

	return ReleasePackage{Context: ctx, Blog: post, Storyboard: sb, VoiceOver: vo}
}

func newGen() *Generator {
	return &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
}

func TestYouTubeEndToEnd(t *testing.T) {
	pkg := samplePackage()
	s, err := newGen().YouTube(context.Background(), pkg)
	if err != nil {
		t.Fatalf("YouTube: %v", err)
	}

	if s.SchemaVersion != SchemaVersion || s.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", s.Metadata)
	}
	// One chapter per storyboard scene.
	if len(s.Chapters) != len(pkg.Storyboard.Scenes) {
		t.Fatalf("chapters = %d, want %d", len(s.Chapters), len(pkg.Storyboard.Scenes))
	}
	// Framing sections present.
	if s.Hook.Script == "" || s.Introduction.Script == "" || s.Conclusion.Script == "" || s.CallToAction.Script == "" {
		t.Error("framing sections must be populated")
	}
	// Runtime is internally consistent: max(hook+chapters, spoken-narration time).
	computed := s.Hook.DurationSec
	for _, ch := range s.Chapters {
		computed += ch.Duration.TargetSec
	}
	if speaking := speakingSeconds(totalWords(s), defaultWordsPerMinute); speaking > computed {
		computed = speaking
	}
	if computed != s.Video.DurationSec {
		t.Errorf("runtime %d != computed %d", s.Video.DurationSec, computed)
	}
	// Validation passes.
	if probs := s.Validate(len(pkg.Storyboard.Scenes)); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
	// JSON round-trips.
	blob, err := json.Marshal(s)
	if err != nil || !strings.Contains(string(blob), "\"schemaVersion\":\"1.0.0\"") {
		t.Errorf("marshal: %v", err)
	}
	// Provenance recorded.
	if s.Metadata.SourceSchemas["storyboard"] == "" || s.Metadata.SourceSchemas["voiceOver"] == "" {
		t.Errorf("source schemas = %+v", s.Metadata.SourceSchemas)
	}
}

func TestChaptersRepresentEveryScene(t *testing.T) {
	pkg := samplePackage()
	s, _ := newGen().YouTube(context.Background(), pkg)
	represented := map[int]bool{}
	for _, ch := range s.Chapters {
		for _, n := range ch.StoryboardScenes {
			represented[n] = true
		}
	}
	for n := 1; n <= len(pkg.Storyboard.Scenes); n++ {
		if !represented[n] {
			t.Errorf("scene %d not represented", n)
		}
	}
}

func TestTimestampsSequentialWithHookOffset(t *testing.T) {
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	// First chapter starts exactly at the hook's end.
	if s.Chapters[0].Timestamp.StartSec != s.Hook.DurationSec {
		t.Errorf("chapter 1 start %d != hook end %d", s.Chapters[0].Timestamp.StartSec, s.Hook.DurationSec)
	}
	prev := s.Hook.Timestamp.EndSec
	for i, ch := range s.Chapters {
		if ch.Timestamp.StartSec != prev {
			t.Errorf("chapter %d start %d not contiguous with %d", i+1, ch.Timestamp.StartSec, prev)
		}
		if ch.Timestamp.EndSec <= ch.Timestamp.StartSec {
			t.Errorf("chapter %d end not after start", i+1)
		}
		prev = ch.Timestamp.EndSec
	}
}

func TestHookTypeSelection(t *testing.T) {
	pkg := samplePackage()
	// Medium complexity + architecture highlights => "insight".
	_, draft := hookDraft(pkg)
	if draft == "" {
		t.Fatal("empty hook draft")
	}
	// High complexity => "problem".
	pkg.Context.ContentIntelligence.ImplementationComplexity = "high"
	typ, _ := hookDraft(pkg)
	if typ != "problem" {
		t.Errorf("high complexity hook type = %q, want problem", typ)
	}
	// Hook duration is clamped to 15–30s.
	h := newGen().hook(context.Background(), samplePackage())
	if h.DurationSec < hookMinSec || h.DurationSec > hookMaxSec {
		t.Errorf("hook duration %d outside [%d,%d]", h.DurationSec, hookMinSec, hookMaxSec)
	}
}

func TestChaptersGroundedWalkthrough(t *testing.T) {
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	// Architecture chapter pulls AWS services from the context.
	arch := s.Chapters[1]
	if !strings.Contains(arch.Script, "AWS Lambda") && !strings.Contains(arch.Script, "event-driven") {
		t.Errorf("architecture chapter not grounded: %q", arch.Script)
	}
	// Architecture chapter references the real diagram as a visual.
	var hasDiagram bool
	for _, v := range arch.VisualReferences {
		if strings.Contains(v, "docs/architecture.md") {
			hasDiagram = true
		}
	}
	if !hasDiagram {
		t.Errorf("architecture chapter missing diagram visual: %v", arch.VisualReferences)
	}
	// Implementation chapter gets demonstration steps.
	if len(s.Chapters[2].Demonstration) == 0 {
		t.Error("implementation chapter should have demonstration steps")
	}
}

func TestCalloutsByType(t *testing.T) {
	arch := planCallouts("architecture", samplePackage().Context)
	var hasDecision bool
	for _, c := range arch {
		if c.Kind == "Architecture Decision" {
			hasDecision = true
		}
	}
	if !hasDecision {
		t.Errorf("architecture callouts missing decision: %+v", arch)
	}
	// A type with no callouts returns nothing.
	if got := planCallouts("introduction", samplePackage().Context); len(got) != 0 {
		t.Errorf("introduction should have no callouts, got %+v", got)
	}
}

func TestEngagementCadence(t *testing.T) {
	// Framing chapters never get engagement.
	if got := planEngagement("introduction", 1); got != nil {
		t.Errorf("intro engagement = %v, want nil", got)
	}
	// Body cadence: every engagementEvery-th body chapter.
	if got := planEngagement("architecture", engagementEvery); len(got) == 0 {
		t.Error("expected engagement on cadence hit")
	}
	if got := planEngagement("architecture", 1); engagementEvery > 1 && got != nil {
		t.Errorf("expected no engagement off cadence, got %v", got)
	}
}

func TestCTAGrounded(t *testing.T) {
	cta := newGen().planCTA(samplePackage())
	if cta.Script == "" || len(cta.Items) == 0 {
		t.Fatal("empty CTA")
	}
	var repoURLd bool
	for _, it := range cta.Items {
		if it.Kind == "GitHub Repository" && it.URL == "https://github.com/acme/widget" {
			repoURLd = true
		}
	}
	if !repoURLd {
		t.Errorf("CTA missing grounded repo URL: %+v", cta.Items)
	}
	if cta.PinnedComment == "" {
		t.Error("expected a pinned comment")
	}
}

func TestMetadata(t *testing.T) {
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	ci := s.ContentIntelligence
	if ci.WordCount == 0 || ci.SpeakingTime == "" {
		t.Errorf("intelligence under-populated: %+v", ci)
	}
	if ci.SuggestedTitle == "" || len(ci.SEOKeywords) == 0 || ci.SuggestedDescription == "" {
		t.Errorf("SEO metadata missing: %+v", ci)
	}
	// Chapter markers include the hook and every chapter.
	if len(ci.Chapters) < len(s.Chapters)+1 {
		t.Errorf("chapter markers = %d, want >= %d", len(ci.Chapters), len(s.Chapters)+1)
	}
	// Description embeds the chapter list.
	if !strings.Contains(ci.SuggestedDescription, "Chapters:") {
		t.Errorf("description missing chapters: %q", ci.SuggestedDescription)
	}
}

func TestMarkdownRenders(t *testing.T) {
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	md := s.Markdown()
	for _, want := range []string{
		"# YouTube Script", "## Hook", "## Introduction", "## Chapter 1",
		"### Narration", "## Conclusion", "## Call to Action", "## Production Metadata",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type verboseChapterModel struct{}

func (verboseChapterModel) Generate(_ context.Context, _ string) (string, error) {
	// ~90 sentences of filler — far over every chapter cap.
	return strings.Repeat("The system leans on IAM, Lambda, CloudWatch, and EC2 in a clean layout. ", 40), nil
}

func TestChapterCapsAndRuntimeConsistency(t *testing.T) {
	g := &Generator{Model: verboseChapterModel{}, Now: newGen().Now}
	s, err := g.YouTube(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	// #6: every chapter is capped to its budget (120/140/180 by title).
	for _, ch := range s.Chapters {
		if cap := chapterWordCap(ch.Title); ch.WordCount > cap {
			t.Errorf("chapter %q = %d words, over cap %d", ch.Title, ch.WordCount, cap)
		}
	}
	// #4: runtime metadata is consistent — the impossible combo can't occur.
	if hasRuntimeProblem(s) {
		t.Errorf("runtime inconsistent: durationSec=%d words=%d", s.Video.DurationSec, totalWords(s))
	}
	if probs := s.Validate(len(samplePackage().Storyboard.Scenes)); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
}

func TestDedupeMarkers(t *testing.T) {
	in := []ChapterMarker{
		{Timestamp: "0:00", Title: "Intro / Hook"},
		{Timestamp: "4:31", Title: "Conclusion"},
		{Timestamp: "4:31", Title: "Conclusion"}, // duplicate timestamp
	}
	out := dedupeMarkers(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 markers after dedup, got %d: %+v", len(out), out)
	}
	if !markersUnique(out) {
		t.Error("markers still share a timestamp after dedup")
	}
}

func TestPublishReadinessPASSandYAML(t *testing.T) {
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	p := s.PublishReadiness()
	if p.Status != "PASS" {
		t.Fatalf("expected PASS, got %s: %v", p.Status, p.Errors)
	}
	yaml := p.YAML()
	for _, want := range []string{"validation:", "status: PASS", "runtime_consistent: true", "qa_notes_removed: true"} {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q:\n%s", want, yaml)
		}
	}
}

func TestPublishReadinessFAIL(t *testing.T) {
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	s.Video.DurationSec = 1 // force runtime inconsistency
	s.ContentIntelligence.SuggestedTags = []string{"one"}
	p := s.PublishReadiness()
	if p.Status != "FAIL" {
		t.Fatalf("expected FAIL")
	}
	yaml := p.YAML()
	if !strings.Contains(yaml, "status: FAIL") || !strings.Contains(yaml, "errors:") {
		t.Errorf("FAIL YAML malformed:\n%s", yaml)
	}
}

func TestWarningsNeverLeakIntoMarkdown(t *testing.T) {
	// Internal QA diagnostics must never surface in the viewer-facing artifact.
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	s.Warnings = []string{
		"runtime 4:54 is under the 10-minute long-form target",
		"the upstream blog/storyboard may be too thin for a full long-form video",
	}
	md := s.Markdown()
	for _, banned := range []string{"**Notes:**", "under the", "too thin", "upstream blog"} {
		if strings.Contains(md, banned) {
			t.Errorf("QA diagnostic leaked into markdown: %q", banned)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "Model-expanded engaging teaching narration.", nil
}

func TestModelExpandsNarration(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	s, err := g.YouTube(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	// Hook + intro + conclusion + one per chapter.
	if fm.calls < len(s.Chapters)+3 {
		t.Errorf("model called %d times, want >= %d", fm.calls, len(s.Chapters)+3)
	}
	if s.Chapters[0].Script != "Model-expanded engaging teaching narration." {
		t.Errorf("chapter script not from model: %q", s.Chapters[0].Script)
	}
}

func TestFirstSentencesDoesNotSplitVersions(t *testing.T) {
	// A version number's dots must not be treated as sentence boundaries.
	got := firstSentences("widget v0.2.0 delivers a release context builder. And more.", 1)
	if got != "widget v0.2.0 delivers a release context builder." {
		t.Errorf("firstSentences split a version: %q", got)
	}
}

func TestNarrationHasNoBrokenVersions(t *testing.T) {
	// End-to-end: a v0.2.0 tag must never appear as "v0." in the hook or intro.
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	for _, text := range []string{s.Hook.Script, s.Introduction.Script, s.Conclusion.Script} {
		if strings.Contains(text, "v0. ") || strings.HasSuffix(strings.TrimSpace(text), "v0.") {
			t.Errorf("broken version in narration: %q", text)
		}
	}
}

func TestRequiresContextAndScenes(t *testing.T) {
	if _, err := newGen().YouTube(context.Background(), ReleasePackage{}); err == nil {
		t.Error("expected error for nil context")
	}
	pkg := samplePackage()
	pkg.Storyboard.Scenes = nil
	if _, err := newGen().YouTube(context.Background(), pkg); err == nil {
		t.Error("expected error for storyboard with no scenes")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	bad := YouTubeScript{
		SchemaVersion: "1.0.0",
		Chapters: []Chapter{{
			Number: 1, Title: "x", Script: "", // empty narration
			Timestamp: Timestamp{StartSec: 10, EndSec: 5},
			Duration:  Duration{TargetSec: 30},
		}},
		Video: Video{DurationSec: 999}, // runtime mismatch
	}
	probs := bad.Validate(3)
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"missing introduction", "missing conclusion", "missing call to action", "empty narration", "runtime 999 != max", "not represented"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}

func TestTargetRuntimeWarning(t *testing.T) {
	// The sample (~7 min) is under the 10-minute target, so it warns.
	s, _ := newGen().YouTube(context.Background(), samplePackage())
	if s.Video.DurationSec >= targetMinRuntimeSec {
		t.Skip("sample runtime unexpectedly at/over target")
	}
	if len(s.Warnings) == 0 || !strings.Contains(strings.Join(s.Warnings, " "), "under the") {
		t.Errorf("expected under-target warning, got %v", s.Warnings)
	}
}
