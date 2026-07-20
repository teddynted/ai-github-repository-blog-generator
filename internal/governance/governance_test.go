package governance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// --- fixtures ---

func goodBlog() Content {
	return Content{
		ID:    "blog-1",
		Type:  TypeBlog,
		Title: "Inside widget v0.2.0",
		Body: "# Inside widget v0.2.0\n\n" +
			"This release adds a Release Context builder. It is built with AWS Lambda and Amazon SQS, so that ingestion is decoupled from compute. " +
			"The builder is a pure Go package, which keeps it testable because the logic sits behind a small interface. " +
			"Read the full write-up on the blog for the details.",
		Metadata:  map[string]string{"cta": "Read the blog", "thumbnail": "widget-thumb.png"},
		Grounding: Grounding{Repository: "acme/widget", Release: "v0.2.0", Terms: []string{"release context builder", "AWS Lambda", "Amazon SQS", "Go", "interface"}},
		Author:    "author@example.com",
	}
}

func fixedClock() Clock {
	t := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	return func() time.Time { t = t.Add(time.Second); return t }
}

func newEngine(cfg Config) *Engine {
	return NewEngine(cfg, NewMemoryRepository(), nil, fixedClock())
}

// --- validation ---

func TestValidationCatchesIssues(t *testing.T) {
	v := ValidationEngine{}
	// Placeholder + unbalanced fence + empty link.
	bad := Content{Type: TypeBlog, Title: "x", Body: "# H\n\nTODO finish this. See [here](). ```go\nunclosed"}
	rep := v.Validate(bad)
	codes := issueCodes(rep)
	for _, want := range []string{"placeholder", "empty-link", "code-fence"} {
		if !codes[want] {
			t.Errorf("expected validation code %q in %v", want, keys(codes))
		}
	}
	if rep.Passed {
		t.Error("report should not pass with errors")
	}
	// A clean blog passes (no errors).
	if r := v.Validate(goodBlog()); !r.Passed {
		t.Errorf("good blog should pass validation, got issues %+v", r.Issues)
	}
}

func TestValidationMermaid(t *testing.T) {
	v := ValidationEngine{}
	good := Content{Type: TypeArchitectureDiagram, Body: "flowchart TD\n  A --> B"}
	if r := v.Validate(good); !r.Passed {
		t.Errorf("valid mermaid should pass: %+v", r.Issues)
	}
	bad := Content{Type: TypeArchitectureDiagram, Body: "notadiagram\n  A --> B"}
	if r := v.Validate(bad); r.Passed {
		t.Error("invalid mermaid should fail")
	}
}

func TestValidationSEOLimit(t *testing.T) {
	v := ValidationEngine{}
	c := Content{Type: TypeSEO, Body: "seo metadata content here for the release notes and blog post distribution", Metadata: map[string]string{"metaDescription": strings.Repeat("x", 200), "slug": "ok"}}
	if issueCodes(v.Validate(c))["meta-desc-limit"] == false {
		t.Error("expected meta-desc-limit error")
	}
}

// --- grounding ---

func TestGroundingVerifiesClaims(t *testing.T) {
	g := GroundingEngine{}
	if r := g.Verify(goodBlog()); !r.Verified {
		t.Errorf("good blog should be grounded: unverified=%v", r.UnverifiedClaims)
	}
	// An ungrounded AWS claim is flagged.
	c := goodBlog()
	c.Body += " It also uses Amazon Redshift for analytics."
	r := g.Verify(c)
	if r.Verified {
		t.Error("expected ungrounded Redshift claim to fail verification")
	}
	if len(r.UnverifiedClaims) == 0 {
		t.Error("expected an unverified claim to be listed")
	}
}

func TestGroundingFailsClosedWithoutTerms(t *testing.T) {
	g := GroundingEngine{}
	c := goodBlog()
	c.Grounding = Grounding{} // no terms at all
	if g.Verify(c).Verified {
		t.Error("content with no grounding terms must fail closed (never approve hallucinations)")
	}
}

// --- scoring & decision ---

func TestScoringApprovesGoodContent(t *testing.T) {
	cfg := DefaultConfig()
	e := ScoringEngine{Config: cfg}
	c := goodBlog()
	v := ValidationEngine{}.Validate(c)
	g := GroundingEngine{}.Verify(c)
	s, d := e.Score(c, v, g)
	if s.Overall < cfg.MinOverallScore {
		t.Errorf("good content overall %d below min %d", s.Overall, cfg.MinOverallScore)
	}
	if d != DecisionApprove {
		t.Errorf("decision = %q, want Approve (scores %+v)", d, s)
	}
}

func TestScoringNeedsRevisionOnUngrounded(t *testing.T) {
	cfg := DefaultConfig()
	e := ScoringEngine{Config: cfg}
	c := goodBlog()
	c.Body += " It also uses Amazon Redshift and Amazon SageMaker."
	v := ValidationEngine{}.Validate(c)
	g := GroundingEngine{}.Verify(c)
	_, d := e.Score(c, v, g)
	if d == DecisionApprove {
		t.Error("ungrounded content must never be approved")
	}
}

func TestScoringRejectsGarbage(t *testing.T) {
	cfg := DefaultConfig()
	e := ScoringEngine{Config: cfg}
	c := Content{Type: TypeBlog, Title: "x", Body: "aws lambda thing. aws lambda thing. aws lambda thing.", Grounding: Grounding{Terms: []string{"aws lambda"}}}
	v := ValidationEngine{}.Validate(c)
	g := GroundingEngine{}.Verify(c)
	s, d := e.Score(c, v, g)
	if d == DecisionApprove {
		t.Errorf("low-quality content should not be approved (overall %d)", s.Overall)
	}
}

// --- review engine ---

func TestReviewEngineProducesReport(t *testing.T) {
	e := NewReviewEngine(DefaultConfig(), nil, fixedClock())
	r := e.Review(context.Background(), goodBlog())
	if r.Reviewer != "deterministic" || r.Scores.Overall == 0 || r.Summary == "" {
		t.Errorf("report incomplete: %+v", r)
	}
	if len(r.Strengths) == 0 {
		t.Error("expected strengths for a good blog")
	}
}

type fakeAI struct{ calls int }

func (f *fakeAI) Review(_ context.Context, _ Content) (AINotes, error) {
	f.calls++
	return AINotes{Reviewer: "fake-model", Summary: "AI summary.", Strengths: []string{"Clear."}, Suggestions: []string{"Tighten intro."}, Confidence: 90}, nil
}

func TestReviewEngineMergesAINotes(t *testing.T) {
	ai := &fakeAI{}
	e := NewReviewEngine(DefaultConfig(), ai, fixedClock())
	r := e.Review(context.Background(), goodBlog())
	if ai.calls != 1 {
		t.Fatalf("AI called %d times, want 1", ai.calls)
	}
	if r.Summary != "AI summary." || !strings.Contains(r.Reviewer, "fake-model") {
		t.Errorf("AI notes not merged: %+v", r)
	}
	// The AI never changes the deterministic decision.
	det := NewReviewEngine(DefaultConfig(), nil, fixedClock()).Review(context.Background(), goodBlog())
	if r.Decision != det.Decision {
		t.Errorf("AI changed the decision: %q vs %q", r.Decision, det.Decision)
	}
}

type errAI struct{}

func (errAI) Review(context.Context, Content) (AINotes, error) { return AINotes{}, errors.New("boom") }

func TestReviewEngineFallsBackOnAIError(t *testing.T) {
	e := NewReviewEngine(DefaultConfig(), errAI{}, fixedClock())
	r := e.Review(context.Background(), goodBlog())
	if r.Reviewer != "deterministic" || r.Summary == "" {
		t.Errorf("expected deterministic fallback on AI error: %+v", r)
	}
}

func TestModelReviewerParsing(t *testing.T) {
	r := NewModelReviewer(fakeModel(`Sure! {"summary":"ok","strengths":["a"],"weaknesses":[],"suggestions":["b"],"confidence":77} done`), "claude")
	notes, err := r.Review(context.Background(), goodBlog())
	if err != nil {
		t.Fatal(err)
	}
	if notes.Summary != "ok" || notes.Confidence != 77 || notes.Reviewer != "claude" {
		t.Errorf("parsed notes = %+v", notes)
	}
	// Unparseable output errors (so the engine falls back).
	if _, err := NewModelReviewer(fakeModel("no json here"), "claude").Review(context.Background(), goodBlog()); err == nil {
		t.Error("expected parse error on non-JSON output")
	}
}

// --- workflow / state machine ---

func TestFullApprovalWorkflow(t *testing.T) {
	e := newEngine(DefaultConfig())
	state, err := e.Submit(goodBlog())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if state.Status != StatusGenerated {
		t.Fatalf("status after submit = %q", state.Status)
	}

	state, err = e.Review(context.Background(), "blog-1")
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if state.Status != StatusPendingApproval {
		t.Fatalf("good content should reach Pending Approval, got %q (decision %q)", state.Status, state.LatestReport.Decision)
	}

	// Approve with a permitted role.
	state, err = e.Approve(context.Background(), "blog-1", Reviewer{Name: "rev@x", Role: RoleReviewer}, true, "LGTM")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if state.Status != StatusApproved {
		t.Fatalf("status after approval = %q", state.Status)
	}

	// Publish.
	state, err = e.Publish(context.Background(), "blog-1", "pub@x")
	if err != nil {
		t.Fatalf("publish: %v (readiness %+v)", err, state.Readiness)
	}
	if state.Status != StatusPublished {
		t.Fatalf("status after publish = %q", state.Status)
	}

	// Audit trail records the whole journey.
	actions := auditActions(state)
	for _, want := range []string{"submit", "generate", "ai-review", "approve", "publish"} {
		if !actions[want] {
			t.Errorf("audit missing action %q", want)
		}
	}
}

func TestPublishBlockedBeforeApproval(t *testing.T) {
	e := newEngine(DefaultConfig())
	_, _ = e.Submit(goodBlog())
	if _, err := e.Publish(context.Background(), "blog-1", "pub@x"); !errors.Is(err, ErrNotApproved) {
		t.Errorf("publish before approval should fail with ErrNotApproved, got %v", err)
	}
}

func TestApprovePermissionDenied(t *testing.T) {
	e := newEngine(DefaultConfig())
	_, _ = e.Submit(goodBlog())
	_, _ = e.Review(context.Background(), "blog-1")
	// Author role may not approve.
	if _, err := e.Approve(context.Background(), "blog-1", Reviewer{Name: "a", Role: RoleAuthor}, true, ""); !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("author approval should be denied, got %v", err)
	}
}

func TestDuplicateApprovalRejected(t *testing.T) {
	e := newEngine(DefaultConfig())
	_, _ = e.Submit(goodBlog())
	_, _ = e.Review(context.Background(), "blog-1")
	rev := Reviewer{Name: "rev@x", Role: RoleReviewer}
	if _, err := e.Approve(context.Background(), "blog-1", rev, false, "changes"); err != nil {
		// first decision (reject) moves to NeedsRevision; that's fine
	}
	// It's now NeedsRevision, not PendingApproval → a second approve is invalid.
	if _, err := e.Approve(context.Background(), "blog-1", rev, true, ""); !errors.Is(err, ErrNotPendingApproval) {
		t.Errorf("approving non-pending content should fail, got %v", err)
	}
}

func TestRevisionLoop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RevisionLimit = 2
	e := newEngine(cfg)
	// Ungrounded content → Needs Revision.
	c := goodBlog()
	c.Body += " It also uses Amazon Redshift and Amazon SageMaker for analytics."
	_, _ = e.Submit(c)
	state, _ := e.Review(context.Background(), "blog-1")
	if state.Status != StatusNeedsRevision {
		t.Fatalf("ungrounded content should need revision, got %q", state.Status)
	}

	// Revise with grounded content → Updated → re-review → Pending Approval.
	state, err := e.Revise("blog-1", goodBlog().Body, []string{"removed the Redshift claim"}, "author@x")
	if err != nil {
		t.Fatalf("revise: %v", err)
	}
	if state.Status != StatusUpdated || len(state.Revisions) != 1 {
		t.Fatalf("after revise: status %q, revisions %d", state.Status, len(state.Revisions))
	}
	if len(state.Revisions[0].Feedback) == 0 {
		t.Error("revision should preserve feedback")
	}
	state, _ = e.Review(context.Background(), "blog-1")
	if state.Status != StatusPendingApproval {
		t.Fatalf("re-review of grounded content should reach Pending Approval, got %q", state.Status)
	}
}

func TestRevisionLimitEnforced(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RevisionLimit = 1
	e := newEngine(cfg)
	c := goodBlog()
	c.Body += " It also uses Amazon Redshift for analytics."
	_, _ = e.Submit(c)
	_, _ = e.Review(context.Background(), "blog-1")
	// First revision is allowed but stays ungrounded → back to NeedsRevision.
	_, _ = e.Revise("blog-1", c.Body, nil, "author@x")
	_, _ = e.Review(context.Background(), "blog-1")
	// Second revision exceeds the limit.
	if _, err := e.Revise("blog-1", c.Body, nil, "author@x"); !errors.Is(err, ErrRevisionLimit) {
		t.Errorf("expected ErrRevisionLimit, got %v", err)
	}
}

func TestRequiredRolesGate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RequiredApprovals = 2
	cfg.RequiredRoles = []Role{RoleReviewer, RoleTechnicalReviewer}
	e := newEngine(cfg)
	_, _ = e.Submit(goodBlog())
	_, _ = e.Review(context.Background(), "blog-1")

	// One approval is not enough.
	state, _ := e.Approve(context.Background(), "blog-1", Reviewer{Name: "r1", Role: RoleReviewer}, true, "")
	if state.Status == StatusApproved {
		t.Fatal("should not be approved with only one of two required approvals")
	}
	// Second required role approves → Approved.
	state, err := e.Approve(context.Background(), "blog-1", Reviewer{Name: "r2", Role: RoleTechnicalReviewer}, true, "")
	if err != nil {
		t.Fatalf("second approve: %v", err)
	}
	if state.Status != StatusApproved {
		t.Fatalf("should be approved after both required roles, got %q", state.Status)
	}
}

func TestQualityGatesBlockApproval(t *testing.T) {
	// Force a low overall min so the AI review approves, but then tamper the
	// report so a gate fails at approval time.
	e := newEngine(DefaultConfig())
	_, _ = e.Submit(goodBlog())
	state, _ := e.Review(context.Background(), "blog-1")
	if state.Status != StatusPendingApproval {
		t.Skip("precondition: content must be pending approval")
	}
	// Tamper: drop the technical accuracy below the floor.
	state.LatestReport.Scores.TechnicalAccuracy = 10
	_ = e.Repo.Save(state)
	if _, err := e.Approve(context.Background(), "blog-1", Reviewer{Name: "r", Role: RoleReviewer}, true, ""); !errors.Is(err, ErrQualityGateFailed) {
		t.Errorf("expected ErrQualityGateFailed when a gate fails, got %v", err)
	}
}

func TestInvalidTransitionRejected(t *testing.T) {
	e := newEngine(DefaultConfig())
	_, _ = e.Submit(goodBlog())
	// Cannot review twice from Generated → after first review it's not Generated.
	_, _ = e.Review(context.Background(), "blog-1")
	if _, err := e.Review(context.Background(), "blog-1"); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("re-review from a non-reviewable state should fail, got %v", err)
	}
}

func TestEmptyContentRejected(t *testing.T) {
	e := newEngine(DefaultConfig())
	if _, err := e.Submit(Content{ID: "x", Type: TypeBlog, Body: "   "}); !errors.Is(err, ErrEmptyContent) {
		t.Errorf("empty content should be rejected, got %v", err)
	}
}

func TestNotFound(t *testing.T) {
	e := newEngine(DefaultConfig())
	if _, err := e.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPublishBlockedWhenNotReady(t *testing.T) {
	e := newEngine(DefaultConfig())
	c := goodBlog()
	delete(c.Metadata, "thumbnail") // blog needs a thumbnail to be publication-ready
	_, _ = e.Submit(c)
	_, _ = e.Review(context.Background(), "blog-1")
	_, _ = e.Approve(context.Background(), "blog-1", Reviewer{Name: "r", Role: RoleReviewer}, true, "")
	state, err := e.Publish(context.Background(), "blog-1", "pub")
	if !errors.Is(err, ErrNotReadyToPublish) {
		t.Fatalf("expected ErrNotReadyToPublish, got %v", err)
	}
	if state.Readiness == nil || state.Readiness.Ready {
		t.Error("readiness should be recorded and not ready")
	}
	var sawThumb bool
	for _, ck := range state.Readiness.Checks {
		if ck.Name == "Thumbnail available" && !ck.Passed {
			sawThumb = true
		}
	}
	if !sawThumb {
		t.Error("expected the failing thumbnail check to be reported")
	}
}

func TestArchiveFromNeedsRevision(t *testing.T) {
	e := newEngine(DefaultConfig())
	c := goodBlog()
	c.Body += " It also uses Amazon Redshift for analytics."
	_, _ = e.Submit(c)
	_, _ = e.Review(context.Background(), "blog-1")
	state, err := e.Archive("blog-1", "admin")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if state.Status != StatusArchived {
		t.Errorf("status = %q, want Archived", state.Status)
	}
	// Archived is terminal — no further transitions.
	if _, err := e.Archive("blog-1", "admin"); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("archiving a terminal state should fail, got %v", err)
	}
}

type spyGitHub struct{ changes int }

func (s *spyGitHub) OnStatusChange(context.Context, *WorkflowState, WorkflowStatus, WorkflowStatus) error {
	s.changes++
	return nil
}

func TestGitHubGatewayNotified(t *testing.T) {
	e := newEngine(DefaultConfig())
	spy := &spyGitHub{}
	e.GitHub = spy
	_, _ = e.Submit(goodBlog())
	_, _ = e.Review(context.Background(), "blog-1")
	if spy.changes == 0 {
		t.Error("expected the GitHub gateway to be notified on status changes")
	}
}

func TestValidationVisualAndFrontMatter(t *testing.T) {
	v := ValidationEngine{}
	// Unclosed YAML front matter.
	fm := Content{Type: TypeBlog, Title: "t", Body: "---\ntitle: x\n\n# H\n\nsome body text here that is long enough to pass the completeness floor easily."}
	if !issueCodes(v.Validate(fm))["frontmatter"] {
		t.Error("expected unclosed front-matter error")
	}
	// Thumbnail prompt without a no-text guard warns.
	tp := Content{Type: TypeThumbnailPrompt, Title: "t", Body: "A bold isometric render of a cloud architecture with a headline zone reserved on the left."}
	if !issueCodes(v.Validate(tp))["no-text-guard"] {
		t.Error("expected no-text-guard warning on a thumbnail prompt")
	}
}

func TestLinkedInContentEndToEnd(t *testing.T) {
	e := newEngine(DefaultConfig())
	c := Content{
		ID:    "li-1",
		Type:  TypeLinkedIn,
		Title: "Shipping widget v0.2.0",
		Body: "Just shipped widget v0.2.0. It adds a Release Context builder, built with AWS Lambda and Amazon SQS. " +
			"The important part is that it grounds every downstream artifact in real analysis, so the output stays accurate. " +
			"Read the full write-up on GitHub — link below.",
		Metadata:  map[string]string{"cta": "Explore the repo on GitHub"},
		Grounding: Grounding{Repository: "acme/widget", Release: "v0.2.0", Terms: []string{"release context builder", "AWS Lambda", "Amazon SQS"}},
	}
	_, err := e.Submit(c)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	state, _ := e.Review(context.Background(), "li-1")
	if state.Status != StatusPendingApproval {
		t.Fatalf("grounded LinkedIn post should reach Pending Approval, got %q (decision %q, overall %d)", state.Status, state.LatestReport.Decision, state.LatestReport.Scores.Overall)
	}
	// A reviewer approves (satisfying the required role); a publisher publishes.
	if _, err := e.Approve(context.Background(), "li-1", Reviewer{Name: "rev", Role: RoleReviewer}, true, ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	state, err = e.Publish(context.Background(), "li-1", "p")
	if err != nil {
		t.Fatalf("publish: %v (readiness %+v)", err, state.Readiness)
	}
	if state.Status != StatusPublished {
		t.Errorf("status = %q, want Published", state.Status)
	}
}

func TestListReturnsAll(t *testing.T) {
	e := newEngine(DefaultConfig())
	_, _ = e.Submit(goodBlog())
	c2 := goodBlog()
	c2.ID = "blog-2"
	_, _ = e.Submit(c2)
	all, err := e.List()
	if err != nil || len(all) != 2 {
		t.Errorf("list = %d items, err %v", len(all), err)
	}
}

// --- markdown + json ---

func TestStateMarkdownAndJSON(t *testing.T) {
	e := newEngine(DefaultConfig())
	_, _ = e.Submit(goodBlog())
	state, _ := e.Review(context.Background(), "blog-1")
	md := state.Markdown()
	for _, want := range []string{"# Governance Report", "## Latest Quality Review", "## Audit Trail", "| Overall |"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
	if _, err := json.Marshal(state); err != nil {
		t.Errorf("state should marshal to JSON: %v", err)
	}
	gates := e.Approval.QualityGates(state)
	if !strings.Contains(GatesMarkdown(gates), "## Quality Gates") {
		t.Error("gates markdown malformed")
	}
}

// --- helpers ---

type fakeModel string

func (f fakeModel) Generate(_ context.Context, _ string) (string, error) { return string(f), nil }

func issueCodes(r ValidationReport) map[string]bool {
	m := map[string]bool{}
	for _, i := range r.Issues {
		m[i.Code] = true
	}
	return m
}

func auditActions(s *WorkflowState) map[string]bool {
	m := map[string]bool{}
	for _, e := range s.Audit {
		m[e.Action] = true
	}
	return m
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
