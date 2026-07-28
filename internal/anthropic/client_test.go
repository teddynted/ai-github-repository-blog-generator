package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// stubDoer captures the request and returns a canned response.
type stubDoer struct {
	gotReq  *http.Request
	gotBody []byte
	status  int
	respRaw string
	err     error
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.gotReq = req
	if req.Body != nil {
		s.gotBody, _ = io.ReadAll(req.Body)
	}
	if s.err != nil {
		return nil, s.err
	}
	code := s.status
	if code == 0 {
		code = 200
	}
	return &http.Response{
		StatusCode: code,
		Status:     http.StatusText(code),
		Body:       io.NopCloser(strings.NewReader(s.respRaw)),
		Header:     make(http.Header),
	}, nil
}

func TestGenerateSendsCorrectRequestAndParsesText(t *testing.T) {
	d := &stubDoer{respRaw: `{"content":[{"type":"text","text":"# Blog\n\nBody."}],"stop_reason":"end_turn"}`}
	c, err := New(Config{APIKey: "sk-test", System: "You are the lead engineer.", Temperature: 0.6, HTTP: d})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := c.Generate(context.Background(), "Write about v0.3.0")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(got, "# Blog") {
		t.Errorf("text = %q", got)
	}
	// Headers.
	if d.gotReq.Header.Get("x-api-key") != "sk-test" {
		t.Errorf("x-api-key = %q", d.gotReq.Header.Get("x-api-key"))
	}
	if d.gotReq.Header.Get("anthropic-version") != apiVersion {
		t.Errorf("anthropic-version = %q", d.gotReq.Header.Get("anthropic-version"))
	}
	if !strings.HasSuffix(d.gotReq.URL.Path, "/v1/messages") {
		t.Errorf("path = %q", d.gotReq.URL.Path)
	}
	// Body.
	var body requestBody
	if err := json.Unmarshal(d.gotBody, &body); err != nil {
		t.Fatalf("request body invalid: %v", err)
	}
	if body.Model != DefaultModel || body.MaxTokens != 8192 {
		t.Errorf("model/max = %q/%d", body.Model, body.MaxTokens)
	}
	if body.System != "You are the lead engineer." {
		t.Errorf("system = %q", body.System)
	}
	if len(body.Messages) != 1 || body.Messages[0].Role != "user" || body.Messages[0].Content != "Write about v0.3.0" {
		t.Errorf("messages = %+v", body.Messages)
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(Config{APIKey: "  "}); err == nil {
		t.Fatal("expected error with no API key")
	}
}

func TestGenerateAPIError(t *testing.T) {
	d := &stubDoer{status: 401, respRaw: `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`}
	c, _ := New(Config{APIKey: "bad", HTTP: d})
	_, err := c.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "authentication_error") {
		t.Fatalf("expected API error surfaced, got %v", err)
	}
}

func TestGenerateEmptyPrompt(t *testing.T) {
	c, _ := New(Config{APIKey: "k", HTTP: &stubDoer{}})
	if _, err := c.Generate(context.Background(), "  "); err == nil {
		t.Fatal("expected error on empty prompt")
	}
}

func TestGenerateEmptyCompletion(t *testing.T) {
	d := &stubDoer{respRaw: `{"content":[],"stop_reason":"end_turn"}`}
	c, _ := New(Config{APIKey: "k", HTTP: d})
	if _, err := c.Generate(context.Background(), "hi"); err == nil {
		t.Fatal("expected error on empty completion")
	}
}

func TestStringRedactsKey(t *testing.T) {
	c, _ := New(Config{APIKey: "sk-super-secret", HTTP: &stubDoer{}})
	s := c.String()
	if strings.Contains(s, "sk-super-secret") || !strings.Contains(s, "REDACTED") {
		t.Errorf("String() leaked key or missing redaction: %q", s)
	}
}
