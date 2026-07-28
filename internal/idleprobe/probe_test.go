package idleprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestN8NBusyWhenRunningExecution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-N8N-API-KEY"); got != "k" {
			t.Errorf("api key header = %q", got)
		}
		w.Write([]byte(`{"data":[{"id":"1"}]}`))
	}))
	defer srv.Close()
	if !N8N(srv.URL, "k", time.Second, nil).Busy(context.Background()) {
		t.Error("expected busy when an execution is running")
	}
}

func TestN8NIdleWhenNoExecutions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	if N8N(srv.URL, "k", time.Second, nil).Busy(context.Background()) {
		t.Error("expected idle when no executions are running")
	}
}

func TestOllamaBusyWhenModelLoaded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"models":[{"name":"llama3"}]}`))
	}))
	defer srv.Close()
	if !Ollama(srv.URL, time.Second, nil).Busy(context.Background()) {
		t.Error("expected busy when a model is resident")
	}
}

func TestFailSafeBusyOnUnreachable(t *testing.T) {
	// Nothing listening — an unreachable probe must count as BUSY, never idle.
	p := Ollama("http://127.0.0.1:1", 200*time.Millisecond, nil)
	if !p.Busy(context.Background()) {
		t.Error("unreachable probe must fail safe to busy")
	}
}

func TestFailSafeBusyOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if !N8N(srv.URL, "k", time.Second, nil).Busy(context.Background()) {
		t.Error("non-2xx must fail safe to busy")
	}
}
