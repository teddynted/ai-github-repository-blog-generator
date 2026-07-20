package platform

import (
	"context"
	"fmt"
)

// Workflow capabilities.
const (
	CapOrchestrate Capability = "orchestrate"
	CapSchedule2   Capability = "workflow-schedule"
)

// WorkflowSpec describes a workflow to run. Steps are opaque names the engine
// resolves; the platform stays engine-agnostic (n8n, Temporal, Argo, Airflow,
// Step Functions, …).
type WorkflowSpec struct {
	Name  string
	Steps []string
	Input map[string]string
}

// RunHandle references a submitted workflow run.
type RunHandle struct {
	Provider string
	RunID    string
	Status   string // queued | running | succeeded | failed
}

// WorkflowEngine is the abstraction for orchestration engines. Current: n8n
// (external). Future: Temporal, Argo Workflows, Airflow, Step Functions.
type WorkflowEngine interface {
	Provider
	Submit(ctx context.Context, spec WorkflowSpec) (RunHandle, error)
	Status(ctx context.Context, runID string) (RunHandle, error)
}

// inProcessWorkflow is the reference workflow engine: it "runs" a spec
// synchronously and deterministically, demonstrating the submit/status contract.
type inProcessWorkflow struct {
	id   string
	runs map[string]RunHandle
}

func newInProcessWorkflow(id string) *inProcessWorkflow {
	return &inProcessWorkflow{id: id, runs: map[string]RunHandle{}}
}

func (w *inProcessWorkflow) ID() string { return w.id }
func (w *inProcessWorkflow) Kind() Kind { return KindWorkflow }
func (w *inProcessWorkflow) Capabilities() []Capability {
	return []Capability{CapOrchestrate, CapSchedule2}
}
func (w *inProcessWorkflow) Health(context.Context) Health { return OK() }

func (w *inProcessWorkflow) Submit(_ context.Context, spec WorkflowSpec) (RunHandle, error) {
	if spec.Name == "" || len(spec.Steps) == 0 {
		return RunHandle{}, fmt.Errorf("%s: workflow needs a name and at least one step", w.id)
	}
	h := RunHandle{Provider: w.id, RunID: "run-" + spec.Name, Status: "succeeded"}
	w.runs[h.RunID] = h
	return h, nil
}

func (w *inProcessWorkflow) Status(_ context.Context, runID string) (RunHandle, error) {
	if h, ok := w.runs[runID]; ok {
		return h, nil
	}
	return RunHandle{}, ErrNotFound
}

// NewInProcessWorkflow builds the reference workflow engine.
func NewInProcessWorkflow(id string) WorkflowEngine { return newInProcessWorkflow(id) }

func registerWorkflow(r *Registry) {
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "inprocess", Kind: KindWorkflow, Name: "In-process (reference)", Version: "1.0.0",
			Priority: 1, Description: "Synchronous reference workflow engine",
			Capabilities: []Capability{CapOrchestrate, CapSchedule2},
		},
		Factory: func(ConfigSource) (Provider, error) { return newInProcessWorkflow("inprocess"), nil },
	})
}
