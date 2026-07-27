package archspec

import (
	"context"
	"errors"
	"strings"
	"testing"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

type fakeModel struct {
	prompt string
	reply  string
	err    error
}

func (f *fakeModel) Generate(_ context.Context, p string) (string, error) {
	f.prompt = p
	if f.err != nil {
		return "", f.err
	}
	if f.reply != "" {
		return f.reply, nil
	}
	return specHeading + "\n\n## Diagram Metadata\n- Title: generated", nil
}

// richContext carries real AWS evidence (CloudFormation resources + services +
// architecture + a diagram) so the spec has something honest to ground on.
func richContext() *rc.ReleaseContext {
	return &rc.ReleaseContext{
		Repository: rc.Repository{Owner: "acme", Name: "widget", FullName: "acme/widget",
			Summary: "Turns repositories into content."},
		Release: rc.Release{Tag: "v0.3.0", Name: "v0.3.0"},
		Architecture: rc.Architecture{
			Overview:         "An event-driven content platform on AWS.",
			AWSServices:      []string{"AWS Lambda", "Amazon SQS", "Amazon S3"},
			Components:       []rc.ArchitectureComponent{{Name: "Worker", Responsibility: "process release events", Kind: "compute"}},
			EventDrivenFlows: []string{"EventBridge routes release events to an SQS queue consumed by an EC2 worker"},
			Security:         "Least-privilege IAM roles per stack.",
		},
		CloudFormation: rc.CloudFormationAnalysis{
			Templates: []string{"serverless.yaml", "compute.yaml"},
			Services:  []string{"AWS Lambda", "Amazon SQS", "Amazon S3"},
			Resources: []rc.CFNResource{
				{LogicalID: "EventsQueue", Type: "AWS::SQS::Queue", Service: "Amazon SQS", Category: "Messaging", Template: "serverless.yaml"},
				{LogicalID: "ContentBucket", Type: "AWS::S3::Bucket", Service: "Amazon S3", Category: "Storage", Template: "bootstrap.yaml"},
			},
			Counts: rc.CFNCounts{Resources: 2, Messaging: 1, Storage: 1, IAM: 2, Networking: 1},
		},
		Mermaid: []rc.MermaidDiagram{{
			Type: "flowchart", Summary: "release pipeline", Source: "docs/architecture.md",
			Edges: []rc.MermaidEdge{{From: "EventBridge", To: "SQS"}, {From: "SQS", To: "Worker", Label: "poll"}},
		}},
	}
}

// TestSpecSkipsWithoutEvidence: a repository with no CloudFormation and no
// detected AWS services has nothing groundable — the generator must refuse
// rather than invent a diagram.
func TestSpecSkipsWithoutEvidence(t *testing.T) {
	c := richContext()
	c.Architecture = rc.Architecture{}
	c.CloudFormation = rc.CloudFormationAnalysis{}

	if HasEvidence(c) {
		t.Fatal("HasEvidence should be false with no CFN and no AWS services")
	}
	_, err := (&Generator{Model: &fakeModel{}}).Spec(context.Background(), ReleasePackage{Context: c})
	if !errors.Is(err, ErrNoEvidence) {
		t.Fatalf("want ErrNoEvidence, got %v", err)
	}
}

// TestPromptIsGroundedAndClosesServiceVocabulary: the prompt must constrain the
// model to a closed set of evidence-backed services, expose the concrete
// CloudFormation evidence, focus on the article topic, and forbid every
// rendered/diagram-markup output form.
func TestPromptIsGroundedAndClosesServiceVocabulary(t *testing.T) {
	fm := &fakeModel{}
	_, err := (&Generator{Model: fm}).Spec(context.Background(), ReleasePackage{
		Context: richContext(),
		Blog:    releasegen.BlogPost{Title: "Designing an Event-Driven AI Platform on AWS"},
	})
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	p := fm.prompt

	// Closed vocabulary: only evidence-backed services may be named.
	if !strings.Contains(p, "ONLY these AWS services") {
		t.Error("prompt does not constrain the model to a closed service vocabulary")
	}
	for _, svc := range []string{"AWS Lambda", "Amazon SQS", "Amazon S3"} {
		if !strings.Contains(p, svc) {
			t.Errorf("prompt missing evidence-backed service %q", svc)
		}
	}

	// Concrete CloudFormation evidence is surfaced so claims can cite it.
	for _, ev := range []string{"EventsQueue", "AWS::SQS::Queue", "serverless.yaml", "ContentBucket"} {
		if !strings.Contains(p, ev) {
			t.Errorf("prompt missing repository evidence %q", ev)
		}
	}

	// The diagram is focused on the article's engineering topic.
	if !strings.Contains(p, "Designing an Event-Driven AI Platform on AWS") {
		t.Error("prompt does not anchor the diagram to the article topic")
	}

	// The output contract and its mandatory sections are present.
	for _, sec := range []string{
		specHeading, "## Diagram Metadata", "## Components", "## Connections",
		"## Security", "## Operational Flow", "## Failure Handling", "## Rendering Notes",
	} {
		if !strings.Contains(p, sec) {
			t.Errorf("prompt missing required output section %q", sec)
		}
	}

	// Rendering / markup output is forbidden — Claude designs, it does not render.
	rulesLine := ""
	for _, line := range strings.Split(p, "\n") {
		if strings.Contains(line, "Do NOT output") {
			rulesLine = line
			break
		}
	}
	if rulesLine == "" {
		t.Fatal("prompt no longer forbids rendered output forms")
	}
	for _, banned := range []string{"SVG", "XML", "Graphviz", "Mermaid"} {
		if !strings.Contains(rulesLine, banned) {
			t.Errorf("prompt no longer forbids %q output", banned)
		}
	}
}

// TestOfflineSpecIsDeterministicAndGrounded: with no model the generator still
// produces a complete, evidence-grounded specification (never a rendered form),
// and it is deterministic.
func TestOfflineSpecIsDeterministicAndGrounded(t *testing.T) {
	pkg := ReleasePackage{Context: richContext(), Blog: releasegen.BlogPost{Title: "Event-Driven Platform"}}

	spec, err := (&Generator{}).Spec(context.Background(), pkg) // nil Model → offline
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	body := spec.Markdown()

	if !strings.HasPrefix(body, specHeading) {
		t.Errorf("offline spec must start with %q", specHeading)
	}
	for _, sec := range []string{
		"## Diagram Metadata", "## Components", "## Connections",
		"## Security", "## Operational Flow", "## Failure Handling", "## Rendering Notes",
	} {
		if !strings.Contains(body, sec) {
			t.Errorf("offline spec missing section %q", sec)
		}
	}
	// Components and metadata are drawn from real evidence.
	for _, ev := range []string{"EventsQueue", "ContentBucket", "acme/widget", "v0.3.0", DiagramVersion} {
		if !strings.Contains(body, ev) {
			t.Errorf("offline spec missing grounded value %q", ev)
		}
	}
	// A connection is derived from the event-driven flow / diagram edges.
	if !strings.Contains(body, "EventBridge") {
		t.Error("offline spec did not derive any connection from the evidence")
	}
	// It is a specification, not a rendering: no code fences / diagram markup.
	if strings.Contains(body, "```") || strings.Contains(body, "<svg") {
		t.Error("offline spec must not contain rendered/markup output")
	}
	// Deterministic.
	spec2, _ := (&Generator{}).Spec(context.Background(), pkg)
	if spec2.Markdown() != body {
		t.Error("offline spec is not deterministic")
	}
}

// TestModelSpecNormalisesHeading: the artifact always begins with the canonical
// heading exactly once, whether or not the model emitted it.
func TestModelSpecNormalisesHeading(t *testing.T) {
	// Model omits the heading → it is prepended.
	fm := &fakeModel{reply: "## Diagram Metadata\n- Title: X"}
	spec, err := (&Generator{Model: fm}).Spec(context.Background(), ReleasePackage{Context: richContext()})
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	if !strings.HasPrefix(spec.Body, specHeading) {
		t.Error("heading not prepended when the model omitted it")
	}

	// Model includes the heading → it is not duplicated.
	fm2 := &fakeModel{reply: specHeading + "\n\n## Diagram Metadata\n- Title: X"}
	spec2, _ := (&Generator{Model: fm2}).Spec(context.Background(), ReleasePackage{Context: richContext()})
	if n := strings.Count(spec2.Body, specHeading); n != 1 {
		t.Errorf("heading appears %d times, want exactly 1", n)
	}
}

// TestSpecPropagatesModelError: a model failure surfaces as an error (so the
// orchestrator records the stage as failed) rather than a silent empty artifact.
func TestSpecPropagatesModelError(t *testing.T) {
	fm := &fakeModel{err: errors.New("boom")}
	if _, err := (&Generator{Model: fm}).Spec(context.Background(), ReleasePackage{Context: richContext()}); err == nil {
		t.Error("expected an error when the model fails")
	}
}
