package power

import (
	"context"
	"errors"
	"testing"
)

// fakeInstances records the calls the Switch makes and returns scripted values.
type fakeInstances struct {
	state    string
	stateErr error
	startErr error
	stopErr  error
	started  []string
	stopped  []string
}

func (f *fakeInstances) InstanceState(_ context.Context, _ string) (string, error) {
	return f.state, f.stateErr
}
func (f *fakeInstances) StartInstance(_ context.Context, id string) error {
	f.started = append(f.started, id)
	return f.startErr
}
func (f *fakeInstances) StopInstance(_ context.Context, id string) error {
	f.stopped = append(f.stopped, id)
	return f.stopErr
}

func newSwitch(f *fakeInstances) *Switch {
	return &Switch{Instances: f, InstanceID: "i-123"}
}

func TestEnsureStartedStartsStopped(t *testing.T) {
	for _, state := range []string{stateStopped, stateStopping, stateShuttingDown} {
		f := &fakeInstances{state: state}
		changed, err := newSwitch(f).EnsureStarted(context.Background())
		if err != nil {
			t.Fatalf("state %s: %v", state, err)
		}
		if !changed || len(f.started) != 1 || f.started[0] != "i-123" {
			t.Errorf("state %s: expected start, changed=%v started=%v", state, changed, f.started)
		}
	}
}

func TestEnsureStartedNoopWhenRunning(t *testing.T) {
	for _, state := range []string{stateRunning, statePending} {
		f := &fakeInstances{state: state}
		changed, err := newSwitch(f).EnsureStarted(context.Background())
		if err != nil {
			t.Fatalf("state %s: %v", state, err)
		}
		if changed || len(f.started) != 0 {
			t.Errorf("state %s: expected no-op, changed=%v started=%v", state, changed, f.started)
		}
	}
}

func TestEnsureStoppedStopsRunning(t *testing.T) {
	for _, state := range []string{stateRunning, statePending} {
		f := &fakeInstances{state: state}
		changed, err := newSwitch(f).EnsureStopped(context.Background())
		if err != nil {
			t.Fatalf("state %s: %v", state, err)
		}
		if !changed || len(f.stopped) != 1 || f.stopped[0] != "i-123" {
			t.Errorf("state %s: expected stop, changed=%v stopped=%v", state, changed, f.stopped)
		}
	}
}

func TestEnsureStoppedNoopWhenStopped(t *testing.T) {
	for _, state := range []string{stateStopped, stateStopping, stateShuttingDown, stateTerminated} {
		f := &fakeInstances{state: state}
		changed, err := newSwitch(f).EnsureStopped(context.Background())
		if err != nil {
			t.Fatalf("state %s: %v", state, err)
		}
		if changed || len(f.stopped) != 0 {
			t.Errorf("state %s: expected no-op, changed=%v stopped=%v", state, changed, f.stopped)
		}
	}
}

func TestEnsureStartedRejectsTerminated(t *testing.T) {
	f := &fakeInstances{state: stateTerminated}
	if _, err := newSwitch(f).EnsureStarted(context.Background()); err == nil {
		t.Error("expected error starting a terminated instance")
	}
	if len(f.started) != 0 {
		t.Errorf("should not attempt start on terminated: %v", f.started)
	}
}

func TestErrorsWhenInstanceMissing(t *testing.T) {
	f := &fakeInstances{state: ""} // absent
	if _, err := newSwitch(f).EnsureStarted(context.Background()); err == nil {
		t.Error("EnsureStarted: expected error when instance is absent")
	}
	if _, err := newSwitch(f).EnsureStopped(context.Background()); err == nil {
		t.Error("EnsureStopped: expected error when instance is absent")
	}
}

func TestPropagatesAPIErrors(t *testing.T) {
	// Describe failure.
	f := &fakeInstances{stateErr: errors.New("api down")}
	if _, err := newSwitch(f).EnsureStarted(context.Background()); err == nil {
		t.Error("expected describe error to propagate")
	}
	// Start failure.
	f = &fakeInstances{state: stateStopped, startErr: errors.New("capacity")}
	if _, err := newSwitch(f).EnsureStarted(context.Background()); err == nil {
		t.Error("expected start error to propagate")
	}
	// Stop failure.
	f = &fakeInstances{state: stateRunning, stopErr: errors.New("throttled")}
	if _, err := newSwitch(f).EnsureStopped(context.Background()); err == nil {
		t.Error("expected stop error to propagate")
	}
}
