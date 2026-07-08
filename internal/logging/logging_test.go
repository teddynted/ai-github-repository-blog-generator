package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewEmitsJSONAtLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New("warn", &buf)

	l.Info("suppressed") // below warn, should not appear
	l.Warn("kept", "key", "value")

	out := buf.String()
	if strings.Contains(out, "suppressed") {
		t.Errorf("info record should be filtered at warn level: %q", out)
	}
	if !strings.Contains(out, "kept") {
		t.Fatalf("warn record missing: %q", out)
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, out)
	}
	if rec["key"] != "value" {
		t.Errorf("structured attr missing: %v", rec)
	}
}

func TestWithRunTagsRecords(t *testing.T) {
	var buf bytes.Buffer
	l := WithRun(New("info", &buf), "run-123")
	l.Info("hello")

	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &rec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rec["run_id"] != "run-123" {
		t.Errorf("run_id not attached: %v", rec)
	}
}

func TestRedact(t *testing.T) {
	if got := Redact("github_pat_supersecret"); got != redacted {
		t.Errorf("Redact(secret) = %q, want %q", got, redacted)
	}
	if got := Redact(""); got != "[EMPTY]" {
		t.Errorf("Redact(empty) = %q, want [EMPTY]", got)
	}
	if strings.Contains(Redact("github_pat_supersecret"), "secret") {
		t.Error("Redact must not leak the secret value")
	}
}

func TestUnknownLevelDefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	l := New("verbose", &buf)
	l.Info("visible")
	if !strings.Contains(buf.String(), "visible") {
		t.Errorf("info should be emitted when level is unrecognised: %q", buf.String())
	}
}
