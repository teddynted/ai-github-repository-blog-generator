package terminology

import (
	"strings"
	"testing"
)

func TestViolationsDetectsCorruptions(t *testing.T) {
	r := Base()
	cases := []struct {
		name    string
		text    string
		wantCan string // canonical expected in a violation ("" = expect none)
		wantHad string // offending text expected ("" = don't care)
	}{
		{"claw in prose", "Then OpenClaw dispatches, and claw runs the agent.", "OpenClaw", "claw"},
		{"claw hallucinated to Claude", "The Claude orchestrator fails over to Bedrock.", "OpenClaw", "Claude"},
		{"n8n uppercased", "Work lands on N8N for the human step.", "n8n", "N8N"},
		{"EventBridge split", "The Event Bridge rule invokes Lambda.", "EventBridge", "Event Bridge"},
		{"CloudWatch split", "Everything is logged to Cloud Watch.", "CloudWatch", "Cloud Watch"},
		{"Ollama mis-cased", "The ollama backend answers first.", "Ollama", "ollama"},
		{"clean prose", "OpenClaw runs Ollama first, then Amazon Bedrock; EventBridge invokes Lambda.", "", ""},
		{"short forms allowed", "Bedrock is the fallback; S3 stores the output; Lambda routes.", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vs := r.Violations(c.text)
			if c.wantCan == "" {
				if len(vs) != 0 {
					t.Fatalf("want no violations, got %+v", vs)
				}
				return
			}
			found := false
			for _, v := range vs {
				if v.Canonical == c.wantCan && (c.wantHad == "" || v.Found == c.wantHad) {
					found = true
				}
			}
			if !found {
				t.Errorf("want violation of %q (%q), got %+v", c.wantCan, c.wantHad, vs)
			}
		})
	}
}

func TestViolationsIgnoresCodeBlocks(t *testing.T) {
	// The diagram node-id "claw" (and eb/lam) is legitimate inside a fenced Mermaid
	// block and inline code — it must not be flagged.
	text := "The orchestrator is OpenClaw.\n\n```mermaid\nflowchart TD\n  eb --> lam --> claw\n  claw --> n8n\n```\n\nInline `claw` node too."
	if vs := Base().Violations(text); len(vs) != 0 {
		t.Fatalf("code-block node-ids should not be flagged, got %+v", vs)
	}
}

func TestDeriveAddsReleaseTerms(t *testing.T) {
	reg := Derive([]string{"Amazon DynamoDB", "AWS Step Functions"}, []string{"DynamoDB"}, []string{"claw", "eb"})
	terms := strings.Join(reg.Terms(), "|")
	if !strings.Contains(terms, "Amazon DynamoDB") || !strings.Contains(terms, "AWS Step Functions") {
		t.Errorf("derived registry missing release services: %s", terms)
	}
	// A derived camelCase term is split-checked automatically: "DynamoDB" -> "Dynamo DB".
	if vs := reg.Violations("We used Dynamo DB for state."); len(vs) == 0 {
		t.Error("expected a split-compound violation for 'Dynamo DB'")
	}
}

func TestPromptClauseListsTerms(t *testing.T) {
	c := Base().PromptClause()
	for _, want := range []string{"OpenClaw", "EventBridge", "n8n", "PROTECTED TERMS"} {
		if !strings.Contains(c, want) {
			t.Errorf("prompt clause missing %q:\n%s", want, c)
		}
	}
}
