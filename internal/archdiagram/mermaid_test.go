package archdiagram

import (
	"strings"
	"testing"
)

func TestGraphValidateDetectsDanglingEdge(t *testing.T) {
	g := NewGraph("LR")
	g.AddNode("a", "A")
	g.AddEdge("a", "b", "") // b is never declared
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for edge to undeclared node")
	}
}

func TestGraphValidateDetectsOrphan(t *testing.T) {
	g := NewGraph("LR")
	g.AddNode("a", "A")
	g.AddNode("b", "B")
	g.AddNode("orphan", "Orphan")
	g.AddEdge("a", "b", "")
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for orphan node")
	}
}

func TestGraphValidatePasses(t *testing.T) {
	g := NewGraph("LR")
	g.AddNode("a", "A")
	g.AddNode("b", "B")
	g.AddEdge("a", "b", "uses")
	if err := g.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestMermaidRenderIsWellFormed(t *testing.T) {
	g := NewGraph("LR")
	g.AddNode("client", "Client")
	g.AddNode("s3", "Amazon S3")
	g.AddEdge("client", "s3", "reads / writes")
	g.AddCluster("aws", "AWS Cloud", "s3")

	out := g.Mermaid()
	for _, want := range []string{
		"flowchart LR",
		`subgraph aws["AWS Cloud"]`,
		`s3["Amazon S3"]`,
		`client -->|reads / writes| s3`,
		"end",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered Mermaid missing %q\n---\n%s", want, out)
		}
	}
	// A node inside a cluster must be declared exactly once.
	if n := strings.Count(out, `s3["Amazon S3"]`); n != 1 {
		t.Errorf("s3 node declared %d times, want 1", n)
	}
}

func TestSanitizeID(t *testing.T) {
	cases := map[string]string{
		"Amazon S3": "Amazon_S3",
		"9lives":    "n9lives",
		"a.b/c":     "a_b_c",
		"":          "n",
	}
	for in, want := range cases {
		if got := sanitizeID(in); got != want {
			t.Errorf("sanitizeID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapeQuotes(t *testing.T) {
	if got := escape(`say "hi"` + "\nbye"); got != "say 'hi' bye" {
		t.Errorf("escape mismatch: %q", got)
	}
}
