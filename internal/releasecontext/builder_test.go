package releasecontext

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// fakeSources returns canned data so the builder can be tested end-to-end
// without GitHub or git.
type fakeSources struct {
	repo    RawRepository
	rel     RawRelease
	commits []RawCommit
	changed []RawChangedFile
	files   []RawFile
	err     error
}

func (f *fakeSources) RepositoryMeta(context.Context, Request) (RawRepository, error) {
	return f.repo, f.err
}
func (f *fakeSources) ReleaseByTag(context.Context, Request) (RawRelease, error) {
	return f.rel, f.err
}
func (f *fakeSources) CommitsBetween(context.Context, Request, string) ([]RawCommit, error) {
	return f.commits, f.err
}
func (f *fakeSources) ChangedFilesBetween(context.Context, Request, string) ([]RawChangedFile, error) {
	return f.changed, f.err
}
func (f *fakeSources) Files(context.Context, Request) ([]RawFile, error) { return f.files, f.err }

func sampleSources() *fakeSources {
	return &fakeSources{
		repo: RawRepository{
			Owner: "teddynted", Name: "ai-github-repository-blog-generator",
			FullName:    "teddynted/ai-github-repository-blog-generator",
			Description: "Turns GitHub repositories into technical content",
			Language:    "Go", License: "MIT", DefaultBranch: "main", Visibility: "public",
			Topics: []string{"aws", "golang", "serverless"},
		},
		rel: RawRelease{Tag: "v0.2.0", Name: "v0.2.0", Author: "teddynted",
			PublishedAt: "2026-07-20T10:00:00Z", PreviousTag: "v0.1.0", Body: "Release context builder."},
		commits: []RawCommit{
			{SHA: "a1", Subject: "feat(api): add POST /process endpoint", Author: "Teddy", Parents: 1},
			{SHA: "a2", Subject: "feat(context): build release context", Author: "Teddy", Parents: 1},
			{SHA: "a3", Subject: "fix: handle empty changelog", Author: "Teddy", Parents: 1},
			{SHA: "a4", Subject: "Merge pull request #1", Author: "Teddy", Parents: 2},
			{SHA: "a5", Subject: "chore(release): v0.2.0", Author: "Teddy", Parents: 1},
			{SHA: "a6", Subject: "docs: document the schema", Author: "Teddy", Parents: 1},
		},
		changed: []RawChangedFile{
			{Path: "internal/releasecontext/builder.go", Status: "added", Additions: 200},
			{Path: "lambdas/process/main.go", Status: "added", Additions: 120},
			{Path: "infrastructure/serverless.yaml", Status: "modified", Additions: 40, Deletions: 5},
			{Path: "docs/release-context.md", Status: "added", Additions: 90},
			{Path: "internal/releasecontext/builder_test.go", Status: "added", Additions: 150},
		},
		files: []RawFile{
			{Path: "README.md", Content: "# Project\n\nAn event-driven, serverless platform using Clean Architecture and Infrastructure as Code.\n\n## Goals\n\n- reliability\n"},
			{Path: "CHANGELOG.md", Content: "# Changelog\n\n## [0.2.0] - 2026-07-20\n\n### Features\n- release context builder\n\n### Bug Fixes\n- empty changelog handling\n"},
			{Path: "go.mod", Content: "module x\nrequire github.com/aws/aws-lambda-go v1.54.0\n"},
			{Path: "cmd/worker/main.go", Content: "package main"},
			{Path: "internal/releasecontext/builder.go", Content: "package releasecontext"},
			{Path: "lambdas/process/main.go", Content: "package main"},
			{Path: "infrastructure/serverless.yaml", Content: "AWSTemplateFormatVersion: \"2010-09-09\"\nResources:\n  Handler:\n    Type: AWS::Lambda::Function\n  Role:\n    Type: AWS::IAM::Role\n  Queue:\n    Type: AWS::SQS::Queue\n  Bus:\n    Type: AWS::Events::EventBus\n"},
			{Path: "docs/architecture.md", Content: "# Architecture\n\n```mermaid\nflowchart TB\n    A[Webhook] --> B[API Gateway]\n    B --> C[EventBridge]\n```\n"},
		},
	}
}

func TestBuilderBuild(t *testing.T) {
	b := &Builder{Sources: sampleSources(), Now: func() time.Time {
		return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	}}
	rc, err := b.Build(context.Background(), Request{Owner: "teddynted", Repository: "ai-github-repository-blog-generator", ReleaseTag: "v0.2.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if rc.SchemaVersion != SchemaVersion || rc.ContextID == "" {
		t.Errorf("envelope = %q / %q", rc.SchemaVersion, rc.ContextID)
	}
	if rc.GeneratedAt != "2026-07-20T12:00:00Z" {
		t.Errorf("generatedAt = %q", rc.GeneratedAt)
	}
	if rc.Repository.Summary == "" || !strings.Contains(rc.Repository.Summary, "MIT") {
		t.Errorf("repo summary = %q", rc.Repository.Summary)
	}
	if rc.CommitStats.Analyzed != 4 { // 6 - merge - bump
		t.Errorf("analyzed commits = %d, want 4", rc.CommitStats.Analyzed)
	}
	if rc.CommitStats.ByCategory["Features"] != 2 {
		t.Errorf("features = %d, want 2", rc.CommitStats.ByCategory["Features"])
	}
	if !rc.Changelog.Found || len(rc.Changelog.Features) != 1 {
		t.Errorf("changelog = %+v", rc.Changelog)
	}
	if rc.CloudFormation.Counts.Resources != 4 {
		t.Errorf("cfn resources = %d, want 4", rc.CloudFormation.Counts.Resources)
	}
	if len(rc.Mermaid) != 1 || rc.Mermaid[0].Type != "flowchart" {
		t.Errorf("mermaid = %+v", rc.Mermaid)
	}
	if !containsString(rc.Architecture.AWSServices, "AWS Lambda") {
		t.Errorf("aws services = %v", rc.Architecture.AWSServices)
	}
	if rc.ContentIntelligence.Summary == "" {
		t.Error("content intelligence summary is empty")
	}
	if len(rc.ContentIntelligence.BlogTitles) == 0 || len(rc.ContentIntelligence.SEOKeywords) == 0 {
		t.Error("content intelligence missing blog titles / seo keywords")
	}
	if rc.ContentIntelligence.ImplementationComplexity == "" {
		t.Error("complexity not set")
	}

	// The whole context must round-trip through JSON (it is the API payload).
	blob, err := json.Marshal(rc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(blob), "\"schemaVersion\":\"2.0.0\"") {
		t.Error("schemaVersion not serialized")
	}
}

func TestBuilderValidatesRequest(t *testing.T) {
	b := &Builder{Sources: sampleSources()}
	if _, err := b.Build(context.Background(), Request{Owner: "", Repository: "x", ReleaseTag: "v1"}); err == nil {
		t.Error("expected validation error for empty owner")
	}
}

func TestBuilderWarnsOnMissingData(t *testing.T) {
	src := sampleSources()
	src.files = []RawFile{{Path: "cmd/main.go", Content: "package main"}} // no changelog, no cfn
	src.commits = nil
	b := &Builder{Sources: src}
	rc, err := b.Build(context.Background(), Request{Owner: "a", Repository: "b", ReleaseTag: "v0.2.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	joined := strings.Join(rc.Warnings, " | ")
	for _, want := range []string{"CHANGELOG", "CloudFormation", "commits"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing warning about %q in: %s", want, joined)
		}
	}
}
