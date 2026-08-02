package registration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
)

// --- fakes ---

type fakeGitHub struct {
	info       github.RepoInfo
	getErr     error
	hookID     int64
	createErr  error
	updateErr  error
	deleteErr  error
	created    bool
	updated    bool
	deleted    bool
	gotHookCfg github.WebhookConfig
}

func (f *fakeGitHub) GetRepository(_ context.Context, _, _, _ string) (github.RepoInfo, error) {
	return f.info, f.getErr
}
func (f *fakeGitHub) CreateWebhook(_ context.Context, _, _, _ string, cfg github.WebhookConfig) (int64, error) {
	f.created, f.gotHookCfg = true, cfg
	return f.hookID, f.createErr
}
func (f *fakeGitHub) UpdateWebhook(_ context.Context, _, _, _ string, _ int64, cfg github.WebhookConfig) error {
	f.updated, f.gotHookCfg = true, cfg
	return f.updateErr
}
func (f *fakeGitHub) DeleteWebhook(_ context.Context, _, _, _ string, _ int64) error {
	f.deleted = true
	return f.deleteErr
}

type fakeSecrets struct {
	existed   bool
	putErr    error
	pat       string
	putCalled bool
	delCalled bool
	gotPAT    string
	gotWH     string
	allowUpd  bool
}

func (f *fakeSecrets) PutRepoCredentials(_ context.Context, _, pat, wh string, allowUpdate bool) (bool, error) {
	f.putCalled, f.gotPAT, f.gotWH, f.allowUpd = true, pat, wh, allowUpdate
	return f.existed, f.putErr
}
func (f *fakeSecrets) DeleteRepoCredentials(_ context.Context, _ string) (bool, error) {
	f.delCalled = true
	return true, nil
}
func (f *fakeSecrets) PAT(_ context.Context, _ string) (string, error) { return f.pat, nil }

type fakeMeta struct {
	existing  repo.Repository
	found     bool
	getErr    error
	putCalled bool
	delCalled bool
	stored    repo.Repository
}

func (f *fakeMeta) Put(_ context.Context, r repo.Repository) error {
	f.putCalled, f.stored = true, r
	return nil
}
func (f *fakeMeta) Get(_ context.Context, _ string) (repo.Repository, bool, error) {
	return f.existing, f.found, f.getErr
}
func (f *fakeMeta) Delete(_ context.Context, _ string) error { f.delCalled = true; return nil }

func newService(gh *fakeGitHub, sec *fakeSecrets, meta *fakeMeta) *Service {
	return &Service{
		GitHub:         gh,
		Secrets:        sec,
		Metadata:       meta,
		WebhookURL:     "https://api.example/webhook",
		DefaultTrigger: "blog:",
		Now:            func() time.Time { return time.Unix(1700000000, 0) },
	}
}

func validInput() Input {
	return Input{Owner: "acme", Repository: "widget", PAT: "tok", WebhookSecret: "whsec"}
}

func TestRegisterInsert(t *testing.T) {
	gh := &fakeGitHub{info: github.RepoInfo{ID: 7, DefaultBranch: "main"}, hookID: 555}
	sec := &fakeSecrets{}
	meta := &fakeMeta{found: false}
	out, err := newService(gh, sec, meta).Register(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if out.RepoFullName != "acme/widget" || out.Updated {
		t.Errorf("out = %+v", out)
	}
	if !gh.created || gh.updated {
		t.Error("insert must create (not update) the webhook")
	}
	if gh.gotHookCfg.Secret != "whsec" {
		t.Errorf("webhook secret = %q, want the supplied one", gh.gotHookCfg.Secret)
	}
	if !sec.putCalled || sec.gotPAT != "tok" || sec.gotWH != "whsec" {
		t.Errorf("credentials not stored: %+v", sec)
	}
	if meta.stored.SecretRef != "acme/widget" || meta.stored.WebhookID != 555 || !meta.stored.Enabled {
		t.Errorf("stored repo = %+v", meta.stored)
	}
}

func TestRegisterDuplicateRejected(t *testing.T) {
	gh := &fakeGitHub{}
	sec := &fakeSecrets{}
	meta := &fakeMeta{found: true, existing: repo.Repository{WebhookID: 9}}
	_, err := newService(gh, sec, meta).Register(context.Background(), validInput())
	if apperror.CodeOf(err) != apperror.CodeConflict {
		t.Fatalf("code = %s, want conflict", apperror.CodeOf(err))
	}
	if gh.created || gh.updated || sec.putCalled || meta.putCalled {
		t.Error("a duplicate must not touch GitHub, secrets, or metadata")
	}
}

func TestRegisterUpdate(t *testing.T) {
	gh := &fakeGitHub{info: github.RepoInfo{ID: 7, DefaultBranch: "main"}}
	sec := &fakeSecrets{existed: true}
	meta := &fakeMeta{found: true, existing: repo.Repository{WebhookID: 42}}
	in := validInput()
	in.Update = true
	in.WebhookSecret = "rotated"
	out, err := newService(gh, sec, meta).Register(context.Background(), in)
	if err != nil {
		t.Fatalf("Register(update): %v", err)
	}
	if !out.Updated {
		t.Error("expected Updated=true")
	}
	if !gh.updated || gh.created {
		t.Error("update must PATCH (not create) the webhook")
	}
	if gh.gotHookCfg.Secret != "rotated" || !sec.allowUpd {
		t.Errorf("rotated secret not propagated: cfg=%q allowUpd=%v", gh.gotHookCfg.Secret, sec.allowUpd)
	}
}

func TestRegisterValidatesRequiredFields(t *testing.T) {
	svc := newService(&fakeGitHub{}, &fakeSecrets{}, &fakeMeta{})
	for _, in := range []Input{
		{Repository: "w", PAT: "t"},   // no owner
		{Owner: "o", PAT: "t"},        // no repository
		{Owner: "o", Repository: "w"}, // no pat
	} {
		if _, err := svc.Register(context.Background(), in); apperror.CodeOf(err) != apperror.CodeInvalidInput {
			t.Errorf("input %+v: code = %s, want invalid_input", in, apperror.CodeOf(err))
		}
	}
}

func TestRegisterStopsOnGitHubError(t *testing.T) {
	gh := &fakeGitHub{getErr: apperror.New(apperror.CodeUnauthorized, "bad token")}
	sec, meta := &fakeSecrets{}, &fakeMeta{}
	_, err := newService(gh, sec, meta).Register(context.Background(), validInput())
	if apperror.CodeOf(err) != apperror.CodeUnauthorized {
		t.Fatalf("code = %s", apperror.CodeOf(err))
	}
	if sec.putCalled || meta.putCalled {
		t.Error("nothing should be written when access validation fails")
	}
}

func TestRegisterNoMetadataWhenWebhookFails(t *testing.T) {
	gh := &fakeGitHub{info: github.RepoInfo{ID: 1}, createErr: apperror.New(apperror.CodeUnauthorized, "no hook perm")}
	sec, meta := &fakeSecrets{}, &fakeMeta{}
	if _, err := newService(gh, sec, meta).Register(context.Background(), validInput()); err == nil {
		t.Fatal("expected error")
	}
	if sec.putCalled || meta.putCalled {
		t.Error("no credentials or metadata when webhook creation fails")
	}
}

func TestDeleteRemovesEverything(t *testing.T) {
	gh := &fakeGitHub{}
	sec := &fakeSecrets{pat: "tok"}
	meta := &fakeMeta{found: true, existing: repo.Repository{WebhookID: 77}}
	full, err := newService(gh, sec, meta).Delete(context.Background(), DeleteInput{Owner: "acme", Repository: "widget"})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if full != "acme/widget" {
		t.Errorf("full = %q", full)
	}
	if !gh.deleted || !sec.delCalled || !meta.delCalled {
		t.Errorf("delete must remove webhook, secret, metadata: gh=%v sec=%v meta=%v", gh.deleted, sec.delCalled, meta.delCalled)
	}
}

func TestDeleteUnregisteredIsNotFound(t *testing.T) {
	meta := &fakeMeta{found: false}
	_, err := newService(&fakeGitHub{}, &fakeSecrets{}, meta).Delete(context.Background(), DeleteInput{Owner: "a", Repository: "b"})
	if apperror.CodeOf(err) != apperror.CodeNotFound {
		t.Errorf("code = %s, want not_found", apperror.CodeOf(err))
	}
}

// --- handler ---

func TestHandlerRegisterSuccess(t *testing.T) {
	gh := &fakeGitHub{info: github.RepoInfo{ID: 7, DefaultBranch: "main"}, hookID: 5}
	h := &Handler{Service: newService(gh, &fakeSecrets{}, &fakeMeta{})}
	status, body := h.Register(context.Background(), []byte(`{"owner":"acme","repository":"widget","pat":"tok","webhook_secret":"ws"}`))
	if status != 200 {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var r response
	_ = json.Unmarshal(body, &r)
	if r.Status != "success" || r.Repository != "acme/widget" || r.Message == "" {
		t.Errorf("response = %+v", r)
	}
}

func TestHandlerRegisterDuplicateIs409(t *testing.T) {
	meta := &fakeMeta{found: true, existing: repo.Repository{WebhookID: 1}}
	h := &Handler{Service: newService(&fakeGitHub{}, &fakeSecrets{}, meta)}
	status, body := h.Register(context.Background(), []byte(`{"owner":"acme","repository":"widget","pat":"t","webhook_secret":"s"}`))
	if status != 409 {
		t.Fatalf("status = %d, want 409", status)
	}
	var r response
	_ = json.Unmarshal(body, &r)
	if r.Status != "error" || r.Message != "Repository is already registered." {
		t.Errorf("response = %+v", r)
	}
}

func TestHandlerValidationIs400(t *testing.T) {
	h := &Handler{Service: newService(&fakeGitHub{}, &fakeSecrets{}, &fakeMeta{})}
	status, _ := h.Register(context.Background(), []byte(`{"owner":"acme"}`)) // missing fields
	if status != 400 {
		t.Errorf("status = %d, want 400", status)
	}
	status, _ = h.Register(context.Background(), []byte(`not json`))
	if status != 400 {
		t.Errorf("bad json status = %d, want 400", status)
	}
}

func TestHandlerDelete(t *testing.T) {
	meta := &fakeMeta{found: true, existing: repo.Repository{WebhookID: 3}}
	h := &Handler{Service: newService(&fakeGitHub{}, &fakeSecrets{pat: "t"}, meta)}
	status, body := h.Delete(context.Background(), []byte(`{"owner":"acme","repository":"widget"}`))
	if status != 200 {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var r response
	_ = json.Unmarshal(body, &r)
	if r.Status != "success" || r.Repository != "acme/widget" {
		t.Errorf("response = %+v", r)
	}
}

// Guard: the concrete github.Client satisfies the GitHub port.
var _ GitHub = (*github.Client)(nil)
