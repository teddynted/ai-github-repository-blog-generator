package archdiagram

import (
	"context"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

func richSnapshot(t *testing.T) processing.Snapshot {
	root := writeRepo(t, map[string]string{
		"infra/main.tf": `
resource "aws_instance" "app" {}
resource "aws_s3_bucket" "assets" {}
resource "aws_db_instance" "db" {}
resource "aws_sqs_queue" "jobs" {}
resource "aws_api_gateway_rest_api" "api" {}
`,
		".github/workflows/deploy.yml": "on: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n",
		"README.md":                    "AI blog service using an LLM (Ollama) with a REST API.",
	})
	return processing.Snapshot{
		RepoFullName: "acme/widget",
		LocalPath:    root,
		Readme:       "AI blog service using an LLM (Ollama) with a REST API.",
		Analysis: processing.Analysis{
			Languages: []string{"Go"},
			CICD:      []string{"GitHub Actions"},
		},
	}
}

func TestGenerateProducesValidMermaidForEveryDiagram(t *testing.T) {
	res := Generate(richSnapshot(t))
	if len(res.Diagrams) == 0 {
		t.Fatal("expected diagrams")
	}
	// Re-validate each rendered diagram by round-tripping through structural checks:
	// every diagram must be non-empty and declare a flowchart header.
	for _, d := range res.Diagrams {
		if !strings.HasPrefix(d.Mermaid, "flowchart ") {
			t.Errorf("diagram %s not a flowchart:\n%s", d.Type, d.Mermaid)
		}
		if d.Title == "" || d.Description == "" || d.Purpose == "" {
			t.Errorf("diagram %s missing prose fields", d.Type)
		}
	}
}

func TestGenerateSelectsConditionalDiagrams(t *testing.T) {
	res := Generate(richSnapshot(t))
	types := map[string]bool{}
	for _, d := range res.Diagrams {
		types[d.Type] = true
	}
	for _, want := range []string{"aws-solution", "component", "data-flow", "deployment", "cicd", "ai-workflow"} {
		if !types[want] {
			t.Errorf("expected %s diagram to be present", want)
		}
	}
}

func TestPrimaryDiagramExcludesLowConfidence(t *testing.T) {
	det := Detect(richSnapshot(t))
	// Inject a Low-confidence service and confirm it never reaches the primary diagram.
	det.Services = append(det.Services, Service{
		ID: "cloudfront", Name: "Amazon CloudFront", Logical: "CDN",
		Confidence: Low, Evidence: []string{"guess"},
	})
	dg, ok := det.awsSolution()
	if !ok {
		t.Fatal("awsSolution failed to build")
	}
	if strings.Contains(dg.Mermaid, "CloudFront") {
		t.Errorf("Low-confidence service leaked into primary diagram:\n%s", dg.Mermaid)
	}
}

func TestMarkdownHasRequiredSections(t *testing.T) {
	md := Generate(richSnapshot(t)).Markdown()
	for _, want := range []string{
		"# AWS Architecture",
		"## Detected Components",
		"## AWS Service Mapping",
		"## AWS Solution Architecture",
		"## Confidence Report",
		"## Repository Evidence",
		"```mermaid",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown missing section %q", want)
		}
	}
}

func TestDiagrammerReturnsAssetOrSkips(t *testing.T) {
	// Rich repo → an architecture-diagram asset.
	asset, ok, err := Diagrammer{}.Diagram(context.Background(), richSnapshot(t))
	if err != nil || !ok {
		t.Fatalf("expected an asset: ok=%v err=%v", ok, err)
	}
	if asset.Kind != generation.KindArchitectureDiagram {
		t.Errorf("asset kind = %q, want %q", asset.Kind, generation.KindArchitectureDiagram)
	}
	if !strings.Contains(asset.Markdown, "```mermaid") {
		t.Error("asset markdown should embed a mermaid diagram")
	}
}
