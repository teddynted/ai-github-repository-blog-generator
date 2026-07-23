package mcp

import (
	"context"
	"log/slog"
	"time"
)

// SlogAudit writes audit entries as structured logs, standing in for a CloudWatch
// Logs adapter (which would batch these to a log group). It records the caller,
// server, operation, tool/resource, permission, outcome, duration, and any error
// — never a credential value. Swapping in the CloudWatch SDK requires no caller
// change (same AuditLogger port).
type SlogAudit struct {
	Logger *slog.Logger
}

func (a SlogAudit) log() *slog.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return slog.Default()
}

func (a SlogAudit) Log(ctx context.Context, e AuditEntry) {
	attrs := []any{
		"caller", e.Caller,
		"server", e.Server,
		"operation", e.Operation,
		"outcome", e.Outcome,
		"durationMs", e.DurationMs,
	}
	if e.Tool != "" {
		attrs = append(attrs, "tool", e.Tool)
	}
	if e.Resource != "" {
		attrs = append(attrs, "resource", e.Resource)
	}
	if e.Permission != "" {
		attrs = append(attrs, "permission", string(e.Permission))
	}
	if e.Error != "" {
		attrs = append(attrs, "error", e.Error)
	}
	a.log().InfoContext(ctx, "mcp.audit", attrs...)
}

// NopAudit discards audit entries (used when auditing is disabled).
type NopAudit struct{}

func (NopAudit) Log(context.Context, AuditEntry) {}

// RecordingAudit keeps entries in memory (tests + the CLI's --audit view).
type RecordingAudit struct {
	Entries []AuditEntry
}

func (r *RecordingAudit) Log(_ context.Context, e AuditEntry) {
	r.Entries = append(r.Entries, e)
}

// auditOutcome derives the outcome label for an operation from its error.
func auditOutcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case isForbidden(err):
		return "denied"
	case isTimeout(err):
		return "timeout"
	default:
		return "error"
	}
}

func nowOr(clock Clock) time.Time {
	if clock != nil {
		return clock()
	}
	return time.Now()
}
