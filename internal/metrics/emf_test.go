package metrics

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEmitProducesValidEMF(t *testing.T) {
	var buf bytes.Buffer
	e := New(Namespace, &buf)
	e.CountN("AssetsGenerated", 5)

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec); err != nil {
		t.Fatalf("EMF is not valid JSON: %v (%s)", err, buf.String())
	}
	if rec["AssetsGenerated"].(float64) != 5 {
		t.Errorf("metric value = %v", rec["AssetsGenerated"])
	}
	aws, ok := rec["_aws"].(map[string]any)
	if !ok {
		t.Fatalf("_aws block missing: %v", rec)
	}
	cwm := aws["CloudWatchMetrics"].([]any)[0].(map[string]any)
	if cwm["Namespace"] != Namespace {
		t.Errorf("namespace = %v", cwm["Namespace"])
	}
	def := cwm["Metrics"].([]any)[0].(map[string]any)
	if def["Name"] != "AssetsGenerated" || def["Unit"] != "Count" {
		t.Errorf("metric definition = %v", def)
	}
	if _, ok := aws["Timestamp"]; !ok {
		t.Error("Timestamp missing")
	}
}

func TestCountDefaultsToOne(t *testing.T) {
	var buf bytes.Buffer
	New(Namespace, &buf).Count("WebhookReceived")
	if !strings.Contains(buf.String(), `"WebhookReceived":1`) {
		t.Errorf("expected count of 1: %s", buf.String())
	}
}

func TestNilEmitterIsSafe(t *testing.T) {
	var e *Emitter
	e.Count("x")     // must not panic
	e.CountN("y", 3) // must not panic
	e.Emit(Metric{}) // must not panic
}

func TestEmitEmptyIsNoop(t *testing.T) {
	var buf bytes.Buffer
	New(Namespace, &buf).Emit()
	if buf.Len() != 0 {
		t.Errorf("empty emit should write nothing, got %q", buf.String())
	}
}
