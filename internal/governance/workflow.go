package governance

import (
	"context"
	"time"
)

// Engine orchestrates the review-and-approval workflow over the domain, engines,
// and ports. It is the application layer: it advances the state machine,
// enforces gates and revision limits, and writes the audit trail. It depends on
// interfaces (Repository, AIReviewer, GitHubGateway), never on concrete adapters.
type Engine struct {
	Config       Config
	Repo         Repository
	ReviewEngine ReviewEngine
	Approval     ApprovalEngine
	Readiness    ReadinessEngine
	Revisions    RevisionManager
	GitHub       GitHubGateway
	Now          Clock
}

// NewEngine wires an Engine with the given config, repository, and optional AI
// reviewer. A nil repository gets an in-memory one; a nil clock uses time.Now.
func NewEngine(cfg Config, repo Repository, ai AIReviewer, now Clock) *Engine {
	if now == nil {
		now = time.Now
	}
	if repo == nil {
		repo = NewMemoryRepository()
	}
	return &Engine{
		Config:       cfg,
		Repo:         repo,
		ReviewEngine: NewReviewEngine(cfg, ai, now),
		Approval:     ApprovalEngine{Config: cfg},
		Readiness:    ReadinessEngine{Config: cfg},
		Revisions:    RevisionManager{Now: now},
		GitHub:       noopGitHub{},
		Now:          now,
	}
}

// Submit registers new content and moves it Draft → Generated.
func (e *Engine) Submit(c Content) (*WorkflowState, error) {
	if collapse(c.Body) == "" {
		return nil, ErrEmptyContent
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = e.Now()
	}
	state := &WorkflowState{
		Content:   c,
		Status:    StatusDraft,
		CreatedAt: e.Now(),
		UpdatedAt: e.Now(),
	}
	e.audit(state, c.Author, "submit", StatusDraft, StatusDraft, "content submitted")
	if err := e.transition(context.Background(), state, StatusGenerated, c.Author, "generate", "content generated"); err != nil {
		return nil, err
	}
	return state, e.Repo.Save(state)
}

// Review runs validation, grounding, AI review, and scoring, then advances the
// state to Pending Approval, Needs Revision, or Rejected based on the decision.
func (e *Engine) Review(ctx context.Context, id string) (*WorkflowState, error) {
	state, err := e.Repo.Get(id)
	if err != nil {
		return nil, err
	}
	if state.Status != StatusGenerated && state.Status != StatusUpdated {
		return nil, ErrInvalidTransition
	}

	report := e.ReviewEngine.Review(ctx, state.Content)
	state.Reviews = append(state.Reviews, report)
	state.LatestReport = &state.Reviews[len(state.Reviews)-1]

	if err := e.transition(ctx, state, StatusAIReviewed, "ai-reviewer", "ai-review",
		"decision "+string(report.Decision)+", overall "+itoa(report.Scores.Overall)); err != nil {
		return nil, err
	}

	switch report.Decision {
	case DecisionApprove:
		err = e.transition(ctx, state, StatusPendingApproval, "ai-reviewer", "await-approval", "passed automated review")
	case DecisionReject:
		err = e.transition(ctx, state, StatusRejected, "ai-reviewer", "reject", "failed automated review")
	default:
		err = e.transition(ctx, state, StatusNeedsRevision, "ai-reviewer", "request-revision", "quality below threshold")
	}
	if err != nil {
		return nil, err
	}
	return state, e.Repo.Save(state)
}

// Revise records a new body (with the report's feedback + reviewer comments) and
// moves Needs Revision/Rejected → Updated, ready for re-review. Enforces the
// revision limit.
func (e *Engine) Revise(id, newBody string, reviewerComments []string, actor string) (*WorkflowState, error) {
	state, err := e.Repo.Get(id)
	if err != nil {
		return nil, err
	}
	if state.Status != StatusNeedsRevision && state.Status != StatusRejected {
		return nil, ErrInvalidTransition
	}
	if collapse(newBody) == "" {
		return nil, ErrEmptyContent
	}
	if e.Config.RevisionLimit > 0 && len(state.Revisions) >= e.Config.RevisionLimit {
		return nil, ErrRevisionLimit
	}

	rev := e.Revisions.Record(len(state.Revisions)+1, state.Content.Body, newBody, feedbackFrom(state.LatestReport), reviewerComments)
	state.Revisions = append(state.Revisions, rev)
	state.Content.Body = newBody

	// Rejected must first move back into the revision lane.
	if state.Status == StatusRejected {
		if err := e.transition(context.Background(), state, StatusNeedsRevision, actor, "reopen", "reopened for revision"); err != nil {
			return nil, err
		}
	}
	if err := e.transition(context.Background(), state, StatusUpdated, actor, "revise",
		"revision #"+itoa(rev.Number)+" recorded"); err != nil {
		return nil, err
	}
	return state, e.Repo.Save(state)
}

// Approve records a reviewer's approval decision. An approving decision (when the
// quality gates pass and required approvals are met) moves Pending Approval →
// Approved; a rejecting decision moves it → Needs Revision.
func (e *Engine) Approve(ctx context.Context, id string, reviewer Reviewer, approved bool, comment string) (*WorkflowState, error) {
	state, err := e.Repo.Get(id)
	if err != nil {
		return nil, err
	}
	if state.Status != StatusPendingApproval {
		return nil, ErrNotPendingApproval
	}
	if !e.Approval.canApprove(reviewer) {
		return nil, ErrPermissionDenied
	}
	if e.alreadyDecided(state, reviewer.Name) {
		return nil, ErrDuplicateApproval
	}

	// Quality gates must pass before any approval is accepted.
	if approved {
		gates := e.Approval.QualityGates(state)
		if !gatesPassed(gates) {
			return nil, ErrQualityGateFailed
		}
	}

	state.Approvals = append(state.Approvals, ApprovalDecision{
		Reviewer: reviewer, Approved: approved, Comment: comment, DecidedAt: e.Now(),
	})

	if !approved {
		if err := e.transition(ctx, state, StatusNeedsRevision, reviewer.Name, "approval-rejected", comment); err != nil {
			return nil, err
		}
		return state, e.Repo.Save(state)
	}

	// Approved: only advance once the required approvals/roles are satisfied.
	if e.Readiness.approvalsMet(state) {
		if err := e.transition(ctx, state, StatusApproved, reviewer.Name, "approve", comment); err != nil {
			return nil, err
		}
	} else {
		e.audit(state, reviewer.Name, "approve-partial", state.Status, state.Status, "approval recorded; awaiting more approvals")
	}
	return state, e.Repo.Save(state)
}

// Publish verifies publication readiness and moves Approved → Published.
func (e *Engine) Publish(ctx context.Context, id, publisher string) (*WorkflowState, error) {
	state, err := e.Repo.Get(id)
	if err != nil {
		return nil, err
	}
	if state.Status != StatusApproved {
		return nil, ErrNotApproved
	}
	readiness := e.Readiness.Check(state)
	state.Readiness = &readiness
	if !readiness.Ready {
		_ = e.Repo.Save(state)
		return state, ErrNotReadyToPublish
	}
	if err := e.transition(ctx, state, StatusPublished, publisher, "publish", "publication-readiness passed"); err != nil {
		return nil, err
	}
	return state, e.Repo.Save(state)
}

// Archive moves published/rejected/needs-revision content to Archived.
func (e *Engine) Archive(id, actor string) (*WorkflowState, error) {
	state, err := e.Repo.Get(id)
	if err != nil {
		return nil, err
	}
	if !canTransition(state.Status, StatusArchived) {
		return nil, ErrInvalidTransition
	}
	if err := e.transition(context.Background(), state, StatusArchived, actor, "archive", ""); err != nil {
		return nil, err
	}
	return state, e.Repo.Save(state)
}

// Get returns the current workflow state.
func (e *Engine) Get(id string) (*WorkflowState, error) { return e.Repo.Get(id) }

// List returns all workflow states.
func (e *Engine) List() ([]*WorkflowState, error) { return e.Repo.List() }

// --- internals ---

// transition applies a status change if it is legal, records an audit entry, and
// notifies the (optional) GitHub gateway.
func (e *Engine) transition(ctx context.Context, state *WorkflowState, to WorkflowStatus, actor, action, detail string) error {
	from := state.Status
	if !canTransition(from, to) {
		return ErrInvalidTransition
	}
	state.Status = to
	state.UpdatedAt = e.Now()
	e.audit(state, actor, action, from, to, detail)
	if e.GitHub != nil {
		_ = e.GitHub.OnStatusChange(ctx, state, from, to)
	}
	return nil
}

func (e *Engine) audit(state *WorkflowState, actor, action string, from, to WorkflowStatus, detail string) {
	if actor == "" {
		actor = "system"
	}
	state.Audit = append(state.Audit, AuditEntry{
		Timestamp: e.Now(), Actor: actor, Action: action,
		FromStatus: from, ToStatus: to, Detail: detail,
	})
}

// alreadyDecided reports whether a reviewer has decided in the current approval
// cycle (since the most recent revision, or content creation if none).
func (e *Engine) alreadyDecided(state *WorkflowState, name string) bool {
	boundary := state.Content.CreatedAt
	if n := len(state.Revisions); n > 0 {
		boundary = state.Revisions[n-1].CreatedAt
	}
	for _, a := range state.Approvals {
		if a.Reviewer.Name == name && !a.DecidedAt.Before(boundary) {
			return true
		}
	}
	return false
}
