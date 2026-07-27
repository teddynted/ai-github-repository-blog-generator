package contentsuite

import (
	"context"
	"testing"

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
	// 13 stages M3..M15; all should succeed given a rich context + well-formed blog
	// (the AWS diagram spec + SVG ground on the sample's CloudFormation/AWS/Mermaid).
	if len(s.Manifest.Stages) != 13 {
		t.Fatalf("want 13 stages, got %d", len(s.Manifest.Stages))
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
	if s.Manifest.Produced != 13 || len(s.Artifacts()) != 13 {
		t.Errorf("produced=%d artifacts=%d, want 13", s.Manifest.Produced, len(s.Artifacts()))
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
	if len(s.Manifest.Stages) != 13 {
		t.Errorf("all 13 stages should be attempted, got %d", len(s.Manifest.Stages))
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

// fakeModel returns deterministic Markdown so the blog stage succeeds offline-free.
type fakeModel struct{}

func (fakeModel) Generate(_ context.Context, _ string) (string, error) {
	return "# Generated blog\n\n## Overview\n\nIt works.\n", nil
}
