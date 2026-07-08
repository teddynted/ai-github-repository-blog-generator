package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeEC2 struct {
	inst     Instance
	findErr  error
	startErr error
	stopErr  error
	started  string
	stopped  string
}

func (f *fakeEC2) FindInstance(_ context.Context, _ string) (Instance, error) {
	return f.inst, f.findErr
}
func (f *fakeEC2) StartInstance(_ context.Context, id string) error {
	f.started = id
	return f.startErr
}
func (f *fakeEC2) StopInstance(_ context.Context, id string) error {
	f.stopped = id
	return f.stopErr
}

func TestEnsureRunningStartsStoppedInstance(t *testing.T) {
	f := &fakeEC2{inst: Instance{ID: "i-123", State: "stopped"}}
	started, err := (&Starter{EC2: f, Project: "blog-gen"}).EnsureRunning(context.Background())
	if err != nil {
		t.Fatalf("EnsureRunning: %v", err)
	}
	if !started || f.started != "i-123" {
		t.Errorf("expected start of i-123, started=%v id=%q", started, f.started)
	}
}

func TestEnsureRunningNoopWhenRunning(t *testing.T) {
	for _, state := range []string{"running", "pending"} {
		f := &fakeEC2{inst: Instance{ID: "i-1", State: state}}
		started, err := (&Starter{EC2: f, Project: "blog-gen"}).EnsureRunning(context.Background())
		if err != nil {
			t.Fatalf("state %s: %v", state, err)
		}
		if started || f.started != "" {
			t.Errorf("state %s: should not start", state)
		}
	}
}

func TestEnsureRunningErrorsWhenNoInstance(t *testing.T) {
	f := &fakeEC2{inst: Instance{}}
	if _, err := (&Starter{EC2: f, Project: "blog-gen"}).EnsureRunning(context.Background()); err == nil {
		t.Error("expected error when no tagged instance exists")
	}
}

func TestEnsureRunningPropagatesErrors(t *testing.T) {
	f := &fakeEC2{findErr: errors.New("api down")}
	if _, err := (&Starter{EC2: f, Project: "p"}).EnsureRunning(context.Background()); err == nil {
		t.Error("expected find error")
	}
	f2 := &fakeEC2{inst: Instance{ID: "i-1", State: "stopped"}, startErr: errors.New("capacity")}
	if _, err := (&Starter{EC2: f2, Project: "p"}).EnsureRunning(context.Background()); err == nil {
		t.Error("expected start error")
	}
}

// --- Shutdowner ---

type fakeQueue struct {
	depth int64
	err   error
}

func (q *fakeQueue) Depth(_ context.Context) (int64, error) { return q.depth, q.err }

func newShutdowner(f *fakeEC2, q *fakeQueue) *Shutdowner {
	return &Shutdowner{
		EC2:         f,
		Queue:       q,
		Project:     "blog-gen",
		IdleTimeout: 15 * time.Minute,
		Now:         func() time.Time { return time.Unix(1_000_000, 0) },
	}
}

func TestStopIfIdleStopsWhenIdle(t *testing.T) {
	// Running for 20 min, queue empty -> stop.
	launch := time.Unix(1_000_000, 0).Add(-20 * time.Minute)
	f := &fakeEC2{inst: Instance{ID: "i-1", State: "running", LaunchTime: launch}}
	stopped, err := newShutdowner(f, &fakeQueue{depth: 0}).StopIfIdle(context.Background())
	if err != nil {
		t.Fatalf("StopIfIdle: %v", err)
	}
	if !stopped || f.stopped != "i-1" {
		t.Errorf("expected stop of i-1, stopped=%v id=%q", stopped, f.stopped)
	}
}

func TestStopIfIdleWaitsForIdleWindow(t *testing.T) {
	// Running only 5 min (< 15m timeout) -> do not stop even if empty.
	launch := time.Unix(1_000_000, 0).Add(-5 * time.Minute)
	f := &fakeEC2{inst: Instance{ID: "i-1", State: "running", LaunchTime: launch}}
	stopped, err := newShutdowner(f, &fakeQueue{depth: 0}).StopIfIdle(context.Background())
	if err != nil || stopped || f.stopped != "" {
		t.Errorf("should not stop within idle window: stopped=%v err=%v", stopped, err)
	}
}

func TestStopIfIdleKeepsRunningWhenQueueBusy(t *testing.T) {
	launch := time.Unix(1_000_000, 0).Add(-30 * time.Minute)
	f := &fakeEC2{inst: Instance{ID: "i-1", State: "running", LaunchTime: launch}}
	stopped, err := newShutdowner(f, &fakeQueue{depth: 3}).StopIfIdle(context.Background())
	if err != nil || stopped || f.stopped != "" {
		t.Errorf("should not stop while queue has messages: stopped=%v err=%v", stopped, err)
	}
}

func TestStopIfIdleNoopWhenNotRunning(t *testing.T) {
	for _, state := range []string{"stopped", "stopping", "pending"} {
		f := &fakeEC2{inst: Instance{ID: "i-1", State: state}}
		stopped, err := newShutdowner(f, &fakeQueue{depth: 0}).StopIfIdle(context.Background())
		if err != nil || stopped {
			t.Errorf("state %s: should be no-op", state)
		}
	}
}

func TestStopIfIdleNoopWhenNoInstance(t *testing.T) {
	f := &fakeEC2{inst: Instance{}}
	stopped, err := newShutdowner(f, &fakeQueue{}).StopIfIdle(context.Background())
	if err != nil || stopped {
		t.Errorf("no instance: should be no-op, stopped=%v err=%v", stopped, err)
	}
}
