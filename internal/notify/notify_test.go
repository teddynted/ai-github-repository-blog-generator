package notify

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLogNotifierEmitsFields(t *testing.T) {
	var buf bytes.Buffer
	n := &LogNotifier{Logger: slog.New(slog.NewJSONHandler(&buf, nil))}

	if err := n.Notify(context.Background(), Event{Repo: "acme/widget", Status: StatusPublished, Assets: 3}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"acme/widget", "published", `"assets":3`} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestLogNotifierIncludesError(t *testing.T) {
	var buf bytes.Buffer
	n := &LogNotifier{Logger: slog.New(slog.NewJSONHandler(&buf, nil))}
	_ = n.Notify(context.Background(), Event{Repo: "a/b", Status: StatusFailed, Err: "boom"})
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("failed notification should include the error: %s", buf.String())
	}
}

func TestLogNotifierNilLoggerSafe(t *testing.T) {
	if err := (&LogNotifier{}).Notify(context.Background(), Event{}); err != nil {
		t.Errorf("nil logger should be safe: %v", err)
	}
}
