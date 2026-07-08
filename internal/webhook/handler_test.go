package webhook

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/githubsig"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
)

const secret = "whsecret"

type fakeRepos struct {
	r   repo.Repository
	ok  bool
	err error
}

func (f *fakeRepos) Get(_ context.Context, _ string) (repo.Repository, bool, error) {
	return f.r, f.ok, f.err
}

type fakeSecrets struct {
	secret string
	err    error
}

func (f *fakeSecrets) WebhookSecret(_ context.Context, _ string) (string, error) {
	return f.secret, f.err
}

type fakePublisher struct {
	published []Event
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, ev Event) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, ev)
	return nil
}

func registeredRepo() repo.Repository {
	return repo.Repository{
		RepoFullName:   "acme/widget",
		Owner:          "acme",
		Name:           "widget",
		TriggerPattern: "blog:",
		Enabled:        true,
		SecretRef:      "blog-gen/repos/acme/widget",
	}
}

func pushBody(fullName, message string) []byte {
	b, _ := json.Marshal(map[string]any{
		"ref":        "refs/heads/main",
		"after":      "abc123",
		"repository": map[string]any{"full_name": fullName},
		"head_commit": map[string]any{
			"id": "abc123", "message": message,
		},
	})
	return b
}

type fakeCounter struct{ counts map[string]int }

func (f *fakeCounter) Count(name string) {
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.counts[name]++
}

func newHandler(pub *fakePublisher) *Handler {
	return &Handler{
		Repos:          &fakeRepos{r: registeredRepo(), ok: true},
		Secrets:        &fakeSecrets{secret: secret},
		Publisher:      pub,
		DefaultTrigger: "blog:",
	}
}

func signedHeaders(body []byte) map[string]string {
	return map[string]string{
		"X-GitHub-Event":      "push",
		"X-GitHub-Delivery":   "d-1",
		"X-Hub-Signature-256": githubsig.Sign(secret, body),
	}
}

func TestMatchedCommitPublishes(t *testing.T) {
	body := pushBody("acme/widget", "blog: new feature")
	pub := &fakePublisher{}
	status, resp := newHandler(pub).Handle(context.Background(), signedHeaders(body), body)

	if status != 200 {
		t.Fatalf("status = %d, resp = %s", status, resp)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.published))
	}
	ev := pub.published[0]
	if ev.RepoFullName != "acme/widget" || ev.CommitMessage != "blog: new feature" || ev.CommitSHA != "abc123" {
		t.Errorf("event = %+v", ev)
	}
	var r result
	_ = json.Unmarshal(resp, &r)
	if r.Status != "accepted" {
		t.Errorf("status field = %q", r.Status)
	}
}

func TestRoutineCommitIgnored(t *testing.T) {
	body := pushBody("acme/widget", "fix(api): resolve issue")
	pub := &fakePublisher{}
	status, resp := newHandler(pub).Handle(context.Background(), signedHeaders(body), body)

	if status != 200 {
		t.Fatalf("status = %d", status)
	}
	if len(pub.published) != 0 {
		t.Error("routine commit must not publish")
	}
	var r result
	_ = json.Unmarshal(resp, &r)
	if r.Status != "ignored" {
		t.Errorf("status field = %q, want ignored", r.Status)
	}
}

func TestInvalidSignatureRejected(t *testing.T) {
	body := pushBody("acme/widget", "blog: x")
	headers := signedHeaders(body)
	headers["X-Hub-Signature-256"] = "sha256=deadbeef"
	pub := &fakePublisher{}
	status, _ := newHandler(pub).Handle(context.Background(), headers, body)

	if status != 401 {
		t.Fatalf("status = %d, want 401", status)
	}
	if len(pub.published) != 0 {
		t.Error("must not publish on invalid signature")
	}
}

func TestUnregisteredRepoIs404(t *testing.T) {
	body := pushBody("acme/widget", "blog: x")
	h := newHandler(&fakePublisher{})
	h.Repos = &fakeRepos{ok: false}
	status, _ := h.Handle(context.Background(), signedHeaders(body), body)
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
}

func TestNonPushEventIgnored(t *testing.T) {
	body := pushBody("acme/widget", "blog: x")
	headers := signedHeaders(body)
	headers["X-GitHub-Event"] = "ping"
	pub := &fakePublisher{}
	status, resp := newHandler(pub).Handle(context.Background(), headers, body)
	if status != 200 || len(pub.published) != 0 {
		t.Fatalf("ping should be ignored 200: status=%d published=%d", status, len(pub.published))
	}
	var r result
	_ = json.Unmarshal(resp, &r)
	if r.Status != "ignored" {
		t.Errorf("status = %q", r.Status)
	}
}

func TestDisabledRepoIgnored(t *testing.T) {
	body := pushBody("acme/widget", "blog: x")
	h := newHandler(&fakePublisher{})
	dr := registeredRepo()
	dr.Enabled = false
	h.Repos = &fakeRepos{r: dr, ok: true}
	status, resp := h.Handle(context.Background(), signedHeaders(body), body)
	var r result
	_ = json.Unmarshal(resp, &r)
	if status != 200 || r.Status != "ignored" {
		t.Errorf("disabled repo: status=%d field=%q", status, r.Status)
	}
}

func TestPerRepoTriggerPatternHonoured(t *testing.T) {
	body := pushBody("acme/widget", "[blog] via custom pattern")
	h := newHandler(&fakePublisher{})
	cr := registeredRepo()
	cr.TriggerPattern = "[blog]"
	h.Repos = &fakeRepos{r: cr, ok: true}
	pub := &fakePublisher{}
	h.Publisher = pub
	status, _ := h.Handle(context.Background(), signedHeaders(body), body)
	if status != 200 || len(pub.published) != 1 {
		t.Errorf("custom pattern should match: status=%d published=%d", status, len(pub.published))
	}
}

func TestUnparseablePayloadIs400(t *testing.T) {
	body := []byte(`{"not":"a webhook"}`)
	status, _ := newHandler(&fakePublisher{}).Handle(context.Background(), map[string]string{"X-GitHub-Event": "push"}, body)
	if status != 400 {
		t.Errorf("status = %d, want 400", status)
	}
}

func TestMetricsEmitted(t *testing.T) {
	// matched commit -> WebhookReceived + TriggerMatched
	body := pushBody("acme/widget", "blog: x")
	mc := &fakeCounter{}
	h := newHandler(&fakePublisher{})
	h.Metrics = mc
	h.Handle(context.Background(), signedHeaders(body), body)
	if mc.counts["WebhookReceived"] != 1 || mc.counts["TriggerMatched"] != 1 {
		t.Errorf("matched metrics = %v", mc.counts)
	}

	// routine commit -> WebhookIgnored
	mc2 := &fakeCounter{}
	h.Metrics = mc2
	rbody := pushBody("acme/widget", "fix: x")
	h.Handle(context.Background(), signedHeaders(rbody), rbody)
	if mc2.counts["WebhookIgnored"] != 1 || mc2.counts["TriggerMatched"] != 0 {
		t.Errorf("ignored metrics = %v", mc2.counts)
	}

	// bad signature -> WebhookRejected
	mc3 := &fakeCounter{}
	h.Metrics = mc3
	bbody := pushBody("acme/widget", "blog: x")
	bh := signedHeaders(bbody)
	bh["X-Hub-Signature-256"] = "sha256=bad"
	h.Handle(context.Background(), bh, bbody)
	if mc3.counts["WebhookRejected"] != 1 {
		t.Errorf("rejected metrics = %v", mc3.counts)
	}
}
