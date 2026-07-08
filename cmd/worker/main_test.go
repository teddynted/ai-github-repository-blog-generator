package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/awssqs"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/pipeline"
)

type fakeConsumer struct {
	deleted []string
}

func (f *fakeConsumer) Receive(context.Context, int32, int32) ([]awssqs.Message, error) {
	return nil, nil
}
func (f *fakeConsumer) Delete(_ context.Context, receiptHandle string) error {
	f.deleted = append(f.deleted, receiptHandle)
	return nil
}

type fakeRunner struct {
	ran []pipeline.Request
	err error
}

func (f *fakeRunner) Run(_ context.Context, req pipeline.Request) (pipeline.Result, error) {
	f.ran = append(f.ran, req)
	return pipeline.Result{RepoFullName: req.RepoFullName}, f.err
}

func discardLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func msg(body string) awssqs.Message { return awssqs.Message{Body: body, ReceiptHandle: "rh"} }

func TestHandleMessageDeletesOnSuccess(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{}
	handleMessage(context.Background(), discardLogger(), q, r,
		msg(`{"detail":{"repo_full_name":"acme/widget","ref":"refs/heads/main"}}`))

	if len(r.ran) != 1 || r.ran[0].RepoFullName != "acme/widget" || r.ran[0].Ref != "refs/heads/main" {
		t.Errorf("runner ran = %+v", r.ran)
	}
	if len(q.deleted) != 1 {
		t.Errorf("message should be deleted on success, deleted=%v", q.deleted)
	}
}

func TestHandleMessageRetainsOnRunError(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{err: errors.New("ollama down")}
	handleMessage(context.Background(), discardLogger(), q, r,
		msg(`{"detail":{"repo_full_name":"acme/widget"}}`))

	if len(r.ran) != 1 {
		t.Errorf("runner should have been called")
	}
	if len(q.deleted) != 0 {
		t.Errorf("message must NOT be deleted on failure (SQS retry), deleted=%v", q.deleted)
	}
}

func TestHandleMessageDropsUnparseable(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{}
	handleMessage(context.Background(), discardLogger(), q, r, msg(`not json`))
	if len(r.ran) != 0 {
		t.Error("runner should not run for an unparseable message")
	}
	if len(q.deleted) != 1 {
		t.Error("unparseable message should be dropped (deleted)")
	}
}

func TestHandleMessageDropsEmptyDetail(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{}
	handleMessage(context.Background(), discardLogger(), q, r, msg(`{"detail":{}}`))
	if len(r.ran) != 0 || len(q.deleted) != 1 {
		t.Errorf("empty-detail message should be dropped without running: ran=%d deleted=%d", len(r.ran), len(q.deleted))
	}
}
