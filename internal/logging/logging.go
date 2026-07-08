// Package logging provides the platform's structured, JSON-based logger built
// on the standard library's log/slog. A single factory keeps log format and
// level handling consistent across every Lambda and enforces the project's
// security rule: secrets (GitHub PATs, webhook secrets) are never logged.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// redacted is the placeholder emitted in place of secret values.
const redacted = "[REDACTED]"

// New returns a JSON structured logger writing to w at the given level.
// Unrecognised levels fall back to info. Passing nil for w defaults to
// discarding output, which is convenient in tests.
func New(level string, w io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(handler)
}

// WithRun returns a child logger tagged with a run/correlation ID so that all
// records for a single pipeline execution can be traced across stages.
func WithRun(l *slog.Logger, runID string) *slog.Logger {
	return l.With(slog.String("run_id", runID))
}

// Redact returns a value safe to log: non-empty secrets become a fixed
// placeholder, empty values are reported as "[EMPTY]". Use this whenever a
// value derived from a secret must appear in a log for debugging.
func Redact(secret string) string {
	if secret == "" {
		return "[EMPTY]"
	}
	return redacted
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
