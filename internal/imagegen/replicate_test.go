package imagegen

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestReplicate points a client at a test server with instant backoff.
func newTestReplicate(base string) *ReplicateClient {
	c := NewReplicate("test-token", "owner/model")
	c.baseURL = base
	c.sleep = func(time.Duration) {}
	return c
}

func TestReplicateGenerateSyncSuccess(t *testing.T) {
	var gotAuth, gotPrefer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/predictions"):
			gotAuth = r.Header.Get("Authorization")
			gotPrefer = r.Header.Get("Prefer")
			w.Write([]byte(`{"status":"succeeded","output":["http://` + r.Host + `/img.png"]}`))
		case r.URL.Path == "/img.png":
			w.Write([]byte("PNGDATA"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newTestReplicate(srv.URL)
	got, err := c.Generate(context.Background(), Spec{Prompt: "a diagram", Width: 1280, Height: 720})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if string(got) != "PNGDATA" {
		t.Errorf("image = %q, want PNGDATA", got)
	}
	if gotAuth != "Bearer test-token" || gotPrefer != "wait" {
		t.Errorf("auth=%q prefer=%q", gotAuth, gotPrefer)
	}
}

func TestReplicateSDXLUsesVersionedEndpointWithNegative(t *testing.T) {
	var predBody string
	var resolvedVersion bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/models/stability-ai/sdxl":
			resolvedVersion = true
			w.Write([]byte(`{"latest_version":{"id":"ver-123"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/predictions":
			b, _ := io.ReadAll(r.Body)
			predBody = string(b)
			w.Write([]byte(`{"status":"succeeded","output":["http://` + r.Host + `/img.png"]}`))
		case r.URL.Path == "/img.png":
			w.Write([]byte("PNGDATA"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewReplicate("test-token", "stability-ai/sdxl")
	c.baseURL = srv.URL
	c.sleep = func(time.Duration) {}
	got, err := c.Generate(context.Background(), Spec{Prompt: "isometric shapes", NegativePrompt: "text, letters", Width: 720, Height: 1280})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if string(got) != "PNGDATA" {
		t.Errorf("image = %q", got)
	}
	if !resolvedVersion {
		t.Error("SDXL should resolve its version via GET /models/…")
	}
	// The versioned request must carry the version + width/height + negative_prompt,
	// and NOT the flux-only aspect_ratio/output_format keys.
	for _, want := range []string{`"version":"ver-123"`, `"width":720`, `"height":1280`, `"negative_prompt":"text, letters"`} {
		if !strings.Contains(predBody, want) {
			t.Errorf("prediction body missing %s: %s", want, predBody)
		}
	}
	if strings.Contains(predBody, "aspect_ratio") {
		t.Errorf("SDXL body should not contain aspect_ratio: %s", predBody)
	}
}

func TestReplicateGeneratePollsUntilDone(t *testing.T) {
	var polls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/predictions"):
			w.Write([]byte(`{"status":"processing","urls":{"get":"http://` + r.Host + `/poll"}}`))
		case r.URL.Path == "/poll":
			polls++
			if polls < 2 {
				w.Write([]byte(`{"status":"processing","urls":{"get":"http://` + r.Host + `/poll"}}`))
				return
			}
			w.Write([]byte(`{"status":"succeeded","output":"http://` + r.Host + `/img.png"}`))
		case r.URL.Path == "/img.png":
			w.Write([]byte("IMG"))
		}
	}))
	defer srv.Close()

	c := newTestReplicate(srv.URL)
	got, err := c.Generate(context.Background(), Spec{Prompt: "x", Width: 720, Height: 1280})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if string(got) != "IMG" || polls < 2 {
		t.Errorf("got=%q polls=%d", got, polls)
	}
}

func TestReplicateRetriesOn429(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/predictions") {
			attempts++
			if attempts == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"detail":"rate limited"}`))
				return
			}
			w.Write([]byte(`{"status":"succeeded","output":["http://` + r.Host + `/i"]}`))
			return
		}
		w.Write([]byte("Z"))
	}))
	defer srv.Close()

	c := newTestReplicate(srv.URL)
	got, err := c.Generate(context.Background(), Spec{Prompt: "x", Width: 512, Height: 512})
	if err != nil {
		t.Fatalf("should retry a 429: %v", err)
	}
	if string(got) != "Z" || attempts != 2 {
		t.Errorf("got=%q attempts=%d, want retry then success", got, attempts)
	}
}

func TestReplicateFailedStatusErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"failed","error":"NSFW content detected"}`))
	}))
	defer srv.Close()

	c := newTestReplicate(srv.URL)
	_, err := c.Generate(context.Background(), Spec{Prompt: "x", Width: 512, Height: 512})
	if err == nil || !strings.Contains(err.Error(), "NSFW") {
		t.Errorf("failed prediction should surface the error, got %v", err)
	}
}

func TestReplicateGuards(t *testing.T) {
	if _, err := NewReplicate("tok", "").Generate(context.Background(), Spec{Prompt: "  "}); err == nil {
		t.Error("empty prompt should error")
	}
	if _, err := NewReplicate("", "m").Generate(context.Background(), Spec{Prompt: "x"}); err == nil {
		t.Error("missing token should error")
	}
	if NewReplicate("t", "").model != DefaultReplicateModel {
		t.Error("blank model should default to FLUX schnell")
	}
}

func TestNewFromEnvSelectsReplicate(t *testing.T) {
	t.Setenv("REPLICATE_API_TOKEN", "tok")
	t.Setenv("REPLICATE_IMAGE_MODEL", "stability-ai/sdxl")
	g, err := NewFromEnv(context.Background())
	if err != nil {
		t.Fatalf("NewFromEnv: %v", err)
	}
	rc, ok := g.(*ReplicateClient)
	if !ok {
		t.Fatalf("want *ReplicateClient, got %T", g)
	}
	if rc.model != "stability-ai/sdxl" {
		t.Errorf("model = %q, want the env override", rc.model)
	}
}

func TestAspectFor(t *testing.T) {
	if aspectFor(1280, 720) != "16:9" || aspectFor(720, 1280) != "9:16" {
		t.Error("aspectFor mapping wrong")
	}
}

func TestFirstOutputURL(t *testing.T) {
	if u, _ := firstOutputURL([]byte(`"http://a/x.png"`)); u != "http://a/x.png" {
		t.Errorf("string output = %q", u)
	}
	if u, _ := firstOutputURL([]byte(`["","http://b/y.png"]`)); u != "http://b/y.png" {
		t.Errorf("array output should skip blanks, got %q", u)
	}
	if _, err := firstOutputURL([]byte(`[]`)); err == nil {
		t.Error("empty output should error")
	}
}
