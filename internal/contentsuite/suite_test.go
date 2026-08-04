package contentsuite

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// sampleContext returns a Release Context rich enough to ground every stage,
// including CloudFormation/AWS so the architecture stage has infra to diagram.
func sampleContext() *rc.ReleaseContext {
	return &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		ContextID:     "test-1",
		Repository:    rc.Repository{Owner: "acme", Name: "demo", FullName: "acme/demo", Language: "Go"},
		Release:       rc.Release{Tag: "v1.0.0", Name: "First", Body: "Adds an EventBridge pipeline and an SQS buffer."},
		Architecture: rc.Architecture{
			Overview:    "Event-driven content platform",
			AWSServices: []string{"AWS Lambda", "Amazon SQS", "Amazon EventBridge"},
		},
		Commits:      []rc.Commit{{SHA: "abc123", Subject: "add webhook handler", Type: "feat", Category: "Features"}},
		Technologies: []rc.Technology{{Name: "Go"}, {Name: "AWS Lambda"}},
		CloudFormation: rc.CloudFormationAnalysis{
			Templates: []string{"infra/stack.yaml"},
			Services:  []string{"AWS Lambda", "Amazon SQS", "Amazon EventBridge"},
			Resources: []rc.CFNResource{{LogicalID: "Queue", Type: "AWS::SQS::Queue", Service: "Amazon SQS", Category: "Messaging"}},
		},
		Mermaid: []rc.MermaidDiagram{{Type: "flowchart", Nodes: []string{"A", "B"}, Summary: "Flow"}},
	}
}

// wellFormedBlog has the ## sections the storyboard stage needs.
func wellFormedBlog() *releasegen.BlogPost {
	md := "# First release\n\ntags: [go, aws]\n\nIntro.\n\n## The problem\n\nManual publishing was slow.\n\n## The solution\n\nAn event-driven pipeline on AWS.\n\n## How it works\n\nA webhook triggers EventBridge, buffered by SQS.\n"
	return &releasegen.BlogPost{Title: "First release", Markdown: md}
}

func TestOrchestratorRunsEveryStageOffline(t *testing.T) {
	o := &Orchestrator{} // nil Model => offline/deterministic
	s := o.Run(context.Background(), sampleContext(), wellFormedBlog())

	if s.Manifest.Repository != "acme/demo" || s.Manifest.Release != "v1.0.0" {
		t.Errorf("manifest identity wrong: %+v", s.Manifest)
	}
	if !s.Manifest.Offline {
		t.Error("nil model should mark the run offline")
	}
	// 14 stages M3..M16; all should succeed given a rich context + well-formed blog
	// (the AWS diagram spec + SVG ground on the sample's CloudFormation/AWS/Mermaid).
	if len(s.Manifest.Stages) != 14 {
		t.Fatalf("want 14 stages, got %d", len(s.Manifest.Stages))
	}
	if s.Manifest.Failed != 0 {
		var failed []string
		for _, st := range s.Manifest.Stages {
			if st.Status == StageFailed {
				failed = append(failed, st.Name+": "+st.Error)
			}
		}
		t.Fatalf("expected all stages to pass, %d failed: %v", s.Manifest.Failed, failed)
	}
	if s.Manifest.Produced != 14 || len(s.Artifacts()) != 14 {
		t.Errorf("produced=%d artifacts=%d, want 14", s.Manifest.Produced, len(s.Artifacts()))
	}
	// Milestones are present, in order, and each artifact has content + a filename.
	for i, st := range s.Manifest.Stages {
		if st.Milestone != i+3 {
			t.Errorf("stage %d milestone = %d, want %d", i, st.Milestone, i+3)
		}
	}
	for _, a := range s.Artifacts() {
		if a.Filename == "" || a.Markdown == "" {
			t.Errorf("artifact %q has empty filename/content", a.Kind)
		}
	}
	// Typed results are populated (spot checks across the dependency chain).
	if s.Blog.Markdown == "" || len(s.Storyboard.Scenes) == 0 {
		t.Error("blog/storyboard results not captured")
	}
}

func TestOrchestratorStampsProvenance(t *testing.T) {
	// The Provenance hook must tag each produced artifact with its
	// provider/model/prompt version for the release metadata.
	o := &Orchestrator{
		Provenance: func(kind string) (string, string, string) {
			return "test-provider", "test-model", kind + "@1"
		},
	}
	s := o.Run(context.Background(), sampleContext(), wellFormedBlog())

	if len(s.Artifacts()) == 0 {
		t.Fatal("no artifacts produced")
	}
	for _, a := range s.Artifacts() {
		if a.Provider != "test-provider" || a.Model != "test-model" || a.PromptVersion != a.Kind+"@1" {
			t.Errorf("artifact %q provenance = {%q,%q,%q}", a.Kind, a.Provider, a.Model, a.PromptVersion)
		}
	}
}

func TestOrchestratorIsFaultTolerant(t *testing.T) {
	// An empty blog (title only, no body) makes the storyboard stage fail — but the
	// run must continue and still produce the blog, recording the failure.
	o := &Orchestrator{}
	blog := &releasegen.BlogPost{Title: "x", Markdown: "# x\n"}
	s := o.Run(context.Background(), sampleContext(), blog)

	if s.Manifest.Failed == 0 {
		t.Error("expected at least the storyboard stage to fail")
	}
	// The failure is recorded, not fatal: blog still produced.
	if s.Manifest.Produced == 0 {
		t.Error("a single failed stage must not abort the whole run")
	}
	var storyboardFailed bool
	for _, st := range s.Manifest.Stages {
		if st.Name == "storyboard" {
			storyboardFailed = st.Status == StageFailed && st.Error != ""
		}
	}
	if !storyboardFailed {
		t.Error("storyboard failure should be recorded with an error message")
	}
	// Every stage is still attempted (13 records) even after a mid-chain failure.
	if len(s.Manifest.Stages) != 14 {
		t.Errorf("all 14 stages should be attempted, got %d", len(s.Manifest.Stages))
	}
}

func TestOrchestratorSkipsArchitectureWithoutInfra(t *testing.T) {
	// A release with no groundable infrastructure marks the architecture stage as
	// SKIPPED (a graceful non-failure), not FAILED, and the run still succeeds.
	ctx := sampleContext()
	ctx.Architecture = rc.Architecture{}             // no AWS services
	ctx.CloudFormation = rc.CloudFormationAnalysis{} // no CFN
	o := &Orchestrator{}
	s := o.Run(context.Background(), ctx, wellFormedBlog())

	// Both infra-dependent stages skip gracefully without groundable evidence:
	// the architecture diagram and the AWS diagram specification.
	skipped := map[string]bool{}
	for _, st := range s.Manifest.Stages {
		if st.Status == StageSkipped {
			skipped[st.Name] = true
		}
	}
	if !skipped["architecture"] {
		t.Error("architecture should be SKIPPED (not failed) when there is no infrastructure")
	}
	if !skipped["architecture-diagram-spec"] {
		t.Error("architecture-diagram-spec should be SKIPPED when there is no AWS evidence")
	}
	if s.Manifest.Skipped != 2 {
		t.Errorf("skipped count = %d, want 2", s.Manifest.Skipped)
	}
	// The rest still produced — a skip never aborts the run.
	if s.Manifest.Produced < 10 {
		t.Errorf("produced = %d, want >= 10 despite the skips", s.Manifest.Produced)
	}
}

func TestOrchestratorGeneratesBlogWhenNilAndModelPresent(t *testing.T) {
	// With a model and no supplied blog, the blog stage runs first.
	o := &Orchestrator{Model: fakeModel{}}
	s := o.Run(context.Background(), sampleContext(), nil)
	if s.Manifest.Offline {
		t.Error("a model was supplied; run should not be marked offline")
	}
	if s.Manifest.Stages[0].Name != "blog" || s.Manifest.Stages[0].Status != StageOK {
		t.Errorf("blog stage should run and pass, got %+v", s.Manifest.Stages[0])
	}
	if s.Blog.Markdown == "" {
		t.Error("blog should be generated from the model")
	}
}

func stageNames(s *Suite) map[string]bool {
	got := map[string]bool{}
	for _, st := range s.Manifest.Stages {
		got[st.Name] = true
	}
	return got
}

func TestOrchestratorOnlyRunsSelectedStagePlusDependencies(t *testing.T) {
	cases := []struct {
		name string
		only string
		want []string // exact set of stages that should run
	}{
		// architecture reads only the storyboard's labels (seeded from the release
		// context), so it depends on the blog alone — no model-bound storyboard stage.
		{"architecture", "architecture", []string{"blog", "architecture"}},
		// the diagram spec grounds on the blog alone.
		{"diagram-spec", "architecture-diagram-spec", []string{"blog", "architecture", "architecture-diagram-spec"}},
		// seo sits at the end of the storyboard→voiceover→youtube→…→seo chain.
		{"seo", "seo-metadata", []string{
			"blog", "storyboard", "voiceover", "youtube", "youtube-shorts",
			"tiktok", "visual-assets", "seo-metadata",
		}},
		// linkedin consumes architecture + seo, so it expands to nearly everything
		// except x-thread and the diagram artifacts.
		{"linkedin", "linkedin", []string{
			"blog", "storyboard", "voiceover", "youtube", "youtube-shorts",
			"tiktok", "visual-assets", "seo-metadata", "architecture", "linkedin",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &Orchestrator{Only: map[string]bool{tc.only: true}}
			s := o.Run(context.Background(), sampleContext(), wellFormedBlog())
			got := stageNames(s)
			if len(got) != len(tc.want) {
				t.Fatalf("ran %d stages %v, want %d %v", len(got), sortedKeys(got), len(tc.want), tc.want)
			}
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("expected stage %q to run; ran %v", w, sortedKeys(got))
				}
			}
			// The requested artifact must actually be produced.
			var produced bool
			for _, a := range s.Artifacts() {
				if a.Kind == tc.only {
					produced = true
				}
			}
			if !produced {
				t.Errorf("requested artifact %q was not produced", tc.only)
			}
		})
	}
}

func TestArchitectureKeepsRepoLabelsWithoutStoryboardStage(t *testing.T) {
	// Selecting only architecture skips the storyboard stage, but the
	// storyboard's repo/release labels are seeded from the release context — so
	// the architecture output must still carry the real repo, not the "project"
	// fallback it would use with an empty storyboard.
	o := &Orchestrator{Only: map[string]bool{"architecture": true}}
	s := o.Run(context.Background(), sampleContext(), wellFormedBlog())

	if stageNames(s)["storyboard"] {
		t.Fatal("storyboard stage should not run for an architecture-only selection")
	}
	if s.Storyboard.Metadata.Repository != "acme/demo" || s.Storyboard.Metadata.Release != "v1.0.0" {
		t.Errorf("storyboard labels not seeded from context: %+v", s.Storyboard.Metadata)
	}
	var archMD string
	for _, a := range s.Artifacts() {
		if a.Kind == "architecture" {
			archMD = a.Markdown
		}
	}
	if archMD == "" {
		t.Fatal("architecture artifact not produced")
	}
	if !strings.Contains(archMD, "demo") {
		t.Errorf("architecture output lost the repo label (expected the real repo, not the fallback):\n%s", archMD)
	}
}

// fakeModel returns deterministic Markdown so the blog stage succeeds offline-free.
type fakeModel struct{}

func (fakeModel) Generate(_ context.Context, _ string) (string, error) {
	return "# Generated blog\n\n## Overview\n\nIt works.\n", nil
}

// fakeStore is an in-memory ArtifactStore for reuse tests.
type fakeStore struct {
	data     map[string][]byte
	versions map[string]string
	loads    int
	saves    int
}

func (f *fakeStore) Load(stage string, v any) (string, bool, error) {
	b, ok := f.data[stage]
	if !ok {
		return "", false, nil
	}
	f.loads++
	return f.versions[stage], true, jsonUnmarshal(b, v)
}

func (f *fakeStore) Save(stage string, v any, version string) error {
	if f.data == nil {
		f.data = map[string][]byte{}
	}
	if f.versions == nil {
		f.versions = map[string]string{}
	}
	b, err := jsonMarshal(v)
	if err != nil {
		return err
	}
	f.data[stage] = b
	f.versions[stage] = version
	f.saves++
	return nil
}

type reuseDoc struct{ Title string }

func TestReuseOrRun(t *testing.T) {
	fs := &fakeStore{data: map[string][]byte{}}
	b, _ := jsonMarshal(reuseDoc{Title: "reused"})
	fs.data["dep"] = b
	o := &Orchestrator{Only: map[string]bool{"target": true}, Reuse: true, Store: fs}
	render := func(d reuseDoc) string { return d.Title }

	// A dependency (not requested) with a fresh artifact is reused — gen never runs.
	genCalls := 0
	var dst reuseDoc
	out := reuseOrRun(o, "dep", 1, "dep.md", &dst, render, func() (string, error) {
		genCalls++
		dst = reuseDoc{Title: "generated"}
		return "generated", nil
	})
	if genCalls != 0 {
		t.Errorf("gen called for a reusable dependency")
	}
	if dst.Title != "reused" || out.md != "reused" || out.status != StageOK {
		t.Errorf("dependency not reused: dst=%+v md=%q status=%v", dst, out.md, out.status)
	}

	// The requested target always regenerates and is persisted.
	genCalls = 0
	before := fs.saves
	out = reuseOrRun(o, "target", 1, "t.md", &dst, render, func() (string, error) {
		genCalls++
		dst = reuseDoc{Title: "fresh"}
		return "fresh", nil
	})
	if genCalls != 1 || out.md != "fresh" {
		t.Errorf("requested target should regenerate: calls=%d md=%q", genCalls, out.md)
	}
	if fs.saves != before+1 {
		t.Errorf("freshly generated target should be persisted")
	}

	// With no Only (full run) nothing is a reusable dependency — everything regenerates.
	o2 := &Orchestrator{Reuse: true, Store: fs}
	genCalls = 0
	reuseOrRun(o2, "dep", 1, "dep.md", &dst, render, func() (string, error) { genCalls++; return "x", nil })
	if genCalls != 1 {
		t.Errorf("with no Only set, dependencies must regenerate")
	}
}

func TestIdempotentModeReusesAnyExistingStage(t *testing.T) {
	fs := &fakeStore{data: map[string][]byte{}}
	b, _ := jsonMarshal(reuseDoc{Title: "cached"})
	fs.data["blog"] = b
	// Idempotent full run (no Only): an existing artifact is reused; an absent one
	// is generated and persisted.
	o := &Orchestrator{Store: fs, Idempotent: true}
	render := func(d reuseDoc) string { return d.Title }

	var dst reuseDoc
	genCalls := 0
	out := reuseOrRun(o, "blog", 3, "blog.md", &dst, render, func() (string, error) {
		genCalls++
		return "generated", nil
	})
	if genCalls != 0 || dst.Title != "cached" || out.md != "cached" {
		t.Errorf("existing artifact must be reused (no model call): calls=%d dst=%+v", genCalls, dst)
	}

	genCalls = 0
	out = reuseOrRun(o, "storyboard", 4, "sb.md", &dst, render, func() (string, error) {
		genCalls++
		dst = reuseDoc{Title: "fresh"}
		return "fresh", nil
	})
	if genCalls != 1 || out.md != "fresh" {
		t.Errorf("absent artifact must generate: calls=%d md=%q", genCalls, out.md)
	}
	if _, ok := fs.data["storyboard"]; !ok {
		t.Error("a freshly generated artifact must be persisted for the next run")
	}
}

func TestPromptVersionStaleness(t *testing.T) {
	render := func(d reuseDoc) string { return d.Title }
	prov := func(string) (string, string, string) { return "p", "m", "youtube@2" } // current = @2

	// Idempotent + stored version differs → regenerate, and re-persist at current.
	fs := &fakeStore{}
	_ = fs.Save("youtube", reuseDoc{Title: "old"}, "youtube@1")
	o := &Orchestrator{Store: fs, Idempotent: true, Provenance: prov}
	var dst reuseDoc
	calls := 0
	out := reuseOrRun(o, "youtube", 6, "yt.md", &dst, render, func() (string, error) {
		calls++
		dst = reuseDoc{Title: "new"}
		return "new", nil
	})
	if calls != 1 || out.md != "new" {
		t.Errorf("version change must regenerate: calls=%d md=%q", calls, out.md)
	}
	if v, _, _ := fs.Load("youtube", &dst); v != "youtube@2" {
		t.Errorf("regenerated artifact must be re-saved at the current version, got %q", v)
	}

	// Idempotent + versions match → reuse.
	fs2 := &fakeStore{}
	_ = fs2.Save("youtube", reuseDoc{Title: "cached"}, "youtube@2")
	o2 := &Orchestrator{Store: fs2, Idempotent: true, Provenance: prov}
	calls = 0
	out = reuseOrRun(o2, "youtube", 6, "yt.md", &dst, render, func() (string, error) { calls++; return "x", nil })
	if calls != 0 || out.md != "cached" {
		t.Errorf("matching version must reuse: calls=%d md=%q", calls, out.md)
	}

	// Idempotent + legacy artifact (no stored version) → treated as stale, regenerate.
	fs3 := &fakeStore{data: map[string][]byte{}}
	legacy, _ := jsonMarshal(reuseDoc{Title: "legacy"})
	fs3.data["youtube"] = legacy // no versions entry → ""
	o3 := &Orchestrator{Store: fs3, Idempotent: true, Provenance: prov}
	calls = 0
	_ = reuseOrRun(o3, "youtube", 6, "yt.md", &dst, render, func() (string, error) { calls++; return "refreshed", nil })
	if calls != 1 {
		t.Errorf("legacy (unversioned) artifact must regenerate under idempotent mode: calls=%d", calls)
	}

	// Local targeted dep-reuse (NOT idempotent) must reuse even on a version
	// mismatch — it must never burn model tokens re-running the chain.
	fs4 := &fakeStore{}
	_ = fs4.Save("youtube", reuseDoc{Title: "dep"}, "youtube@1")
	o4 := &Orchestrator{Store: fs4, Reuse: true, Only: map[string]bool{"seo-metadata": true}, Provenance: prov}
	calls = 0
	out = reuseOrRun(o4, "youtube", 6, "yt.md", &dst, render, func() (string, error) { calls++; return "x", nil })
	if calls != 0 || out.md != "dep" {
		t.Errorf("local dep-reuse must reuse regardless of version: calls=%d md=%q", calls, out.md)
	}
}

func TestReuseDep(t *testing.T) {
	o := &Orchestrator{Only: map[string]bool{"seo-metadata": true}, Reuse: true, Store: &fakeStore{}}
	if !o.reuseDep("visual-assets") {
		t.Error("a dependency should be reusable")
	}
	if o.reuseDep("seo-metadata") {
		t.Error("the requested stage must never be reused")
	}
	o.Reuse = false
	if o.reuseDep("visual-assets") {
		t.Error("reuse must be off when Reuse=false")
	}
}

func TestBestArchitectureSVG(t *testing.T) {
	col := architecture.ArchitectureCollection{Diagrams: []architecture.Diagram{
		// Empty SVG — never selected even though it is the most connected.
		{SVG: "", Metadata: architecture.DiagramMeta{NodeCount: 20, EdgeCount: 30}},
		// Single node — skipped (a lone box is not a diagram).
		{SVG: "<svg id=lone/>", Metadata: architecture.DiagramMeta{NodeCount: 1, EdgeCount: 0}},
		// Sparse but valid.
		{SVG: "<svg id=sparse/>", Metadata: architecture.DiagramMeta{NodeCount: 10, EdgeCount: 1, Confidence: 100}},
		// Most connected valid diagram — should win.
		{SVG: "<svg id=rich/>", Metadata: architecture.DiagramMeta{NodeCount: 13, EdgeCount: 12, Confidence: 84}},
	}}
	if got := bestArchitectureSVG(col); got != "<svg id=rich/>" {
		t.Errorf("bestArchitectureSVG = %q, want the most-connected diagram", got)
	}

	// No renderable diagram → empty, so the caller falls through to errNoDiagram.
	none := architecture.ArchitectureCollection{Diagrams: []architecture.Diagram{
		{SVG: "", Metadata: architecture.DiagramMeta{NodeCount: 5, EdgeCount: 5}},
		{SVG: "<svg/>", Metadata: architecture.DiagramMeta{NodeCount: 1}},
	}}
	if got := bestArchitectureSVG(none); got != "" {
		t.Errorf("bestArchitectureSVG with no valid diagram = %q, want empty", got)
	}
}

func jsonMarshal(v any) ([]byte, error)   { return json.Marshal(v) }
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
