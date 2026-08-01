// Package idle decides whether an on-demand EC2 host has been idle long enough
// to stop. It combines CloudWatch metrics (CPU, network) with application-level
// activity probes (n8n) and a sustained-idle streak, so the box is
// stopped because nothing is happening — not because a clock struck a number.
//
// The decision logic is pure and injected with small interfaces (Metrics,
// Probe, a clock), so it is unit-testable without touching AWS or the network.
// The caller performs the side effects the Decision asks for: mutating the
// idle-streak tag and issuing the stop.
package idle

import (
	"context"
	"fmt"
	"time"
)

// Action is the outcome of an evaluation.
type Action string

const (
	// ActionSkip means the instance is not eligible for a stop right now
	// (not running, within startup grace, or a KEEP_RUNNING override is set).
	ActionSkip Action = "skip"
	// ActionKeep means the instance is eligible but must keep running — it is
	// active, or the idle streak has not yet reached the configured duration.
	ActionKeep Action = "keep"
	// ActionStop means the instance has been idle long enough and should stop.
	ActionStop Action = "stop"
)

// Instance is the current snapshot of the EC2 instance under evaluation.
type Instance struct {
	State       string    // running|pending|stopped|...
	LaunchTime  time.Time // when the instance most recently started
	KeepRunning bool      // the KEEP_RUNNING tag is set to "true"
	IdleSince   time.Time // value of the IdleSince tag; zero when unset
}

// Metrics supplies CloudWatch datapoints over a trailing window. Each slice is
// one value per period; an empty slice means "no data" (treated as NOT idle, so
// missing metrics never cause a stop).
type Metrics interface {
	// CPUAverages returns per-period Average CPUUtilization (percent).
	CPUAverages(ctx context.Context, start, end time.Time) ([]float64, error)
	// NetworkTotals returns per-period NetworkIn+NetworkOut (bytes).
	NetworkTotals(ctx context.Context, start, end time.Time) ([]float64, error)
}

// Probe reports whether an application is currently busy. Implementations MUST
// fail safe: on error they return busy=true, so a transient network blip can
// never trigger a wrongful stop of live work.
type Probe interface {
	Busy(ctx context.Context) bool
	Name() string
}

// Config holds the idle thresholds and durations.
type Config struct {
	CPUThreshold float64       // percent; every datapoint must be below this
	NetThreshold float64       // bytes per period; every datapoint must be below this
	IdleDuration time.Duration // how long idleness must persist before stopping
	StartupGrace time.Duration // never stop within this long of LaunchTime
	EvalWindow   time.Duration // trailing metric window inspected each run
}

// Signals is the raw evidence behind a Decision, for structured logging.
type Signals struct {
	CPUIdle   bool
	NetIdle   bool
	BusyProbe string // name of the first probe reporting busy; "" when none
	CPU       []float64
	Network   []float64
}

// Decision is the evaluation outcome plus the idle-streak tag mutations the
// caller must apply.
type Decision struct {
	Action    Action
	Reason    string
	IdleFor   time.Duration // how long the streak has lasted (ActionStop/keep)
	MarkIdle  bool          // set IdleSince = now
	ClearIdle bool          // delete the IdleSince tag
}

// Evaluator computes a Decision from an Instance snapshot.
type Evaluator struct {
	Cfg     Config
	Metrics Metrics
	Probes  []Probe
	Now     func() time.Time
}

func (e *Evaluator) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now().UTC()
}

// Evaluate returns the Decision and the Signals behind it. A non-nil error is
// only returned for a hard metrics failure (the caller should keep the instance
// running rather than stop blind).
func (e *Evaluator) Evaluate(ctx context.Context, inst Instance) (Decision, Signals, error) {
	now := e.now()

	if inst.State != "running" {
		return Decision{Action: ActionSkip, Reason: "state:" + inst.State}, Signals{}, nil
	}
	if inst.KeepRunning {
		// Clear any streak so the clock restarts once the override is removed.
		return Decision{Action: ActionSkip, Reason: "keep_running", ClearIdle: true}, Signals{}, nil
	}
	if now.Sub(inst.LaunchTime) < e.Cfg.StartupGrace {
		return Decision{Action: ActionSkip, Reason: "startup_grace"}, Signals{}, nil
	}

	start := now.Add(-e.Cfg.EvalWindow)
	cpu, err := e.Metrics.CPUAverages(ctx, start, now)
	if err != nil {
		return Decision{}, Signals{}, fmt.Errorf("cpu metrics: %w", err)
	}
	net, err := e.Metrics.NetworkTotals(ctx, start, now)
	if err != nil {
		return Decision{}, Signals{}, fmt.Errorf("network metrics: %w", err)
	}

	sig := Signals{
		CPUIdle: allBelow(cpu, e.Cfg.CPUThreshold),
		NetIdle: allBelow(net, e.Cfg.NetThreshold),
		CPU:     cpu,
		Network: net,
	}
	for _, p := range e.Probes {
		if p.Busy(ctx) {
			sig.BusyProbe = p.Name()
			break
		}
	}

	idle := sig.CPUIdle && sig.NetIdle && sig.BusyProbe == ""
	if !idle {
		reason := "active"
		if sig.BusyProbe != "" {
			reason = "busy:" + sig.BusyProbe
		}
		return Decision{Action: ActionKeep, Reason: reason, ClearIdle: true}, sig, nil
	}

	// Idle right now — the IdleSince tag tracks the streak across evaluations.
	if inst.IdleSince.IsZero() {
		return Decision{Action: ActionKeep, Reason: "streak_started", MarkIdle: true}, sig, nil
	}
	idleFor := now.Sub(inst.IdleSince)
	if idleFor < e.Cfg.IdleDuration {
		return Decision{Action: ActionKeep, Reason: "not_long_enough", IdleFor: idleFor}, sig, nil
	}
	return Decision{Action: ActionStop, Reason: "idle", IdleFor: idleFor, ClearIdle: true}, sig, nil
}

// allBelow reports whether vs is non-empty and every value is below threshold.
// An empty slice returns false: absent metrics are treated as "not idle" so a
// data gap never causes a stop.
func allBelow(vs []float64, threshold float64) bool {
	if len(vs) == 0 {
		return false
	}
	for _, v := range vs {
		if v >= threshold {
			return false
		}
	}
	return true
}
