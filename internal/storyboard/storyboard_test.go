package storyboard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

const sampleBlog = "---\n" +
	"title: \"Inside widget v0.2.0\"\n" +
	"description: \"A deep dive into widget v0.2.0.\"\n" +
	"tags: [go, aws-lambda, serverless]\n" +
	"---\n\n" +
	"# Inside widget v0.2.0\n\n" +
	"## Introduction\n\n" +
	"This release adds a release context builder. It grounds all downstream content in real analysis.\n\n" +
	"## Architecture and Design\n\n" +
	"The system is event-driven on AWS Lambda and Amazon SQS, decoupling ingestion from compute.\n\n" +
	"## Implementation Details\n\n" +
	"The builder is a pure package behind a Sources port. Here is the entry point.\n\n" +
	"```go\n" +
	"func (b *Builder) Build(ctx context.Context, req Request) (*ReleaseContext, error) {\n" +
	"    return assemble(req)\n" +
	"}\n" +
	"```\n\n" +
	"## Architecture Diagrams\n\n" +
	"```mermaid\n" +
	"flowchart TD\n" +
	"    A --> B\n" +
	"    B -->|match| C\n" +
	"```\n\n" +
	"## Conclusion\n\n" +
	"That's widget v0.2.0. Try it on your own repository.\n"

func samplePost() releasegen.BlogPost {
	return releasegen.BlogPost{
		Title:           "Inside widget v0.2.0: What Changed and Why It Matters",
		MetaDescription: "A deep dive into widget v0.2.0 and its event-driven architecture.",
		Tags:            []string{"go", "aws-lambda", "serverless"},
		Markdown:        sampleBlog,
	}
}

func sampleContext() *rc.ReleaseContext {
	return &rc.ReleaseContext{
		Repository:   rc.Repository{Name: "widget", FullName: "acme/widget"},
		Release:      rc.Release{Tag: "v0.2.0"},
		CommitStats:  rc.CommitStats{Analyzed: 5, ByCategory: map[string]int{"Features": 3}},
		FileStats:    rc.FileStats{Total: 12},
		Architecture: rc.Architecture{AWSServices: []string{"AWS Lambda", "Amazon SQS", "Amazon EventBridge"}},
		Changelog:    rc.ChangelogAnalysis{Found: true, Features: []string{"release context builder", "process release tag"}},
		Mermaid: []rc.MermaidDiagram{{
			Source: "docs/architecture.md", Type: "flowchart",
			Nodes:     []string{"A", "B", "C"},
			Edges:     []rc.MermaidEdge{{From: "A", To: "B"}, {From: "B", To: "C", Label: "match"}},
			NodeCount: 3, EdgeCount: 2,
		}},
		ContentIntelligence: rc.ContentIntelligence{
			ImplementationComplexity: "medium", TargetAudience: "Cloud engineers",
			TechnicalHighlights: []string{"grounded content intelligence"},
			SEOKeywords:         []string{"go", "aws", "serverless"},
			Summary:             "widget v0.2.0 delivers a release context builder.",
		},
	}
}

func TestExtractSections(t *testing.T) {
	secs := extractSections(sampleBlog)
	if len(secs) != 5 {
		t.Fatalf("sections = %d, want 5: %+v", len(secs), titles(secs))
	}
	want := []string{"Introduction", "Architecture and Design", "Implementation Details", "Architecture Diagrams", "Conclusion"}
	for i, w := range want {
		if secs[i].Title != w {
			t.Errorf("section %d = %q, want %q", i, secs[i].Title, w)
		}
	}
	// Front matter and H1 must be stripped (no "title:" leaking into the first body).
	if strings.Contains(secs[0].Body, "title:") || strings.Contains(secs[0].Body, "# Inside") {
		t.Errorf("front matter/H1 leaked into body: %q", secs[0].Body)
	}
}

func TestSceneType(t *testing.T) {
	cases := map[string]string{
		"Introduction":                     "introduction",
		"Background and Context":           "problem",
		"Architecture and Design":          "architecture",
		"Architecture Diagrams":            "diagram",
		"CloudFormation Topology":          "cloudformation",
		"Repository and Code Changes":      "repository",
		"Implementation Details":           "implementation",
		"Benefits and Outcomes":            "results",
		"How to Use or Extend the Feature": "lessons",
		"Conclusion":                       "conclusion",
		"Something Else":                   "generic",
	}
	for title, want := range cases {
		if got := sceneType(title); got != want {
			t.Errorf("sceneType(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestPlanDuration(t *testing.T) {
	// ~26 words at 2.6 wps ≈ 10s.
	d := planDuration(strings.Repeat("word ", 26), 2.6)
	if d.RecommendedSec != 10 {
		t.Errorf("recommended = %d, want 10", d.RecommendedSec)
	}
	if d.MinSec > d.RecommendedSec || d.MaxSec < d.RecommendedSec {
		t.Errorf("window [%d,%d] must contain %d", d.MinSec, d.MaxSec, d.RecommendedSec)
	}
	// Clamp: a huge narration caps at 25s; an empty one floors at 5s.
	if planDuration(strings.Repeat("word ", 500), 2.6).RecommendedSec != 25 {
		t.Error("long narration should clamp to 25s")
	}
	if planDuration("", 2.6).RecommendedSec != 5 {
		t.Error("empty narration should floor at 5s")
	}
}

func TestPlanDiagramsUsesRealDiagramsOnly(t *testing.T) {
	dgs := sampleContext().Mermaid
	// Non-architecture scene: no diagram references.
	if refs := planDiagrams("introduction", dgs); refs != nil {
		t.Errorf("introduction should have no diagrams, got %+v", refs)
	}
	// Architecture scene: references the real diagram + its real nodes.
	refs := planDiagrams("architecture", dgs)
	if len(refs) != 1 || refs[0].Source != "docs/architecture.md" {
		t.Fatalf("refs = %+v", refs)
	}
	if strings.Join(refs[0].HighlightNodes, ",") != "A,B,C" {
		t.Errorf("highlight nodes = %v, want the parsed nodes A,B,C", refs[0].HighlightNodes)
	}
	if refs[0].ZoomTarget != "A" || refs[0].Animation != "Diagram Build" {
		t.Errorf("ref = %+v", refs[0])
	}
}

func TestPlanCode(t *testing.T) {
	body := "text\n\n```go\nfunc f() {}\n```\n\n```mermaid\nflowchart TD\nA-->B\n```\n\n```yaml\nResources:\n  X:\n    Type: AWS::S3::Bucket\n```"
	refs := planCode(body)
	if len(refs) != 2 { // go + yaml; mermaid excluded
		t.Fatalf("code refs = %d, want 2: %+v", len(refs), refs)
	}
	if refs[0].Language != "go" || refs[0].Instruction != "Highlight function" {
		t.Errorf("go ref = %+v", refs[0])
	}
	if refs[1].Language != "yaml" || refs[1].Instruction != "Zoom into CloudFormation resource" {
		t.Errorf("cfn ref = %+v", refs[1])
	}
}

func TestPlanAnimationsHighlightsRealNodes(t *testing.T) {
	refs := planDiagrams("architecture", sampleContext().Mermaid)
	anims := planAnimations("architecture", refs, nil, false)
	var highlighted []string
	for _, a := range anims {
		if a.Type == "Highlight Node" {
			highlighted = append(highlighted, a.Target)
		}
	}
	if strings.Join(highlighted, ",") != "A,B,C" {
		t.Errorf("highlighted nodes = %v, want A,B,C", highlighted)
	}
	// Sequences must be monotonic starting at 1.
	for i, a := range anims {
		if a.Sequence != i+1 {
			t.Errorf("animation %d has sequence %d", i, a.Sequence)
		}
	}
}

func TestGeneratorEndToEnd(t *testing.T) {
	g := &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
	sb, err := g.Storyboard(context.Background(), samplePost(), sampleContext())
	if err != nil {
		t.Fatalf("Storyboard: %v", err)
	}
	if sb.SchemaVersion != SchemaVersion || sb.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", sb.Metadata)
	}
	if len(sb.Scenes) != 5 || sb.Video.SceneCount != 5 {
		t.Fatalf("scenes = %d", len(sb.Scenes))
	}
	// Types in order.
	gotTypes := make([]string, len(sb.Scenes))
	for i, s := range sb.Scenes {
		gotTypes[i] = s.Type
	}
	if strings.Join(gotTypes, ",") != "introduction,architecture,implementation,diagram,conclusion" {
		t.Errorf("scene types = %v", gotTypes)
	}
	// Architecture + diagram scenes reference the real diagram; others don't.
	if len(sb.Scenes[1].Diagrams) != 1 || len(sb.Scenes[3].Diagrams) != 1 {
		t.Error("architecture/diagram scenes should reference the parsed diagram")
	}
	if len(sb.Scenes[0].Diagrams) != 0 {
		t.Error("introduction should not reference a diagram")
	}
	// Implementation scene picks up the Go code block.
	if len(sb.Scenes[2].Code) != 1 || sb.Scenes[2].Code[0].Language != "go" {
		t.Errorf("implementation code = %+v", sb.Scenes[2].Code)
	}
	// Every scene: narration + duration + camera + transition set.
	for i, s := range sb.Scenes {
		if s.Narration == "" || s.Camera.Direction == "" || s.Transition.Type == "" {
			t.Errorf("scene %d under-populated: %+v", i+1, s)
		}
	}
	// Structural validation passes.
	if probs := sb.Validate(); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
	// JSON round-trips (it is the canonical downstream input).
	blob, err := json.Marshal(sb)
	if err != nil || !strings.Contains(string(blob), "\"schemaVersion\":\"1.0.0\"") {
		t.Errorf("marshal: %v", err)
	}
	// Content intelligence populated.
	if sb.ContentIntelligence.EstimatedVideoLength == "" || len(sb.ContentIntelligence.Chapters) != 5 {
		t.Errorf("intelligence = %+v", sb.ContentIntelligence)
	}
	// Markdown renders scenes.
	md := sb.Markdown()
	for _, want := range []string{"# Storyboard", "## Scene 1", "**Narration:**", "**Camera:**", "## Chapters"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "Polished spoken narration for this scene.", nil
}

func TestGeneratorUsesModelForNarration(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm}
	sb, err := g.Storyboard(context.Background(), samplePost(), sampleContext())
	if err != nil {
		t.Fatal(err)
	}
	if fm.calls != len(sb.Scenes) {
		t.Errorf("model called %d times, want %d (one per scene)", fm.calls, len(sb.Scenes))
	}
	if sb.Scenes[0].Narration != "Polished spoken narration for this scene." {
		t.Errorf("narration not taken from model: %q", sb.Scenes[0].Narration)
	}
}

func TestGeneratorRequiresContext(t *testing.T) {
	if _, err := (&Generator{}).Storyboard(context.Background(), samplePost(), nil); err == nil {
		t.Error("expected error for nil release context")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	bad := Storyboard{SchemaVersion: "1.0.0", Scenes: []Scene{{SceneNumber: 2, Title: "x"}}}
	bad.Video.SceneCount = 5
	probs := bad.Validate()
	if len(probs) == 0 {
		t.Fatal("expected validation problems")
	}
}

func TestStoryboardFallsBackWhenNoSections(t *testing.T) {
	g := &Generator{}
	// A short blog with no ## headings must still produce a one-scene storyboard
	// (grounded in the body) rather than failing.
	post := releasegen.BlogPost{Title: "Small fix", Markdown: "# Small fix\n\nThis release fixes a race in the SQS drainer.\n"}
	sb, err := g.Storyboard(context.Background(), post, sampleContext())
	if err != nil {
		t.Fatalf("section-less blog should not error: %v", err)
	}
	if len(sb.Scenes) != 1 {
		t.Errorf("want 1 fallback scene, got %d", len(sb.Scenes))
	}
	var warned bool
	for _, w := range sb.Warnings {
		if strings.Contains(w, "no ## sections") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("fallback should record a warning, got %v", sb.Warnings)
	}
}

func TestStoryboardStillErrorsOnEmptyBlog(t *testing.T) {
	g := &Generator{}
	// No body at all — nothing groundable to scene, so it still errors honestly.
	post := releasegen.BlogPost{Title: "x", Markdown: "# x\n"}
	if _, err := g.Storyboard(context.Background(), post, sampleContext()); err == nil {
		t.Error("an empty blog should still error")
	}
}

func titles(secs []section) []string {
	out := make([]string, len(secs))
	for i, s := range secs {
		out[i] = s.Title
	}
	return out
}
