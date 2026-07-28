package architecture

import (
	"context"
	"strings"
	"testing"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// hybridPackage grounds a release that runs BOTH local (Ollama) and cloud
// (Bedrock/Claude) inference, so the hybrid-AI framing must engage.
func hybridPackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "platform", FullName: "acme/platform"},
		Release:       rc.Release{Tag: "v0.10.0"},
		Architecture: rc.Architecture{
			Overview:           "An event-driven platform that turns GitHub releases into content using local and cloud AI.",
			AWSServices:        []string{"Amazon SQS", "Amazon Bedrock", "Amazon EC2", "Amazon S3"},
			DeploymentTopology: "Self-hosted automation on EC2 with managed AWS services.",
			EventDrivenFlows:   []string{"GitHub Release -> Amazon SQS -> Ollama -> Amazon Bedrock"},
		},
		Technologies: []rc.Technology{
			{Name: "Ollama", Category: "Tool"},
			{Name: "Anthropic Claude", Category: "Library"},
			{Name: "Go", Category: "Language"},
		},
		CloudFormation: rc.CloudFormationAnalysis{
			Templates: []string{"infra/main.yaml"},
			Services:  []string{"Amazon SQS", "Amazon EC2"},
		},
		Mermaid: []rc.MermaidDiagram{{
			Source: "docs/architecture.md", Type: "flowchart",
			Nodes: []string{"Amazon SQS", "Ollama", "Amazon Bedrock"},
			Edges: []rc.MermaidEdge{
				{From: "Amazon SQS", To: "Ollama", Label: "dispatch"},
				{From: "Ollama", To: "Amazon Bedrock", Label: "fallback"},
			},
		}},
		RepositoryStructure: rc.RepositoryStructure{
			Directories: []rc.DirectoryInfo{{Path: "cmd/worker"}, {Path: "internal/airouter"}},
		},
	}
	return ReleasePackage{Context: ctx, Blog: releasegen.BlogPost{Title: "Inside platform v0.10.0"}}
}

func TestDetectInferenceHybrid(t *testing.T) {
	a := analyze(hybridPackage())
	if !a.Inference.hybrid() {
		t.Fatalf("expected hybrid inference, got local=%v cloud=%v", a.Inference.Local, a.Inference.Cloud)
	}
	if !contains(a.Inference.Local, "Ollama") {
		t.Errorf("local inference should include Ollama, got %v", a.Inference.Local)
	}
	if !contains(a.Inference.Cloud, "Amazon Bedrock") || !contains(a.Inference.Cloud, "Anthropic Claude") {
		t.Errorf("cloud inference should include Bedrock + Claude, got %v", a.Inference.Cloud)
	}
}

func TestHybridMarkdownFramingAndSubgraphs(t *testing.T) {
	col, err := newGen().Architecture(context.Background(), hybridPackage())
	if err != nil {
		t.Fatalf("Architecture: %v", err)
	}
	md := col.Markdown()

	for _, want := range []string{
		"## Platform Overview",
		"hybrid AI platform", // style label
		"Logical Architecture — Hybrid AI Data Flow",
		"- **Local inference:** Ollama",
		"- **Cloud inference:**",
		"- **Operational model:** Self-hosted automation with cloud AI augmentation",
		"- **Primary workflow:**",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("hybrid architecture markdown missing %q\n---\n%s", want, md)
		}
	}

	// The logical data-flow diagram must split Local vs AWS into subgraphs.
	var logical *Diagram
	for i := range col.Diagrams {
		if col.Diagrams[i].Type == "Data Flow Diagram" {
			logical = &col.Diagrams[i]
		}
	}
	if logical == nil {
		t.Fatal("expected a Data Flow Diagram")
	}
	if !strings.Contains(logical.Mermaid, "subgraph") ||
		!strings.Contains(logical.Mermaid, "AWS Cloud") ||
		!strings.Contains(logical.Mermaid, "Local Infrastructure") {
		t.Errorf("logical diagram should have Local + AWS subgraphs:\n%s", logical.Mermaid)
	}
}

func TestNonHybridHasNoAIFraming(t *testing.T) {
	// samplePackage is a plain AWS release (no Ollama, no Bedrock/Claude). It must
	// never gain hybrid-AI framing — the generator must not fabricate it.
	col, err := newGen().Architecture(context.Background(), samplePackage())
	if err != nil {
		t.Fatalf("Architecture: %v", err)
	}
	md := col.Markdown()
	for _, unwanted := range []string{
		"hybrid AI platform",
		"Hybrid AI Data Flow",
		"Local inference",
		"Cloud inference",
	} {
		if strings.Contains(md, unwanted) {
			t.Errorf("non-hybrid architecture markdown should not contain %q\n---\n%s", unwanted, md)
		}
	}
	if len(col.ContentIntelligence.LocalInference) != 0 || len(col.ContentIntelligence.CloudInference) != 0 {
		t.Errorf("non-hybrid release should have no inference lists, got local=%v cloud=%v",
			col.ContentIntelligence.LocalInference, col.ContentIntelligence.CloudInference)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
