package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

type fakeBuilder struct {
	out *rc.ReleaseContext
	err error
}

func (f *fakeBuilder) Build(context.Context, rc.Request) (*rc.ReleaseContext, error) {
	return f.out, f.err
}

type fakeStore struct {
	saved   []byte
	savedID string
	loc     string
	err     error
}

func (f *fakeStore) Save(_ context.Context, id string, data []byte) (string, error) {
	f.savedID, f.saved = id, data
	return f.loc, f.err
}

func TestHandleSuccess(t *testing.T) {
	b := &fakeBuilder{out: &rc.ReleaseContext{ContextID: "ctx-123", Warnings: []string{"no CHANGELOG"}}}
	st := &fakeStore{loc: "s3://bucket/release-contexts/x.json"}
	body := []byte(`{"owner":"acme","repository":"widget","releaseTag":"v0.2.0"}`)

	status, resp := handle(context.Background(), b, st, nil, "req-1", body)
	if status != 202 {
		t.Fatalf("status = %d, want 202: %s", status, resp)
	}
	var out acceptedResponse
	if err := json.Unmarshal(resp, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Status != "accepted" || out.ContextID != "ctx-123" || out.Repository != "widget" || out.ReleaseTag != "v0.2.0" {
		t.Errorf("response = %+v", out)
	}
	if out.Location != st.loc || out.Warnings != 1 {
		t.Errorf("location/warnings = %q / %d", out.Location, out.Warnings)
	}
	if st.savedID != "ctx-123" || len(st.saved) == 0 {
		t.Error("context was not persisted")
	}
}

func TestHandleBadJSON(t *testing.T) {
	status, _ := handle(context.Background(), &fakeBuilder{}, nil, nil, "req", []byte(`{not json`))
	if status != 400 {
		t.Errorf("status = %d, want 400", status)
	}
}

func TestHandleErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"invalid", apperror.New(apperror.CodeInvalidInput, "releaseTag is required"), 400},
		{"not found", apperror.New(apperror.CodeNotFound, "nope"), 404},
		{"unauthorized", apperror.New(apperror.CodeUnauthorized, "nope"), 403},
		{"upstream", apperror.New(apperror.CodeUpstream, "503"), 502},
		{"internal", errors.New("boom"), 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := handle(context.Background(), &fakeBuilder{err: tc.err}, nil, nil, "req",
				[]byte(`{"owner":"a","repository":"b","releaseTag":"v1"}`))
			if status != tc.want {
				t.Errorf("status = %d, want %d", status, tc.want)
			}
		})
	}
}

func TestHandleNoStore(t *testing.T) {
	b := &fakeBuilder{out: &rc.ReleaseContext{ContextID: "ctx-9"}}
	status, resp := handle(context.Background(), b, nil, nil, "req", []byte(`{"owner":"a","repository":"b","releaseTag":"v1"}`))
	if status != 202 {
		t.Fatalf("status = %d", status)
	}
	var out acceptedResponse
	_ = json.Unmarshal(resp, &out)
	if out.Location != "" {
		t.Errorf("location should be empty without a store, got %q", out.Location)
	}
}
