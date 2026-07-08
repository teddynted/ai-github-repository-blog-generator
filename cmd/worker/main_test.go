package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/awssqs"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/notify"
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
	res pipeline.Result
	err error
}

func (f *fakeRunner) Run(_ context.Context, req pipeline.Request) (pipeline.Result, error) {
	f.ran = append(f.ran, req)
	res := f.res
	res.RepoFullName = req.RepoFullName
	return res, f.err
}

type fakeNotifier struct{ events []notify.Event }

func (f *fakeNotifier) Notify(_ context.Context, e notify.Event) error {
	f.events = append(f.events, e)
	return nil
}

func discardLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func msg(body string) awssqs.Message { return awssqs.Message{Body: body, ReceiptHandle: "rh"} }

func TestHandleMessageDeletesAndNotifiesOnSuccess(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{res: pipeline.Result{Published: 5}}
	n := &fakeNotifier{}
	handleMessage(context.Background(), discardLogger(), q, r, n,
		msg(`{"detail":{"repo_full_name":"acme/widget","ref":"refs/heads/main"}}`))

	if len(r.ran) != 1 || r.ran[0].RepoFullName != "acme/widget" || r.ran[0].Ref != "refs/heads/main" {
		t.Errorf("runner ran = %+v", r.ran)
	}
	if len(q.deleted) != 1 {
		t.Errorf("message should be deleted on success, deleted=%v", q.deleted)
	}
	if len(n.events) != 1 || n.events[0].Status != notify.StatusPublished || n.events[0].Assets != 5 {
		t.Errorf("expected published notification: %+v", n.events)
	}
}

func TestHandleMessageNotifiesHeld(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{res: pipeline.Result{Held: true}}
	n := &fakeNotifier{}
	handleMessage(context.Background(), discardLogger(), q, r, n,
		msg(`{"detail":{"repo_full_name":"acme/widget"}}`))
	if len(q.deleted) != 1 {
		t.Error("held run is a terminal outcome; message should be deleted")
	}
	if len(n.events) != 1 || n.events[0].Status != notify.StatusHeld {
		t.Errorf("expected held notification: %+v", n.events)
	}
}

func TestHandleMessageRetainsAndNotifiesOnRunError(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{err: errors.New("ollama down")}
	n := &fakeNotifier{}
	handleMessage(context.Background(), discardLogger(), q, r, n,
		msg(`{"detail":{"repo_full_name":"acme/widget"}}`))

	if len(q.deleted) != 0 {
		t.Errorf("message must NOT be deleted on failure (SQS retry), deleted=%v", q.deleted)
	}
	if len(n.events) != 1 || n.events[0].Status != notify.StatusFailed || n.events[0].Err == "" {
		t.Errorf("expected failed notification: %+v", n.events)
	}
}

func TestHandleMessageDropsUnparseable(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{}
	n := &fakeNotifier{}
	handleMessage(context.Background(), discardLogger(), q, r, n, msg(`not json`))
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
	n := &fakeNotifier{}
	handleMessage(context.Background(), discardLogger(), q, r, n, msg(`{"detail":{}}`))
	if len(r.ran) != 0 || len(q.deleted) != 1 {
		t.Errorf("empty-detail message should be dropped without running: ran=%d deleted=%d", len(r.ran), len(q.deleted))
	}
}
