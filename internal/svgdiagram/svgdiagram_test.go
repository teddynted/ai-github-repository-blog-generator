package svgdiagram

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// assertWellFormedXML fails if s is not parseable XML — the strongest guarantee
// that the emitted SVG is valid and self-contained.
func assertWellFormedXML(t *testing.T, s string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("SVG is not well-formed XML: %v\n---\n%s", err, s)
		}
	}
}

func sampleGraph() Graph {
	return Graph{
		Title: "Release pipeline",
		Nodes: []Node{
			{ID: "EventBridge", Label: "EventBridge", Group: "messaging"},
			{ID: "Queue", Label: "SQS Queue", Group: "messaging"},
			{ID: "Worker", Label: "EC2 Worker", Group: "compute"},
			{ID: "Bucket", Label: "S3 Bucket", Group: "storage"},
		},
		Edges: []Edge{
			{From: "EventBridge", To: "Queue"},
			{From: "Queue", To: "Worker", Label: "poll"},
			{From: "Worker", To: "Bucket", Label: "publish"},
		},
	}
}

func TestRenderProducesWellFormedSelfContainedSVG(t *testing.T) {
	out := Render(sampleGraph())

	if !strings.HasPrefix(out, "<svg") || !strings.HasSuffix(out, "</svg>") {
		t.Fatalf("output is not a standalone <svg> document:\n%s", out)
	}
	assertWellFormedXML(t, out)

	if !strings.Contains(out, "viewBox=") {
		t.Error("SVG missing viewBox (needed for scalable rendering)")
	}
	// Node labels and an edge label are drawn.
	for _, want := range []string{"EC2 Worker", "S3 Bucket", "poll", "publish"} {
		if !strings.Contains(out, want) {
			t.Errorf("SVG missing content %q", want)
		}
	}
	// Edges are rendered with arrowheads.
	if !strings.Contains(out, "marker-end=\"url(#arrow)\"") || !strings.Contains(out, "<line") {
		t.Error("SVG missing arrowed edges")
	}

	// Self-contained: no external references or scripts.
	for _, forbidden := range []string{"<script", "href=", "<image", "url(http"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("SVG is not self-contained: contains %q", forbidden)
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	g := sampleGraph()
	if Render(g) != Render(g) {
		t.Error("Render is not deterministic for identical input")
	}
}

func TestRenderEscapesText(t *testing.T) {
	g := Graph{
		Title: `A & B <topic>`,
		Nodes: []Node{{ID: "n1", Label: `<script>alert('x')&`}},
	}
	out := Render(g)
	assertWellFormedXML(t, out)
	if strings.Contains(out, "<script>") {
		t.Error("unescaped markup leaked into the SVG")
	}
	if !strings.Contains(out, "&amp;") {
		t.Error("ampersand was not escaped")
	}
}

func TestRenderAddsDanglingEdgeEndpoints(t *testing.T) {
	// "Sink" appears only as an edge target, never declared as a node — it must
	// still be drawn so the edge doesn't dangle.
	g := Graph{
		Nodes: []Node{{ID: "Source", Label: "Source"}},
		Edges: []Edge{{From: "Source", To: "Sink"}},
	}
	out := Render(g)
	assertWellFormedXML(t, out)
	if !strings.Contains(out, "Sink") {
		t.Error("edge endpoint not declared as a node was dropped")
	}
}

func TestRenderCycleSafe(t *testing.T) {
	// A cycle must not hang layer assignment.
	g := Graph{
		Nodes: []Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
		Edges: []Edge{{From: "A", To: "B"}, {From: "B", To: "C"}, {From: "C", To: "A"}},
	}
	out := Render(g)
	assertWellFormedXML(t, out)
}

func TestRenderEmptyGraphStillValid(t *testing.T) {
	out := RenderDocument("Nothing here", nil)
	if !strings.HasPrefix(out, "<svg") || !strings.HasSuffix(out, "</svg>") {
		t.Fatal("empty document is not a standalone SVG")
	}
	assertWellFormedXML(t, out)
}

func TestRenderDocumentStacksPanels(t *testing.T) {
	out := RenderDocument("Two diagrams", []Graph{
		{Title: "First", Nodes: []Node{{ID: "a", Label: "Alpha"}}},
		{Title: "Second", Nodes: []Node{{ID: "b", Label: "Beta"}}},
	})
	assertWellFormedXML(t, out)
	for _, want := range []string{"Alpha", "Beta", "First", "Second"} {
		if !strings.Contains(out, want) {
			t.Errorf("stacked document missing %q", want)
		}
	}
	// Panels are offset via translate groups.
	if strings.Count(out, "translate(0,") < 2 {
		t.Error("expected two translated panels")
	}
}
