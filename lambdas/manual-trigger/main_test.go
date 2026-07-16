package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/intake"
)

type fakePub struct {
	published []intake.Event
	err       error
}

func (f *fakePub) Publish(_ context.Context, ev intake.Event) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, ev)
	return nil
}

type fakeWindow struct {
	open bool
	err  error
}

func (w fakeWindow) Open(_ context.Context) (bool, error) { return w.open, w.err }

func svc(pub *fakePub, w intake.Window) *intake.Service {
	return &intake.Service{Publisher: pub, Window: w}
}

func TestHandleAcceptedInWindow(t *testing.T) {
	pub := &fakePub{}
	body := []byte(`{"repository":"widget","owner":"acme","provider":"bedrock"}`)
	status, resp := handle(context.Background(), svc(pub, fakeWindow{open: true}), nil, "req-1", body)

	if status != 202 {
		t.Fatalf("status = %d, want 202; body=%s", status, resp)
	}
	if len(pub.published) != 1 || pub.published[0].RepoFullName != "acme/widget" || pub.published[0].Source != "manual" {
		t.Fatalf("published = %+v", pub.published)
	}
	var r acceptedResponse
	_ = json.Unmarshal(resp, &r)
	if r.Status != "accepted" || r.Trigger != "manual" || r.RequestID != "req-1" || r.Message == "" {
		t.Errorf("response = %+v", r)
	}
}

func TestHandleRejectedOutsideWindow(t *testing.T) {
	pub := &fakePub{}
	body := []byte(`{"repository":"widget","owner":"acme"}`)
	status, resp := handle(context.Background(), svc(pub, fakeWindow{open: false}), nil, "req-2", body)

	if status != 503 {
		t.Fatalf("status = %d, want 503", status)
	}
	if len(pub.published) != 0 {
		t.Errorf("rejected must not publish or start the instance, got %d", len(pub.published))
	}
	var r errorResponse
	_ = json.Unmarshal(resp, &r)
	if r.Status != "rejected" || r.Reason == "" {
		t.Errorf("response = %+v", r)
	}
}

func TestHandleValidationError(t *testing.T) {
	pub := &fakePub{}
	body := []byte(`{"owner":"acme"}`) // missing repository
	status, resp := handle(context.Background(), svc(pub, fakeWindow{open: true}), nil, "req-3", body)
	if status != 400 {
		t.Fatalf("status = %d, want 400", status)
	}
	var r errorResponse
	_ = json.Unmarshal(resp, &r)
	if r.Status != "error" || r.Reason == "" {
		t.Errorf("response = %+v", r)
	}
	if len(pub.published) != 0 {
		t.Error("must not publish an invalid request")
	}
}

func TestHandleInvalidJSON(t *testing.T) {
	status, _ := handle(context.Background(), svc(&fakePub{}, fakeWindow{open: true}), nil, "req-4", []byte("not json"))
	if status != 400 {
		t.Errorf("status = %d, want 400", status)
	}
}

func TestHandlePublishFailureIs500(t *testing.T) {
	pub := &fakePub{err: errors.New("put failed")}
	body := []byte(`{"repository":"widget","owner":"acme"}`)
	status, _ := handle(context.Background(), svc(pub, fakeWindow{open: true}), nil, "req-5", body)
	if status != 500 {
		t.Errorf("status = %d, want 500", status)
	}
}
