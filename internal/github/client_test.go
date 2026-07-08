package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

func TestGetRepositorySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/widget" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth header = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 42, "full_name": "acme/widget", "default_branch": "main",
		})
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	info, err := c.GetRepository(context.Background(), "acme", "widget", "tok")
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}
	if info.ID != 42 || info.FullName != "acme/widget" || info.DefaultBranch != "main" {
		t.Errorf("info = %+v", info)
	}
}

func TestGetRepositoryStatusMapping(t *testing.T) {
	cases := map[int]apperror.Code{
		http.StatusUnauthorized: apperror.CodeUnauthorized,
		http.StatusForbidden:    apperror.CodeUnauthorized,
		http.StatusNotFound:     apperror.CodeNotFound,
	}
	for status, wantCode := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		_, err := New(WithBaseURL(srv.URL)).GetRepository(context.Background(), "a", "b", "tok")
		srv.Close()
		if err == nil {
			t.Fatalf("status %d: expected error", status)
		}
		if apperror.CodeOf(err) != wantCode {
			t.Errorf("status %d: code = %s, want %s", status, apperror.CodeOf(err), wantCode)
		}
	}
}

func TestCreateWebhook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/acme/widget/hooks" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			Events []string       `json:"events"`
			Config map[string]any `json:"config"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		if len(payload.Events) != 1 || payload.Events[0] != "push" {
			t.Errorf("events = %v", payload.Events)
		}
		if payload.Config["secret"] != "whsec" || payload.Config["url"] != "https://api/webhook" {
			t.Errorf("config = %v", payload.Config)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 999})
	}))
	defer srv.Close()

	id, err := New(WithBaseURL(srv.URL)).CreateWebhook(context.Background(), "acme", "widget", "tok",
		WebhookConfig{URL: "https://api/webhook", Secret: "whsec", Events: []string{"push"}})
	if err != nil {
		t.Fatalf("CreateWebhook: %v", err)
	}
	if id != 999 {
		t.Errorf("id = %d, want 999", id)
	}
}

func TestCreateWebhookForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := New(WithBaseURL(srv.URL)).CreateWebhook(context.Background(), "a", "b", "tok",
		WebhookConfig{Events: []string{"push"}})
	if apperror.CodeOf(err) != apperror.CodeUnauthorized {
		t.Errorf("code = %s, want unauthorized", apperror.CodeOf(err))
	}
}
