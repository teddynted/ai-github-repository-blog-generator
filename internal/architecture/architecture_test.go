package architecture

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

func samplePackage() ReleasePackage {
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "widget", FullName: "acme/widget"},
		Release:       rc.Release{Tag: "v0.2.0"},
		Architecture: rc.Architecture{
			Overview:           "An event-driven pipeline that turns GitHub releases into content.",
			AWSServices:        []string{"AWS Lambda", "Amazon SQS", "Amazon EventBridge", "Amazon DynamoDB", "Amazon S3"},
			DeploymentTopology: "Serverless, deployed via CloudFormation.",
			EventDrivenFlows:   []string{"Amazon EventBridge -> Amazon SQS -> AWS Lambda"},
		},
		CloudFormation: rc.CloudFormationAnalysis{
			Templates: []string{"infra/main.yaml"},
			Services:  []string{"AWS Lambda", "Amazon SQS"},
			Resources: []rc.CFNResource{{LogicalID: "Worker", Type: "AWS::Lambda::Function", Service: "AWS Lambda"}},
		},
		Mermaid: []rc.MermaidDiagram{{
			Source: "docs/architecture.md", Type: "flowchart",
			Nodes: []string{"Amazon EventBridge", "Amazon SQS", "AWS Lambda"},
			Edges: []rc.MermaidEdge{{From: "Amazon EventBridge", To: "Amazon SQS"}, {From: "Amazon SQS", To: "AWS Lambda", Label: "consume"}},
		}},
		RepositoryStructure: rc.RepositoryStructure{
			Directories: []rc.DirectoryInfo{
				{Path: "internal/releasecontext", Responsibility: "Builds the release context"},
				{Path: "cmd/worker", Responsibility: "Runs the pipeline"},
			},
		},
		ContentIntelligence: rc.ContentIntelligence{TargetAudience: "Cloud engineers", ImplementationComplexity: "medium"},
	}
	post := releasegen.BlogPost{Title: "Inside widget v0.2.0"}
	return ReleasePackage{Context: ctx, Blog: post}
}

func newGen() *Generator {
	return &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
}

func TestArchitectureEndToEnd(t *testing.T) {
	pkg := samplePackage()
	col, err := newGen().Architecture(context.Background(), pkg)
	if err != nil {
		t.Fatalf("Architecture: %v", err)
	}

	if col.SchemaVersion != SchemaVersion || col.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", col.Metadata)
	}
	if len(col.Diagrams) < 3 {
		t.Fatalf("expected several diagrams, got %d", len(col.Diagrams))
	}

	for _, d := range col.Diagrams {
		if d.Mermaid == "" || d.Graphviz == "" || d.SVG == "" {
			t.Errorf("diagram %d (%s) missing a render format", d.ID, d.Type)
		}
		if d.Metadata.NodeCount == 0 {
			t.Errorf("diagram %d empty", d.ID)
		}
	}
	if col.ContentIntelligence.ArchitectureStyle == "" || col.ContentIntelligence.DiagramConfidence == 0 {
		t.Errorf("intelligence incomplete: %+v", col.ContentIntelligence)
	}

	if probs := col.Validate(pkg); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}
	blob, err := json.Marshal(col)
	if err != nil || !strings.Contains(string(blob), "\"schemaVersion\":\"1.0.0\"") {
		t.Errorf("marshal: %v", err)
	}
}

func TestDiagramTypesSelected(t *testing.T) {
	col, _ := newGen().Architecture(context.Background(), samplePackage())
	types := map[string]bool{}
	for _, d := range col.Diagrams {
		types[d.Type] = true
	}
	for _, want := range []string{"High-Level Architecture", "Data Flow Diagram", "Event-Driven Architecture", "Component Diagram", "CI/CD Pipeline"} {
		if !types[want] {
			t.Errorf("expected diagram type %q; got %v", want, keys(types))
		}
	}
}

func TestGroundingOnlyContextServices(t *testing.T) {
	col, _ := newGen().Architecture(context.Background(), samplePackage())
	grounded := groundedServiceSet(samplePackage())
	for _, d := range col.Diagrams {
		for _, svc := range d.AWSServices {
			if !serviceGrounded(svc, grounded) {
				t.Errorf("diagram %q names ungrounded service %q", d.Type, svc)
			}
		}
	}
}

func TestNoServicesNoAWSDiagrams(t *testing.T) {
	pkg := samplePackage()
	pkg.Context.Architecture.AWSServices = nil
	pkg.Context.CloudFormation.Services = nil
	pkg.Context.CloudFormation.Templates = nil
	pkg.Context.Architecture.EventDrivenFlows = nil
	pkg.Context.Mermaid = nil
	// Only the repository structure remains groundable → Component Diagram.
	col, err := newGen().Architecture(context.Background(), pkg)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range col.Diagrams {
		if len(d.AWSServices) > 0 {
			t.Errorf("no services in context but diagram %q has AWS services", d.Type)
		}
	}
	if len(col.Warnings) == 0 {
		t.Error("expected a warning about skipped AWS diagrams")
	}
}

func TestDataFlowUsesParsedMermaidEdges(t *testing.T) {
	spec, ok := dataFlow(samplePackage(), analyze(samplePackage()))
	if !ok {
		t.Fatal("data flow should build from parsed mermaid")
	}
	if len(spec.Graph.Edges) != 2 {
		t.Errorf("expected 2 grounded edges, got %d", len(spec.Graph.Edges))
	}
	// Every edge endpoint is a defined node (no orphans/invented relations).
	if probs := validateGraph(spec.Graph); len(probs) != 0 {
		t.Errorf("graph invalid: %v", probs)
	}
}

func TestMermaidRenders(t *testing.T) {
	spec, _ := highLevel(samplePackage(), analyze(samplePackage()))
	m := renderMermaid(spec.Graph, "flowchart")
	if !strings.HasPrefix(m, "flowchart") {
		t.Errorf("mermaid missing directive: %q", firstLine(m))
	}
	if !strings.Contains(m, "subgraph") {
		t.Errorf("expected category subgraphs")
	}
	if err := validateMermaid(m, "flowchart"); err != "" {
		t.Errorf("mermaid invalid: %s", err)
	}
}

func TestSequenceMermaidRenders(t *testing.T) {
	spec, ok := sequence(samplePackage(), analyze(samplePackage()))
	if !ok {
		t.Fatal("sequence should build from the event flow")
	}
	m := renderMermaid(spec.Graph, "sequenceDiagram")
	if !strings.HasPrefix(m, "sequenceDiagram") || !strings.Contains(m, "participant") {
		t.Errorf("sequence diagram malformed:\n%s", m)
	}
	if err := validateMermaid(m, "sequenceDiagram"); err != "" {
		t.Errorf("sequence invalid: %s", err)
	}
}

func TestGraphvizRenders(t *testing.T) {
	spec, _ := highLevel(samplePackage(), analyze(samplePackage()))
	dot := renderGraphviz(spec.Graph)
	if !strings.HasPrefix(dot, "digraph") {
		t.Errorf("dot missing digraph")
	}
	if strings.Count(dot, "{") != strings.Count(dot, "}") {
		t.Errorf("unbalanced braces")
	}
	if err := validateGraphviz(dot); err != "" {
		t.Errorf("graphviz invalid: %s", err)
	}
}

func TestSVGRendersAccessible(t *testing.T) {
	spec, _ := highLevel(samplePackage(), analyze(samplePackage()))
	svg := renderSVG(spec.Graph, spec.Title)
	if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
		t.Errorf("svg malformed")
	}
	for _, want := range []string{"role=\"img\"", "<title>", "<desc>", "viewBox"} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg missing %q", want)
		}
	}
	if err := validateSVG(svg); err != "" {
		t.Errorf("svg invalid: %s", err)
	}
}

func TestPNGExportByShape(t *testing.T) {
	wide, _ := dataFlow(samplePackage(), analyze(samplePackage())) // LR
	if p := planPNG(wide.Graph); p.AspectRatio != "16:9" {
		t.Errorf("LR diagram should export 16:9, got %s", p.AspectRatio)
	}
}

func TestMetadataAndComponents(t *testing.T) {
	col, _ := newGen().Architecture(context.Background(), samplePackage())
	ci := col.ContentIntelligence
	if len(ci.CloudServices) == 0 || len(ci.ServerlessComponents) == 0 {
		t.Errorf("components not categorized: %+v", ci)
	}
	// No AI inference in this sample, but event/messaging services are present →
	// evidence-driven classification is "Event-driven platform".
	if ci.ArchitectureStyle != "Event-driven platform" {
		t.Errorf("architecture style = %q", ci.ArchitectureStyle)
	}
	// A diagram references its grounded resources.
	for _, d := range col.Diagrams {
		if d.Metadata.ReleaseVersion != "v0.2.0" {
			t.Errorf("diagram release version = %q", d.Metadata.ReleaseVersion)
		}
	}
}

func TestMarkdownRenders(t *testing.T) {
	col, _ := newGen().Architecture(context.Background(), samplePackage())
	md := col.Markdown()
	for _, want := range []string{
		"# Architecture Diagrams", "```mermaid", "**AWS services:**", "Graphviz (DOT)",
		"**PNG export:**", "## Architecture Intelligence",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "A grounded description mentioning AWS Lambda and Amazon SQS.", nil
}

func TestModelPolishesDescription(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	col, err := g.Architecture(context.Background(), samplePackage())
	if err != nil {
		t.Fatal(err)
	}
	if fm.calls != len(col.Diagrams) {
		t.Errorf("model called %d times, want %d (one per diagram)", fm.calls, len(col.Diagrams))
	}
	if !strings.Contains(col.Diagrams[0].Description, "grounded description") {
		t.Errorf("description not from model: %q", col.Diagrams[0].Description)
	}
}

func TestRequiresContext(t *testing.T) {
	if _, err := newGen().Architecture(context.Background(), ReleasePackage{}); err == nil {
		t.Error("expected error for nil context")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	pkg := samplePackage()
	bad := ArchitectureCollection{
		SchemaVersion: "1.0.0",
		Metadata:      Metadata{DiagramCount: 1},
		Diagrams: []Diagram{{
			ID: 1, Type: "Bogus",
			Mermaid:     "not a diagram",
			Graphviz:    "not dot",
			SVG:         "not svg",
			AWSServices: []string{"Amazon Redshift"}, // ungrounded
			Metadata:    DiagramMeta{NodeCount: 1},
		}},
	}
	probs := bad.Validate(pkg)
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"not in the release context", "mermaid:", "graphviz:", "svg:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}

func TestValidateGraphCatchesOrphanEdge(t *testing.T) {
	g := graph{Nodes: []gnode{{ID: "a", Label: "A"}}, Edges: []gedge{{From: "a", To: "ghost"}}}
	if probs := validateGraph(g); len(probs) == 0 {
		t.Error("expected orphan-edge problem")
	}
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
