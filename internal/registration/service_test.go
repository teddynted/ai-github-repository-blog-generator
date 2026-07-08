package registration

import (
	"context"
	"encoding/json"
	"errors"
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
	hookErr    error
	gotHookCfg github.WebhookConfig
}

func (f *fakeGitHub) GetRepository(_ context.Context, _, _, _ string) (github.RepoInfo, error) {
	return f.info, f.getErr
}
func (f *fakeGitHub) CreateWebhook(_ context.Context, _, _, _ string, cfg github.WebhookConfig) (int64, error) {
	f.gotHookCfg = cfg
	return f.hookID, f.hookErr
}

type fakeSecrets struct {
	ref    string
	err    error
	gotPAT string
	gotWH  string
	called bool
}

func (f *fakeSecrets) PutRepoCredentials(_ context.Context, _, _, pat, wh string) (string, error) {
	f.called, f.gotPAT, f.gotWH = true, pat, wh
	return f.ref, f.err
}

type fakeMeta struct {
	stored repo.Repository
	err    error
	called bool
}

func (f *fakeMeta) Put(_ context.Context, r repo.Repository) error {
	f.called, f.stored = true, r
	return f.err
}

func newService(gh *fakeGitHub, sec *fakeSecrets, meta *fakeMeta) *Service {
	return &Service{
		GitHub:         gh,
		Secrets:        sec,
		Metadata:       meta,
		WebhookURL:     "https://api.example/webhook",
		DefaultTrigger: "blog:",
		Now:            func() time.Time { return time.Unix(1700000000, 0) },
		NewSecret:      func() (string, error) { return "whsecret", nil },
	}
}

func TestRegisterHappyPath(t *testing.T) {
	gh := &fakeGitHub{info: github.RepoInfo{ID: 7, FullName: "acme/widget", DefaultBranch: "main"}, hookID: 555}
	sec := &fakeSecrets{ref: "blog-gen/repos/acme/widget"}
	meta := &fakeMeta{}
	svc := newService(gh, sec, meta)

	out, err := svc.Register(context.Background(), Input{RepositoryURL: "https://github.com/acme/widget", PAT: "tok"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if out.WebhookID != 555 || out.DefaultBranch != "main" || out.Status != "registered" {
		t.Errorf("out = %+v", out)
	}
	if gh.gotHookCfg.URL != "https://api.example/webhook" || gh.gotHookCfg.Secret != "whsecret" {
		t.Errorf("webhook cfg = %+v", gh.gotHookCfg)
	}
	if !sec.called || sec.gotPAT != "tok" || sec.gotWH != "whsecret" {
		t.Errorf("secrets not stored correctly: %+v", sec)
	}
	if !meta.called {
		t.Fatal("metadata not stored")
	}
	if meta.stored.SecretRef != "blog-gen/repos/acme/widget" || meta.stored.RepositoryID != 7 ||
		meta.stored.TriggerPattern != "blog:" || !meta.stored.Enabled {
		t.Errorf("stored repo = %+v", meta.stored)
	}
	if meta.stored.RegisteredAt == "" {
		t.Error("RegisteredAt not set")
	}
}

func TestRegisterValidatesInput(t *testing.T) {
	svc := newService(&fakeGitHub{}, &fakeSecrets{}, &fakeMeta{})
	for _, in := range []Input{
		{RepositoryURL: "", PAT: "t"},
		{RepositoryURL: "https://github.com/acme/widget", PAT: ""},
		{RepositoryURL: "https://gitlab.com/a/b", PAT: "t"},
	} {
		if _, err := svc.Register(context.Background(), in); apperror.CodeOf(err) != apperror.CodeInvalidInput {
			t.Errorf("input %+v: code = %s, want invalid_input", in, apperror.CodeOf(err))
		}
	}
}

func TestRegisterStopsOnGitHubError(t *testing.T) {
	gh := &fakeGitHub{getErr: apperror.New(apperror.CodeUnauthorized, "bad token")}
	sec := &fakeSecrets{}
	meta := &fakeMeta{}
	svc := newService(gh, sec, meta)

	_, err := svc.Register(context.Background(), Input{RepositoryURL: "https://github.com/acme/widget", PAT: "tok"})
	if apperror.CodeOf(err) != apperror.CodeUnauthorized {
		t.Fatalf("code = %s", apperror.CodeOf(err))
	}
	if sec.called || meta.called {
		t.Error("no credentials or metadata should be written when access validation fails")
	}
}

func TestRegisterDoesNotWriteMetadataWhenWebhookFails(t *testing.T) {
	gh := &fakeGitHub{info: github.RepoInfo{ID: 1}, hookErr: apperror.New(apperror.CodeUnauthorized, "no hook perm")}
	meta := &fakeMeta{}
	svc := newService(gh, &fakeSecrets{}, meta)

	_, err := svc.Register(context.Background(), Input{RepositoryURL: "https://github.com/acme/widget", PAT: "tok"})
	if err == nil {
		t.Fatal("expected error")
	}
	if meta.called {
		t.Error("metadata must not be written when webhook creation fails")
	}
}

func TestHandleJSONSuccess(t *testing.T) {
	gh := &fakeGitHub{info: github.RepoInfo{ID: 7, DefaultBranch: "main"}, hookID: 5}
	h := &Handler{Service: newService(gh, &fakeSecrets{ref: "ref"}, &fakeMeta{})}

	status, body := h.HandleJSON(context.Background(), []byte(`{"repository_url":"https://github.com/acme/widget","pat":"tok"}`))
	if status != 200 {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var out Output
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("bad response json: %v", err)
	}
	if out.RepoFullName != "acme/widget" {
		t.Errorf("out = %+v", out)
	}
}

func TestHandleJSONBadBody(t *testing.T) {
	h := &Handler{Service: newService(&fakeGitHub{}, &fakeSecrets{}, &fakeMeta{})}
	status, body := h.HandleJSON(context.Background(), []byte(`not json`))
	if status != 400 {
		t.Fatalf("status = %d", status)
	}
	var e errorBody
	if err := json.Unmarshal(body, &e); err != nil || e.Error != string(apperror.CodeInvalidInput) {
		t.Errorf("error body = %s (%v)", body, err)
	}
}

func TestHandleJSONMapsErrorStatus(t *testing.T) {
	gh := &fakeGitHub{getErr: apperror.New(apperror.CodeNotFound, "nope")}
	h := &Handler{Service: newService(gh, &fakeSecrets{}, &fakeMeta{})}
	status, _ := h.HandleJSON(context.Background(), []byte(`{"repository_url":"https://github.com/a/b","pat":"t"}`))
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
}

// Guard: the concrete github.Client satisfies the GitHub port.
var _ GitHub = (*github.Client)(nil)

// Guard: apperror composes through wrapping for the handler's status mapping.
func TestWrappedErrorStatus(t *testing.T) {
	err := apperror.Wrap(errors.New("x"), apperror.CodeConflict, "dup")
	if apperror.HTTPStatusOf(err) != 409 {
		t.Errorf("status = %d", apperror.HTTPStatusOf(err))
	}
}
