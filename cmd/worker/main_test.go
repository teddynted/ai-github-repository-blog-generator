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
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasepipeline"
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

type fakeMeter struct{ counts map[string]float64 }

func (f *fakeMeter) Count(name string)             { f.add(name, 1) }
func (f *fakeMeter) CountN(name string, n float64) { f.add(name, n) }
func (f *fakeMeter) add(name string, n float64) {
	if f.counts == nil {
		f.counts = map[string]float64{}
	}
	f.counts[name] += n
}

func discardLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func msg(body string) awssqs.Message { return awssqs.Message{Body: body, ReceiptHandle: "rh"} }

func TestHandleMessageDeletesAndNotifiesOnSuccess(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{res: pipeline.Result{Published: 5}}
	n := &fakeNotifier{}
	mt := &fakeMeter{}
	handleMessage(context.Background(), discardLogger(), q, r, nil, n, mt,
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
	if mt.counts["RunsStarted"] != 1 || mt.counts["RunsSucceeded"] != 1 || mt.counts["AssetsGenerated"] != 5 {
		t.Errorf("metrics = %v", mt.counts)
	}
}

func TestHandleMessageNotifiesHeld(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{res: pipeline.Result{Held: true}}
	n := &fakeNotifier{}
	mt := &fakeMeter{}
	handleMessage(context.Background(), discardLogger(), q, r, nil, n, mt,
		msg(`{"detail":{"repo_full_name":"acme/widget"}}`))
	if len(q.deleted) != 1 {
		t.Error("held run is a terminal outcome; message should be deleted")
	}
	if len(n.events) != 1 || n.events[0].Status != notify.StatusHeld {
		t.Errorf("expected held notification: %+v", n.events)
	}
	if mt.counts["RunsHeld"] != 1 || mt.counts["RunsSucceeded"] != 0 {
		t.Errorf("metrics = %v", mt.counts)
	}
}

func TestHandleMessageRetainsAndNotifiesOnRunError(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{err: errors.New("ollama down")}
	n := &fakeNotifier{}
	mt := &fakeMeter{}
	handleMessage(context.Background(), discardLogger(), q, r, nil, n, mt,
		msg(`{"detail":{"repo_full_name":"acme/widget"}}`))

	if len(q.deleted) != 0 {
		t.Errorf("message must NOT be deleted on failure (SQS retry), deleted=%v", q.deleted)
	}
	if len(n.events) != 1 || n.events[0].Status != notify.StatusFailed || n.events[0].Err == "" {
		t.Errorf("expected failed notification: %+v", n.events)
	}
	if mt.counts["RunsFailed"] != 1 {
		t.Errorf("metrics = %v", mt.counts)
	}
}

type fakeReleaseRunner struct {
	ran []rc.Request
	res releasepipeline.Result
	err error
}

func (f *fakeReleaseRunner) Run(_ context.Context, req rc.Request) (releasepipeline.Result, error) {
	f.ran = append(f.ran, req)
	return f.res, f.err
}

// A published-release event routes to the release-content pipeline, not the
// snapshot runner.
func TestHandleMessageRoutesReleaseEvent(t *testing.T) {
	q := &fakeConsumer{}
	snapshot := &fakeRunner{}
	rr := &fakeReleaseRunner{res: releasepipeline.Result{Published: 3}}
	n := &fakeNotifier{}
	mt := &fakeMeter{}
	body := `{"detail":{"repo_full_name":"acme/widget","owner":"acme","name":"widget","ref":"refs/tags/v0.2.0","source":"release"}}`
	handleMessage(context.Background(), discardLogger(), q, snapshot, rr, n, mt, msg(body))

	if len(snapshot.ran) != 0 {
		t.Error("snapshot runner must not run for a release event")
	}
	if len(rr.ran) != 1 || rr.ran[0].Owner != "acme" || rr.ran[0].Repository != "widget" || rr.ran[0].ReleaseTag != "v0.2.0" {
		t.Errorf("release runner ran = %+v", rr.ran)
	}
	if len(q.deleted) != 1 {
		t.Error("release message should be deleted on success")
	}
	if len(n.events) != 1 || n.events[0].Status != notify.StatusPublished || n.events[0].Assets != 3 {
		t.Errorf("expected published notification: %+v", n.events)
	}
	if mt.counts["ReleaseRunsSucceeded"] != 1 || mt.counts["AssetsGenerated"] != 3 {
		t.Errorf("metrics = %v", mt.counts)
	}
}

// With no release runner configured, a release event falls through to the
// snapshot pipeline (back-compat).
func TestHandleMessageReleaseFallsBackWithoutRunner(t *testing.T) {
	q := &fakeConsumer{}
	snapshot := &fakeRunner{res: pipeline.Result{Published: 1}}
	body := `{"detail":{"repo_full_name":"acme/widget","owner":"acme","name":"widget","ref":"refs/tags/v0.2.0","source":"release"}}`
	handleMessage(context.Background(), discardLogger(), q, snapshot, nil, &fakeNotifier{}, &fakeMeter{}, msg(body))
	if len(snapshot.ran) != 1 {
		t.Error("without a release runner, a release event should use the snapshot pipeline")
	}
}

func TestHandleReleaseRunErrorRetains(t *testing.T) {
	q := &fakeConsumer{}
	rr := &fakeReleaseRunner{err: errors.New("github 502")}
	n := &fakeNotifier{}
	mt := &fakeMeter{}
	body := `{"detail":{"repo_full_name":"acme/widget","owner":"acme","name":"widget","ref":"refs/tags/v0.2.0","source":"release"}}`
	handleMessage(context.Background(), discardLogger(), q, &fakeRunner{}, rr, n, mt, msg(body))

	if len(q.deleted) != 0 {
		t.Error("failed release run must not delete the message (SQS retry)")
	}
	if len(n.events) != 1 || n.events[0].Status != notify.StatusFailed {
		t.Errorf("expected failed notification: %+v", n.events)
	}
	if mt.counts["ReleaseRunsFailed"] != 1 {
		t.Errorf("metrics = %v", mt.counts)
	}
}

func TestHandleReleaseMissingTagDrops(t *testing.T) {
	q := &fakeConsumer{}
	rr := &fakeReleaseRunner{}
	// source=release but no owner/name/ref -> cannot build a request; drop it.
	body := `{"detail":{"repo_full_name":"acme/widget","source":"release"}}`
	handleMessage(context.Background(), discardLogger(), q, &fakeRunner{}, rr, &fakeNotifier{}, &fakeMeter{}, msg(body))
	if len(rr.ran) != 0 {
		t.Error("release runner should not run without owner/name/tag")
	}
	if len(q.deleted) != 1 {
		t.Error("un-runnable release message should be dropped")
	}
}

func TestHandleMessageDropsUnparseable(t *testing.T) {
	q := &fakeConsumer{}
	r := &fakeRunner{}
	handleMessage(context.Background(), discardLogger(), q, r, nil, &fakeNotifier{}, &fakeMeter{}, msg(`not json`))
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
	handleMessage(context.Background(), discardLogger(), q, r, nil, &fakeNotifier{}, &fakeMeter{}, msg(`{"detail":{}}`))
	if len(r.ran) != 0 || len(q.deleted) != 1 {
		t.Errorf("empty-detail message should be dropped without running: ran=%d deleted=%d", len(r.ran), len(q.deleted))
	}
}
