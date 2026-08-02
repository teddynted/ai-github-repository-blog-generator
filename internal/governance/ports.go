package governance

import (
	"context"
	"time"
)

// AIReviewer is the port for AI-assisted qualitative review. It is abstracted so
// Amazon Bedrock or Anthropic (Claude), or any future provider can be plugged in
// without changing the workflow. Implementations must ground their review in the
// provided grounding and must never fabricate facts.
type AIReviewer interface {
	// Review returns qualitative notes for the content. It must not compute the
	// authoritative scores or decision — those are deterministic (see scoring.go)
	// — it only enriches the report with a summary, strengths, weaknesses, and
	// suggestions, plus a self-reported confidence.
	Review(ctx context.Context, c Content) (AINotes, error)
}

// AINotes are the qualitative outputs of an AI reviewer.
type AINotes struct {
	Reviewer    string   `json:"reviewer"`
	Summary     string   `json:"summary"`
	Strengths   []string `json:"strengths,omitempty"`
	Weaknesses  []string `json:"weaknesses,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	Confidence  int      `json:"confidence"` // 0–100
}

// Repository is the port for persisting workflow state. In-memory today; SQLite
// or PostgreSQL adapters can implement the same interface.
type Repository interface {
	Save(state *WorkflowState) error
	Get(id string) (*WorkflowState, error)
	List() ([]*WorkflowState, error)
}

// Clock is the port for time, so the workflow is deterministic in tests.
type Clock func() time.Time

// GitHubGateway is an optional port for mirroring the review lifecycle onto
// GitHub (issues, PRs, labels, review comments). A no-op adapter satisfies it
// when GitHub integration is disabled, so the core never depends on GitHub.
type GitHubGateway interface {
	OnStatusChange(ctx context.Context, state *WorkflowState, from, to WorkflowStatus) error
}

// noopGitHub is the default gateway — governance works with GitHub disabled.
type noopGitHub struct{}

func (noopGitHub) OnStatusChange(context.Context, *WorkflowState, WorkflowStatus, WorkflowStatus) error {
	return nil
}
