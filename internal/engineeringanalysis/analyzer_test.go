package engineeringanalysis

import (
	"context"
	"errors"
	"strings"
	"testing"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

type fakeModel struct {
	out    string
	err    error
	prompt string
}

func (f *fakeModel) Generate(_ context.Context, prompt string) (string, error) {
	f.prompt = prompt
	return f.out, f.err
}

func sampleContext() *rc.ReleaseContext {
	return &rc.ReleaseContext{
		Repository: rc.Repository{FullName: "acme/widget", Summary: "A widget platform", Language: "Go"},
		Release:    rc.Release{Tag: "v0.3.0", Name: "Autumn", Body: "Adds SQS buffering."},
		Changelog:  rc.ChangelogAnalysis{Features: []string{"SQS buffering"}},
	}
}

const goodJSON = `{
  "problem": "Webhook spikes overwhelmed the worker.",
  "engineering_decisions": [{"decision": "Buffer via SQS", "rationale": "decouple ingest from compute"}],
  "tradeoffs": [{"choice": "SQS", "alternatives": ["direct invoke"], "advantages": ["absorbs spikes"], "disadvantages": ["at-least-once"]}],
  "aws_services": [{"name": "SQS", "purpose": "buffer events", "rationale": "managed, durable"}],
  "security": ["least-privilege IAM on the queue"],
  "scalability": {"current": "one worker drains the queue", "future": "N workers"},
  "cost_optimizations": ["on-demand instance stopped off-window"],
  "future_milestones": ["autoscaling workers"],
  "implementation_notes": ["visibility timeout 3600s"]
}`

func TestAnalyzeParsesAndStampsRelease(t *testing.T) {
	fm := &fakeModel{out: goodJSON}
	a := &Analyzer{Model: fm}
	ec, err := a.Analyze(context.Background(), sampleContext())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// Release identity comes from the authoritative context, not the model.
	if ec.Release.Version != "v0.3.0" || ec.Release.Name != "Autumn" {
		t.Errorf("release = %+v", ec.Release)
	}
	if ec.Problem == "" || len(ec.EngineeringDecisions) != 1 || len(ec.Tradeoffs) != 1 {
		t.Errorf("shallow parse: %+v", ec)
	}
	if ec.AWSServices[0].Name != "SQS" || ec.Scalability.Future != "N workers" {
		t.Errorf("fields = %+v / %+v", ec.AWSServices, ec.Scalability)
	}
	if ec.IsZero() {
		t.Error("IsZero() should be false for a populated analysis")
	}
	// The prompt must forbid prose and demand JSON only.
	if !strings.Contains(fm.prompt, "JSON object ONLY") {
		t.Error("prompt should instruct JSON-only output")
	}
	if !strings.Contains(fm.prompt, "acme/widget") {
		t.Error("prompt should ground in the release facts")
	}
}

func TestAnalyzeToleratesFencedAndProseWrappedJSON(t *testing.T) {
	wrapped := "Sure, here is the analysis:\n```json\n" + goodJSON + "\n```\nHope that helps!"
	a := &Analyzer{Model: &fakeModel{out: wrapped}}
	ec, err := a.Analyze(context.Background(), sampleContext())
	if err != nil {
		t.Fatalf("Analyze wrapped: %v", err)
	}
	if ec.Problem == "" {
		t.Error("failed to extract JSON from fenced/prose-wrapped output")
	}
}

func TestAnalyzeModelError(t *testing.T) {
	a := &Analyzer{Model: &fakeModel{err: errors.New("boom")}}
	if _, err := a.Analyze(context.Background(), sampleContext()); err == nil {
		t.Fatal("expected error on model failure")
	}
}

func TestAnalyzeNoJSON(t *testing.T) {
	a := &Analyzer{Model: &fakeModel{out: "I could not analyze this."}}
	if _, err := a.Analyze(context.Background(), sampleContext()); err == nil {
		t.Fatal("expected error when no JSON is present")
	}
}

func TestAnalyzeNilModel(t *testing.T) {
	a := &Analyzer{}
	if _, err := a.Analyze(context.Background(), sampleContext()); err == nil {
		t.Fatal("expected error with no model configured")
	}
}

func TestExtractJSONObjectBalancesBraces(t *testing.T) {
	in := `prefix {"a": {"b": "}"}, "c": 1} suffix`
	got := extractJSONObject(in)
	want := `{"a": {"b": "}"}, "c": 1}`
	if got != want {
		t.Errorf("extractJSONObject = %q, want %q", got, want)
	}
}

func TestGroundingBlockRoundTrips(t *testing.T) {
	a := &Analyzer{Model: &fakeModel{out: goodJSON}}
	ec, err := a.Analyze(context.Background(), sampleContext())
	if err != nil {
		t.Fatal(err)
	}
	block := ec.GroundingBlock()
	for _, want := range []string{"Problem solved", "Buffer via SQS", "Chose SQS", "SQS: buffer events"} {
		if !strings.Contains(block, want) {
			t.Errorf("grounding block missing %q\n%s", want, block)
		}
	}
	// A zero analysis renders nothing (so grounding stays clean).
	if (&rc.EngineeringContext{}).GroundingBlock() != "" {
		t.Error("zero EngineeringContext should render an empty grounding block")
	}
}

func TestAnalyzeUsesFallbackWhenPrimaryEmpty(t *testing.T) {
	primary := &fakeModel{out: `{}`}      // parses, but empty
	fallback := &fakeModel{out: goodJSON} // populated
	a := &Analyzer{Model: primary, Fallback: fallback}
	ec, err := a.Analyze(context.Background(), sampleContext())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if ec.IsZero() {
		t.Error("expected the fallback's populated analysis")
	}
	if fallback.prompt == "" {
		t.Error("fallback model was not invoked")
	}
	if ec.Release.Version != "v0.3.0" {
		t.Errorf("release not stamped: %+v", ec.Release)
	}
}

func TestAnalyzeSkipsFallbackWhenPrimaryPopulated(t *testing.T) {
	primary := &fakeModel{out: goodJSON}
	fallback := &fakeModel{out: goodJSON}
	a := &Analyzer{Model: primary, Fallback: fallback}
	if _, err := a.Analyze(context.Background(), sampleContext()); err != nil {
		t.Fatal(err)
	}
	if fallback.prompt != "" {
		t.Error("fallback must not run when the primary analysis is non-empty")
	}
}
