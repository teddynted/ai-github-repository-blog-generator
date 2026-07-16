package intake

import (
	"context"
	"errors"
	"testing"
)

type fakePub struct {
	published []Event
	err       error
}

func (f *fakePub) Publish(_ context.Context, ev Event) error {
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

type fakeStarter struct {
	started bool
	err     error
}

func (s *fakeStarter) Start(_ context.Context) error { s.started = true; return s.err }

func sampleEvent() Event { return Event{RepoFullName: "acme/widget", Source: "manual"} }

func TestSubmitAcceptedWhenWindowOpen(t *testing.T) {
	pub := &fakePub{}
	s := &Service{Publisher: pub, Window: fakeWindow{open: true}}
	for _, policy := range []Policy{BufferOutsideWindow, RejectOutsideWindow} {
		d, err := s.Submit(context.Background(), sampleEvent(), policy)
		if err != nil || d != Accepted {
			t.Fatalf("policy %d: decision=%q err=%v, want accepted", policy, d, err)
		}
	}
	if len(pub.published) != 2 {
		t.Errorf("expected 2 published, got %d", len(pub.published))
	}
}

func TestSubmitDeferredWhenClosedAndBuffering(t *testing.T) {
	pub := &fakePub{}
	s := &Service{Publisher: pub, Window: fakeWindow{open: false}}
	d, err := s.Submit(context.Background(), sampleEvent(), BufferOutsideWindow)
	if err != nil || d != Deferred {
		t.Fatalf("decision=%q err=%v, want deferred", d, err)
	}
	if len(pub.published) != 1 {
		t.Errorf("deferred must still publish (buffer), got %d", len(pub.published))
	}
}

func TestSubmitRejectedWhenClosedAndRejecting(t *testing.T) {
	pub := &fakePub{}
	s := &Service{Publisher: pub, Window: fakeWindow{open: false}}
	d, err := s.Submit(context.Background(), sampleEvent(), RejectOutsideWindow)
	if err != nil || d != Rejected {
		t.Fatalf("decision=%q err=%v, want rejected", d, err)
	}
	if len(pub.published) != 0 {
		t.Errorf("rejected must NOT publish, got %d", len(pub.published))
	}
}

func TestSubmitStartsWhenClosedAndStarting(t *testing.T) {
	pub := &fakePub{}
	st := &fakeStarter{}
	s := &Service{Publisher: pub, Window: fakeWindow{open: false}, Starter: st}
	d, err := s.Submit(context.Background(), sampleEvent(), StartOutsideWindow)
	if err != nil || d != Started {
		t.Fatalf("decision=%q err=%v, want started", d, err)
	}
	if !st.started || len(pub.published) != 1 {
		t.Errorf("must start the host and publish: started=%v published=%d", st.started, len(pub.published))
	}
}

func TestSubmitStartFailureDefersNotDrops(t *testing.T) {
	pub := &fakePub{}
	st := &fakeStarter{err: errors.New("start failed")}
	s := &Service{Publisher: pub, Window: fakeWindow{open: false}, Starter: st}
	d, err := s.Submit(context.Background(), sampleEvent(), StartOutsideWindow)
	if err != nil || d != Deferred {
		t.Fatalf("decision=%q err=%v, want deferred (buffered)", d, err)
	}
	if len(pub.published) != 1 {
		t.Error("a start failure must still buffer the event")
	}
}

func TestSubmitFailsOpenOnWindowError(t *testing.T) {
	pub := &fakePub{}
	s := &Service{Publisher: pub, Window: fakeWindow{err: errors.New("describe failed")}}
	// Even the reject policy fails open so a transient describe error never drops work.
	d, err := s.Submit(context.Background(), sampleEvent(), RejectOutsideWindow)
	if err != nil || d != Accepted {
		t.Fatalf("decision=%q err=%v, want accepted (fail-open)", d, err)
	}
	if len(pub.published) != 1 {
		t.Errorf("fail-open must publish, got %d", len(pub.published))
	}
}

func TestSubmitNilWindowIsAlwaysOpen(t *testing.T) {
	pub := &fakePub{}
	s := &Service{Publisher: pub}
	d, err := s.Submit(context.Background(), sampleEvent(), RejectOutsideWindow)
	if err != nil || d != Accepted {
		t.Fatalf("decision=%q err=%v, want accepted", d, err)
	}
}

func TestSubmitPropagatesPublishError(t *testing.T) {
	s := &Service{Publisher: &fakePub{err: errors.New("put failed")}, Window: fakeWindow{open: true}}
	if _, err := s.Submit(context.Background(), sampleEvent(), BufferOutsideWindow); err == nil {
		t.Error("expected publish error to propagate")
	}
}

func TestRequestValidate(t *testing.T) {
	if err := (Request{Repository: "r", Owner: "o"}).Validate(); err != nil {
		t.Errorf("valid request rejected: %v", err)
	}
	for _, r := range []Request{{Owner: "o"}, {Repository: "r"}, {}} {
		if err := r.Validate(); err == nil {
			t.Errorf("expected validation error for %+v", r)
		}
	}
}

func TestRequestToEvent(t *testing.T) {
	ev := Request{Repository: "widget", Owner: "acme", Provider: "bedrock", Force: true}.ToEvent("manual")
	if ev.RepoFullName != "acme/widget" || ev.Owner != "acme" || ev.Name != "widget" {
		t.Errorf("identity fields wrong: %+v", ev)
	}
	if ev.Ref != "refs/heads/main" { // default branch
		t.Errorf("ref = %q, want refs/heads/main", ev.Ref)
	}
	if ev.Source != "manual" || ev.TriggerPattern != "manual" || ev.Provider != "bedrock" || !ev.Force {
		t.Errorf("trigger metadata wrong: %+v", ev)
	}

	ev2 := Request{Repository: "w", Owner: "o", Branch: "dev", Commit: "abc"}.ToEvent("manual")
	if ev2.Ref != "refs/heads/dev" || ev2.CommitSHA != "abc" {
		t.Errorf("branch/commit mapping wrong: %+v", ev2)
	}
}
