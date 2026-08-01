// Package modelfallback makes the content pipeline resilient to a writer model
// that is configured but unavailable at call time. It wraps a primary model
// (e.g. Claude on Bedrock) and a secondary (e.g. the model via the provider router): each
// Generate tries the primary and, on error, falls back to the secondary.
//
// This closes a real gap: a model client can construct successfully yet fail on
// every invocation (e.g. Bedrock returns "Operation not allowed" when model
// access is not granted). Without a per-call fallback, a single failed stage —
// the blog — is lost and cascades to everything that depends on it. With it, the
// run degrades to local generation instead of dropping content.
package modelfallback

import (
	"context"
	"log/slog"
)

// Model is the inference port shared across the platform's generators
// (Generate(ctx, prompt) -> text). Both the primary and secondary satisfy it.
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Fallback tries Primary first and falls back to Secondary on any error.
type Fallback struct {
	Primary   Model
	Secondary Model
	// Label names the primary for logs (e.g. "bedrock-claude"); optional.
	Label  string
	Logger *slog.Logger
}

// Generate runs the primary; on error it logs a warning and runs the secondary.
// A context cancellation from the primary is NOT retried on the secondary —
// that is the caller giving up, not the model being unavailable.
func (f *Fallback) Generate(ctx context.Context, prompt string) (string, error) {
	out, err := f.Primary.Generate(ctx, prompt)
	if err == nil {
		return out, nil
	}
	if ctx.Err() != nil || f.Secondary == nil {
		return "", err
	}
	if f.Logger != nil {
		f.Logger.Warn("primary model failed; falling back to local model",
			slog.String("primary", f.label()),
			slog.String("error", err.Error()))
	}
	return f.Secondary.Generate(ctx, prompt)
}

func (f *Fallback) label() string {
	if f.Label != "" {
		return f.Label
	}
	return "primary"
}
