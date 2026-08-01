package idle

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeMetrics struct {
	cpu, net []float64
	err      error
}

func (f fakeMetrics) CPUAverages(context.Context, time.Time, time.Time) ([]float64, error) {
	return f.cpu, f.err
}
func (f fakeMetrics) NetworkTotals(context.Context, time.Time, time.Time) ([]float64, error) {
	return f.net, f.err
}

type fakeProbe struct {
	name string
	busy bool
}

func (p fakeProbe) Busy(context.Context) bool { return p.busy }
func (p fakeProbe) Name() string              { return p.name }

func evaluator(m Metrics, probes ...Probe) *Evaluator {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	return &Evaluator{
		Cfg: Config{
			CPUThreshold: 5,
			NetThreshold: 1 << 20, // 1 MB
			IdleDuration: 30 * time.Minute,
			StartupGrace: 15 * time.Minute,
			EvalWindow:   10 * time.Minute,
		},
		Metrics: m,
		Probes:  probes,
		Now:     func() time.Time { return now },
	}
}

// running instance launched an hour ago, idle streak started 31 min ago.
func idleInstance() Instance {
	return Instance{
		State:      "running",
		LaunchTime: time.Date(2026, 7, 28, 11, 0, 0, 0, time.UTC),
		IdleSince:  time.Date(2026, 7, 28, 11, 29, 0, 0, time.UTC),
	}
}

func TestSkipsWhenNotRunning(t *testing.T) {
	e := evaluator(fakeMetrics{})
	inst := idleInstance()
	inst.State = "stopped"
	d, _, err := e.Evaluate(context.Background(), inst)
	if err != nil || d.Action != ActionSkip {
		t.Fatalf("got %+v err=%v, want skip", d, err)
	}
}

func TestSkipsOnKeepRunningAndClearsStreak(t *testing.T) {
	e := evaluator(fakeMetrics{cpu: []float64{1}, net: []float64{1}})
	inst := idleInstance()
	inst.KeepRunning = true
	d, _, _ := e.Evaluate(context.Background(), inst)
	if d.Action != ActionSkip || d.Reason != "keep_running" || !d.ClearIdle {
		t.Fatalf("got %+v, want skip/keep_running/ClearIdle", d)
	}
}

func TestSkipsWithinStartupGrace(t *testing.T) {
	e := evaluator(fakeMetrics{cpu: []float64{1}, net: []float64{1}})
	inst := idleInstance()
	inst.LaunchTime = time.Date(2026, 7, 28, 11, 50, 0, 0, time.UTC) // 10 min ago < 15m grace
	d, _, _ := e.Evaluate(context.Background(), inst)
	if d.Action != ActionSkip || d.Reason != "startup_grace" {
		t.Fatalf("got %+v, want skip/startup_grace", d)
	}
}

func TestKeepsWhenCPUBusy(t *testing.T) {
	e := evaluator(fakeMetrics{cpu: []float64{2, 40, 1}, net: []float64{1}})
	d, sig, _ := e.Evaluate(context.Background(), idleInstance())
	if d.Action != ActionKeep || d.Reason != "active" || !d.ClearIdle || sig.CPUIdle {
		t.Fatalf("got %+v sig=%+v, want keep/active/ClearIdle", d, sig)
	}
}

func TestKeepsWhenProbeBusyEvenIfMetricsIdle(t *testing.T) {
	e := evaluator(fakeMetrics{cpu: []float64{1, 1}, net: []float64{10}},
		fakeProbe{name: "worker", busy: true})
	d, sig, _ := e.Evaluate(context.Background(), idleInstance())
	if d.Action != ActionKeep || d.Reason != "busy:worker" || sig.BusyProbe != "worker" {
		t.Fatalf("got %+v sig=%+v, want keep/busy:worker", d, sig)
	}
}

func TestStartsStreakWhenIdleWithNoTag(t *testing.T) {
	e := evaluator(fakeMetrics{cpu: []float64{1, 2}, net: []float64{10}},
		fakeProbe{name: "n8n", busy: false})
	inst := idleInstance()
	inst.IdleSince = time.Time{} // no streak yet
	d, _, _ := e.Evaluate(context.Background(), inst)
	if d.Action != ActionKeep || d.Reason != "streak_started" || !d.MarkIdle {
		t.Fatalf("got %+v, want keep/streak_started/MarkIdle", d)
	}
}

func TestKeepsWhenStreakTooShort(t *testing.T) {
	e := evaluator(fakeMetrics{cpu: []float64{1}, net: []float64{10}})
	inst := idleInstance()
	inst.IdleSince = time.Date(2026, 7, 28, 11, 45, 0, 0, time.UTC) // 15 min < 30m
	d, _, _ := e.Evaluate(context.Background(), inst)
	if d.Action != ActionKeep || d.Reason != "not_long_enough" {
		t.Fatalf("got %+v, want keep/not_long_enough", d)
	}
}

func TestStopsWhenIdleLongEnough(t *testing.T) {
	e := evaluator(fakeMetrics{cpu: []float64{1, 2, 0.5}, net: []float64{100, 200}},
		fakeProbe{name: "n8n", busy: false}, fakeProbe{name: "worker", busy: false})
	d, _, err := e.Evaluate(context.Background(), idleInstance())
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if d.Action != ActionStop || d.Reason != "idle" || !d.ClearIdle {
		t.Fatalf("got %+v, want stop/idle/ClearIdle", d)
	}
	if d.IdleFor < 30*time.Minute {
		t.Errorf("IdleFor = %v, want >= 30m", d.IdleFor)
	}
}

func TestEmptyMetricsAreNotIdle(t *testing.T) {
	// A CloudWatch data gap must never be read as idle.
	e := evaluator(fakeMetrics{cpu: nil, net: nil})
	d, sig, _ := e.Evaluate(context.Background(), idleInstance())
	if sig.CPUIdle || sig.NetIdle {
		t.Errorf("empty metrics should not be idle: %+v", sig)
	}
	if d.Action != ActionKeep {
		t.Fatalf("got %+v, want keep", d)
	}
}

func TestMetricsErrorIsFatal(t *testing.T) {
	e := evaluator(fakeMetrics{err: errors.New("throttled")})
	_, _, err := e.Evaluate(context.Background(), idleInstance())
	if err == nil {
		t.Fatal("expected error on metrics failure (must not stop blind)")
	}
}
