package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/retry"
)

var fastRetry = retry.Config{MaxAttempts: 3, BaseDelay: 0}

func TestWebhookNotifierPostsPayload(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhook(srv.URL, nil, WithWebhookRetry(fastRetry))
	if err := n.Notify(context.Background(), Event{Repo: "acme/widget", Status: StatusPublished, Assets: 3}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	var p map[string]any
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	if p["status"] != "published" || p["repo"] != "acme/widget" {
		t.Errorf("payload = %v", p)
	}
	text, _ := p["text"].(string)
	if text == "" || p["assets"].(float64) != 3 {
		t.Errorf("expected Slack-compatible text + assets: %v", p)
	}
}

func TestWebhookNotifierRetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhook(srv.URL, nil, WithWebhookRetry(fastRetry))
	if err := n.Notify(context.Background(), Event{Repo: "a/b", Status: StatusFailed, Err: "boom"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected retry to 3rd attempt, calls=%d", calls.Load())
	}
}

func TestMultiFansOut(t *testing.T) {
	var a, b []Event
	m := Multi{
		notifierFunc(func(e Event) error { a = append(a, e); return nil }),
		notifierFunc(func(e Event) error { b = append(b, e); return nil }),
	}
	_ = m.Notify(context.Background(), Event{Repo: "a/b", Status: StatusPublished})
	if len(a) != 1 || len(b) != 1 {
		t.Errorf("both notifiers should be called: a=%d b=%d", len(a), len(b))
	}
}

type notifierFunc func(Event) error

func (f notifierFunc) Notify(_ context.Context, e Event) error { return f(e) }
