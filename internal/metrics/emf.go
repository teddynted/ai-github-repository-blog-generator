// Package metrics emits CloudWatch custom metrics using the Embedded Metric
// Format (EMF): a structured JSON line written to stdout that CloudWatch Logs
// automatically extracts into metrics. This needs no PutMetricData API calls,
// no extra IAM, and adds no request latency.
package metrics

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Namespace is the platform's custom metric namespace.
const Namespace = "BlogGenerator"

// Metric is a single named measurement.
type Metric struct {
	Name  string
	Value float64
	Unit  string // e.g. "Count"; defaults to "Count" when empty
}

// Emitter writes EMF records. A nil *Emitter is a safe no-op.
type Emitter struct {
	namespace string
	w         io.Writer
	now       func() time.Time
	mu        sync.Mutex
}

// New returns an Emitter writing EMF records to w (typically os.Stdout, which
// Lambda captures into CloudWatch Logs).
func New(namespace string, w io.Writer) *Emitter {
	return &Emitter{namespace: namespace, w: w, now: time.Now}
}

// Count emits a single count metric of value 1.
func (e *Emitter) Count(name string) { e.CountN(name, 1) }

// CountN emits a single count metric with the given value.
func (e *Emitter) CountN(name string, value float64) {
	e.Emit(Metric{Name: name, Value: value, Unit: "Count"})
}

// Emit writes one EMF record containing the given metrics (no dimensions, to
// keep cardinality low). It never fails a caller: write errors are ignored.
func (e *Emitter) Emit(metrics ...Metric) {
	if e == nil || e.w == nil || len(metrics) == 0 {
		return
	}

	defs := make([]map[string]string, 0, len(metrics))
	rec := map[string]any{}
	for _, m := range metrics {
		unit := m.Unit
		if unit == "" {
			unit = "Count"
		}
		defs = append(defs, map[string]string{"Name": m.Name, "Unit": unit})
		rec[m.Name] = m.Value
	}
	rec["_aws"] = map[string]any{
		"Timestamp": e.now().UnixMilli(),
		"CloudWatchMetrics": []any{
			map[string]any{
				"Namespace":  e.namespace,
				"Dimensions": [][]string{{}},
				"Metrics":    defs,
			},
		},
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.w.Write(append(data, '\n'))
}
