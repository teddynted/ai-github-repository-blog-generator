package releasecontext

import (
	"strings"
	"testing"
)

func TestAnalyzeCommitsCategorizesAndIgnores(t *testing.T) {
	raw := []RawCommit{
		{SHA: "1", Subject: "feat(api): add POST /process", Author: "Alice", Parents: 1},
		{SHA: "2", Subject: "fix: handle nil head commit", Author: "Bob", Parents: 1},
		{SHA: "3", Subject: "Merge pull request #10 from x", Author: "Bob", Parents: 2},
		{SHA: "4", Subject: "chore(release): v1.2.0", Author: "Bob", Parents: 1},
		{SHA: "5", Subject: "style: gofmt", Author: "Bob", Parents: 1},
		{SHA: "6", Subject: "feat(infra): add scheduler stack", Author: "Alice", Parents: 1},
	}
	commits, stats := analyzeCommits(raw)

	if stats.Total != 6 || stats.Analyzed != 3 || stats.Ignored != 3 {
		t.Fatalf("stats totals = %+v", stats)
	}
	if stats.ByCategory["Features"] != 1 || stats.ByCategory["Bug Fixes"] != 1 || stats.ByCategory["Infrastructure"] != 1 {
		t.Errorf("byCategory = %v", stats.ByCategory)
	}
	if stats.Conventional != 3 {
		t.Errorf("conventional = %d, want 3", stats.Conventional)
	}
	if len(stats.Contributors) != 2 || stats.Contributors[0] != "Alice" {
		t.Errorf("contributors = %v", stats.Contributors)
	}
	// The infra-scoped feat must be promoted to Infrastructure.
	var infra bool
	for _, c := range commits {
		if c.SHA == "6" && c.Category == "Infrastructure" {
			infra = true
		}
	}
	if !infra {
		t.Error("feat(infra) should categorize as Infrastructure")
	}
}

func TestCategorizeFile(t *testing.T) {
	cases := map[string]string{
		"infrastructure/network.yaml":      catInfrastructure,
		".github/workflows/go.yml":         catCICD,
		"docs/architecture.md":             catDocumentation,
		"README.md":                        catDocumentation,
		"internal/foo/bar.go":              catAppCode,
		"internal/foo/bar_test.go":         catTests,
		"lambdas/process/main.go":          catAppCode,
		"go.mod":                           catConfiguration,
		".release.json":                    catConfiguration,
		"docs/assets/brand/diagrams/x.mmd": catDiagrams,
	}
	for path, want := range cases {
		if got := categorizeFile(path); got != want {
			t.Errorf("categorizeFile(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestAnalyzeChangelog(t *testing.T) {
	content := `# Changelog

## [0.2.0] - 2026-07-20

### Features
- add release context builder
- add /process endpoint

### Bug Fixes
- fix nil pointer

### Breaking Changes
- rename envelope field

## [0.1.0] - 2026-07-16

### Features
- initial release
`
	a := analyzeChangelog(content, "v0.2.0")
	if !a.Found || a.Version != "0.2.0" || a.Date != "2026-07-20" {
		t.Fatalf("meta = %+v", a)
	}
	if len(a.Features) != 2 || len(a.BugFixes) != 1 || len(a.BreakingChanges) != 1 {
		t.Errorf("sections = feat:%v fix:%v break:%v", a.Features, a.BugFixes, a.BreakingChanges)
	}
	// Must not bleed into the 0.1.0 section.
	for _, f := range a.Features {
		if f == "initial release" {
			t.Error("leaked into previous version section")
		}
	}
}

func TestAnalyzeChangelogMissing(t *testing.T) {
	if a := analyzeChangelog("# Changelog\n\n## [0.1.0] - x\n", "v9.9.9"); a.Found {
		t.Error("should not find a missing version")
	}
}

func TestAnalyzeMermaidFlowchart(t *testing.T) {
	doc := RawFile{Path: "docs/arch.md", Content: "text\n\n```mermaid\n" +
		"flowchart TB\n" +
		"    A[Webhook] --> B[API Gateway]\n" +
		"    B --> C{Match?}\n" +
		"    C -->|yes| D[EventBridge]\n" +
		"    C -->|no| E[Ignore]\n" +
		"```\n"}
	diagrams := analyzeMermaid([]RawFile{doc})
	if len(diagrams) != 1 {
		t.Fatalf("want 1 diagram, got %d", len(diagrams))
	}
	d := diagrams[0]
	if d.Type != "flowchart" {
		t.Errorf("type = %q", d.Type)
	}
	if d.EdgeCount != 4 {
		t.Errorf("edges = %d, want 4: %+v", d.EdgeCount, d.Edges)
	}
	// Node ids must not absorb spaced labels ("API Gateway" -> node "B").
	for _, n := range d.Nodes {
		if strings.Contains(n, "Gateway") || strings.Contains(n, " ") {
			t.Errorf("node id leaked a label: %q (nodes=%v)", n, d.Nodes)
		}
	}
	if len(d.Nodes) != 5 {
		t.Errorf("nodes = %v, want 5", d.Nodes)
	}
	// Label on the yes/no edges must be captured.
	var labelled int
	for _, e := range d.Edges {
		if e.Label != "" {
			labelled++
		}
	}
	if labelled != 2 {
		t.Errorf("labelled edges = %d, want 2", labelled)
	}
}

func TestAnalyzeCloudFormation(t *testing.T) {
	tmpl := RawFile{Path: "infrastructure/serverless.yaml", Content: `AWSTemplateFormatVersion: "2010-09-09"
Parameters:
  ArtifactsBucket:
    Type: String
Resources:
  Handler:
    Type: AWS::Lambda::Function
    Properties:
      Runtime: provided.al2023
  HandlerRole:
    Type: AWS::IAM::Role
  Queue:
    Type: AWS::SQS::Queue
Outputs:
  QueueUrl:
    Value: !Ref Queue
`}
	a := analyzeCloudFormation([]RawFile{tmpl})
	if len(a.Templates) != 1 {
		t.Fatalf("templates = %v", a.Templates)
	}
	if a.Counts.Resources != 3 || a.Counts.Serverless != 1 || a.Counts.IAM != 1 || a.Counts.Messaging != 1 {
		t.Errorf("counts = %+v", a.Counts)
	}
	if len(a.Parameters) != 1 || a.Parameters[0] != "ArtifactsBucket" {
		t.Errorf("params = %v", a.Parameters)
	}
	if len(a.Outputs) != 1 || a.Outputs[0] != "QueueUrl" {
		t.Errorf("outputs = %v", a.Outputs)
	}
	if !containsString(a.Services, "Lambda") || !containsString(a.Services, "SQS") {
		t.Errorf("services = %v", a.Services)
	}
}

func TestAnalyzeTechnologies(t *testing.T) {
	files := []RawFile{
		{Path: "go.mod", Content: "module x\nrequire github.com/aws/aws-lambda-go v1.54.0\n"},
		{Path: "cmd/worker/main.go", Content: "package main"},
		{Path: "internal/a/a.go", Content: "package a"},
		{Path: "infrastructure/net.yaml", Content: "Resources:\n  X:\n    Type: AWS::EC2::VPC\n"},
		{Path: ".github/workflows/go.yml", Content: "name: go"},
	}
	techs := analyzeTechnologies(files, []string{"Lambda", "S3"})
	names := map[string]Technology{}
	for _, t := range techs {
		names[t.Name] = t
	}
	if _, ok := names["Go"]; !ok {
		t.Error("Go language not detected")
	}
	if _, ok := names["AWS CloudFormation"]; !ok {
		t.Error("CloudFormation not detected")
	}
	if _, ok := names["GitHub Actions"]; !ok {
		t.Error("GitHub Actions not detected")
	}
	if tech, ok := names["AWS Lambda"]; !ok || tech.Category != "AWS Service" {
		t.Errorf("AWS Lambda service = %+v", tech)
	}
	if _, ok := names["AWS Lambda (Go runtime)"]; !ok {
		t.Error("aws-lambda-go dependency not detected")
	}
}

func TestAnalyzeStructure(t *testing.T) {
	files := []RawFile{
		{Path: "cmd/worker/main.go"}, {Path: "internal/a/a.go"},
		{Path: "lambdas/process/main.go"}, {Path: "infrastructure/net.yaml"},
		{Path: "docs/readme.md"}, {Path: "go.mod"},
	}
	rs := analyzeStructure(files)
	if rs.FileCount != 6 {
		t.Errorf("fileCount = %d", rs.FileCount)
	}
	if rs.Layout != "standard-go-serverless-monorepo" {
		t.Errorf("layout = %q", rs.Layout)
	}
	var foundInternal bool
	for _, d := range rs.Directories {
		if d.Path == "internal/" && strings.Contains(d.Responsibility, "business logic") {
			foundInternal = true
		}
	}
	if !foundInternal {
		t.Errorf("internal/ responsibility missing: %+v", rs.Directories)
	}
}

func TestRequestValidate(t *testing.T) {
	if err := (Request{Owner: "acme", Repository: "widget", ReleaseTag: "v1.0.0"}).Validate(); err != nil {
		t.Errorf("valid request rejected: %v", err)
	}
	if err := (Request{Repository: "widget", ReleaseTag: "v1"}).Validate(); err == nil {
		t.Error("missing owner should fail")
	}
	if err := (Request{Owner: "a b", Repository: "w", ReleaseTag: "v1"}).Validate(); err == nil {
		t.Error("owner with space should fail")
	}
}
