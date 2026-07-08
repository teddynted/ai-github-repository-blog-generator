package lifecycle

import (
	"context"
	"errors"
	"testing"
)

type fakeEC2 struct {
	id, state string
	findErr   error
	startErr  error
	started   string
}

func (f *fakeEC2) FindInstance(_ context.Context, _ string) (string, string, error) {
	return f.id, f.state, f.findErr
}
func (f *fakeEC2) StartInstance(_ context.Context, id string) error {
	f.started = id
	return f.startErr
}

func TestEnsureRunningStartsStoppedInstance(t *testing.T) {
	f := &fakeEC2{id: "i-123", state: "stopped"}
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
		f := &fakeEC2{id: "i-1", state: state}
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
	f := &fakeEC2{id: "", state: ""}
	if _, err := (&Starter{EC2: f, Project: "blog-gen"}).EnsureRunning(context.Background()); err == nil {
		t.Error("expected error when no tagged instance exists")
	}
}

func TestEnsureRunningPropagatesErrors(t *testing.T) {
	f := &fakeEC2{findErr: errors.New("api down")}
	if _, err := (&Starter{EC2: f, Project: "p"}).EnsureRunning(context.Background()); err == nil {
		t.Error("expected find error")
	}
	f2 := &fakeEC2{id: "i-1", state: "stopped", startErr: errors.New("capacity")}
	if _, err := (&Starter{EC2: f2, Project: "p"}).EnsureRunning(context.Background()); err == nil {
		t.Error("expected start error")
	}
}
