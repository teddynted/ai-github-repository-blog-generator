package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

func TestGenerateSendsRequestAndReturnsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/generate" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var req generateRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if req.Model != "qwen2.5:7b" || req.Prompt != "hello" || req.Stream {
			t.Errorf("request = %+v", req)
		}
		_ = json.NewEncoder(w).Encode(generateResponse{Response: "# Post\nbody", Done: true})
	}))
	defer srv.Close()

	out, err := New("qwen2.5:7b", WithBaseURL(srv.URL)).Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "# Post\nbody" {
		t.Errorf("out = %q", out)
	}
}

func TestGenerateMapsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New("m", WithBaseURL(srv.URL)).Generate(context.Background(), "x")
	if apperror.CodeOf(err) != apperror.CodeUpstream {
		t.Errorf("code = %s, want upstream", apperror.CodeOf(err))
	}
}

func TestModelAccessor(t *testing.T) {
	if New("qwen2.5:7b").Model() != "qwen2.5:7b" {
		t.Error("Model() mismatch")
	}
}
