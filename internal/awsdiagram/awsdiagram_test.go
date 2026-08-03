package awsdiagram

import (
	"strings"
	"testing"
)

const sampleSpec = "# AWS Architecture Diagram Specification\n\n" +
	"## Components\n\n### Amazon S3\n- **Type:** storage\n\n" +
	"## Connections\n\n" +
	"| # | Source | Target | Protocol / Mechanism | Purpose | Direction | Repository Evidence |\n" +
	"|---|--------|--------|---------------------|---------|-----------|---------------------|\n" +
	"| 1 | Build Pipeline (Dev) | Amazon S3 | Object upload | Stage scripts | Dev → S3 | ev |\n" +
	"| 2 | Amazon EC2 builder | Custom AMI + snapshot | `create-image` | Produce image | EC2 → AMI | ev |\n" +
	"| 3 | Custom AMI + snapshot | Amazon EC2 runtime host | AMI launch | Boot from image | AMI → EC2 | ev |\n" +
	"| 4 | Amazon EC2 runtime host | Amazon CloudWatch | logs/metrics push | Observability | EC2 → CW | ev |\n" +
	"| 5 | AWS IAM | AWS Lambda / Amazon EC2 | policy attachment | scope permissions | IAM → | ev |\n\n" +
	"## Security\n- none\n"

// nameFieldSpec uses the "- Name: <value>" component form (with sibling field
// bullets) that some model outputs produce instead of a bold "**Name**" header.
// Regression: this form previously parsed to zero nodes because the parser only
// recognized bold component headers, so the whole diagram was dropped.
const nameFieldSpec = "# AWS Architecture Diagram Specification\n\n" +
	"## Components\n\n" +
	"### Control Surface\n" +
	"- Name: CLI\n" +
	"  - Type: external\n" +
	"  - AWS Service: External System\n" +
	"### Runtime\n" +
	"- Name: Runtime Host\n" +
	"  - Type: compute\n" +
	"  - AWS Service: Amazon EC2\n\n" +
	"## Connections\n\n" +
	"- Source: **CLI** → Target: **Runtime Host**\n" +
	"  - Protocol/mechanism: HTTPS\n" +
	"  - Purpose: submit a run\n" +
	"  - Direction: CLI → Runtime Host\n" +
	"  - Repository Evidence: cmd/cli\n\n" +
	"## Security\n- none\n"

func TestParseComponentsNameFieldForm(t *testing.T) {
	comps, _ := parseComponents(nameFieldSpec)
	if len(comps) != 2 {
		t.Fatalf("want 2 components from the Name-field form, got %d: %+v", len(comps), comps)
	}
	if comps[0].Name != "CLI" || comps[0].Type != "external" || comps[0].Service != "External System" {
		t.Errorf("component 0 not fully parsed: %+v", comps[0])
	}
	if comps[1].Name != "Runtime Host" || comps[1].Service != "Amazon EC2" {
		t.Errorf("component 1 not fully parsed: %+v", comps[1])
	}
	// End-to-end: the spec now yields nodes + the edge and renders (was empty).
	d := Parse(nameFieldSpec, "Title")
	if len(d.Nodes) < 2 || len(d.Edges) < 1 {
		t.Fatalf("Name-field spec should yield >=2 nodes and >=1 edge, got %d/%d", len(d.Nodes), len(d.Edges))
	}
	if Render(d) == "" {
		t.Error("Name-field spec must render a non-empty SVG")
	}
}

func edge(d Diagram, from, to string) *Edge {
	for i := range d.Edges {
		if d.Edges[i].From == sanitizeID(from) && d.Edges[i].To == sanitizeID(to) {
			return &d.Edges[i]
		}
	}
	return nil
}

func TestParseConnections(t *testing.T) {
	d := Parse(sampleSpec, "Test")

	// Nodes from Source/Target columns (multi-target "/" split into two).
	if len(d.Nodes) < 7 {
		t.Fatalf("expected the connection nodes, got %d: %+v", len(d.Nodes), d.Nodes)
	}
	// The AMI → runtime EC2 boot is the primary (emphasized) edge.
	if e := edge(d, "Custom AMI + snapshot", "Amazon EC2 runtime host"); e == nil || !e.Emphasis {
		t.Errorf("AMI → runtime EC2 should be the emphasized primary edge, got %+v", e)
	}
	// Observability and governance edges are dashed.
	if e := edge(d, "Amazon EC2 runtime host", "Amazon CloudWatch"); e == nil || !e.Dashed {
		t.Errorf("observability edge should be dashed, got %+v", e)
	}
	if e := edge(d, "AWS IAM", "AWS Lambda"); e == nil || !e.Dashed {
		t.Errorf("IAM governance edge should be dashed, got %+v", e)
	}
	// A data-flow edge is solid.
	if e := edge(d, "Build Pipeline (Dev)", "Amazon S3"); e == nil || e.Dashed {
		t.Errorf("data-flow edge should be solid, got %+v", e)
	}
	// Multi-target split produced both Lambda and EC2 targets from row 5.
	if edge(d, "AWS IAM", "Amazon EC2") == nil {
		t.Error("multi-target 'AWS Lambda / Amazon EC2' should yield an IAM→EC2 edge too")
	}
}

func TestRenderProducesSVG(t *testing.T) {
	svg := Render(Parse(sampleSpec, "Release Diagram"))
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "</svg>") {
		t.Fatalf("not a self-contained SVG document")
	}
	for _, want := range []string{"Amazon S3", "Custom AMI", "Release Diagram", "stroke-dasharray", "url(#arrowHi)"} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG missing %q", want)
		}
	}
	// No external references (self-contained).
	if strings.Contains(svg, "http://www.w3.org/1999/xlink") || strings.Contains(svg, "<image") {
		t.Error("SVG must be self-contained (no external assets)")
	}
}

func TestRenderEmptyIsBlank(t *testing.T) {
	if Render(Parse("# Spec\n\nNo connections here.\n", "x")) != "" {
		t.Error("a spec with no connections table should render nothing (caller falls back)")
	}
}
