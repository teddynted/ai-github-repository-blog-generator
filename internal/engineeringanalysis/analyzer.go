// Package engineeringanalysis is Stage 2 of the content pipeline: it drives a
// LOCAL model (Ollama) to read the factual ReleaseContext and EXTRACT a
// structured engineering analysis (releasecontext.EngineeringContext) — the
// "why" behind a release. It writes no prose.
//
// The analysis becomes the single source of truth handed to the Stage 3
// technical-writer (Claude). Splitting extraction from writing lets the cheap,
// deterministic work stay local while the paid, quality-critical writing is
// grounded in a clean, inspectable contract rather than a raw repository dump.
//
// The package depends only on a small Model port (satisfied by *ollama.Client
// today, any platform.LLMProvider via an adapter tomorrow), so it is decoupled
// from any specific model and fully unit-testable with a fake.
package engineeringanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// Model is the inference port. It matches the minimal generate contract used
// across the platform's generators, so *ollama.Client satisfies it directly.
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// DefaultMaxFactsBytes bounds the facts block so the extraction prompt fits a
// local model's context window.
const DefaultMaxFactsBytes = 16000

// Analyzer extracts a structured EngineeringContext from a ReleaseContext.
type Analyzer struct {
	Model Model
	// Fallback, when set, is retried once if the primary model returns an empty
	// analysis. The local model is cheap but sometimes produces no structured
	// output (e.g. a small model on a thin release); a stronger model then
	// salvages the analysis so the downstream writer has real material. Optional.
	Fallback      Model
	MaxFactsBytes int
	Logger        *slog.Logger
}

// Analyze runs the extraction. It returns the structured analysis; on a model
// or parse failure it returns an error and a nil analysis, so the caller can
// decide to proceed on the factual context alone (graceful degradation) rather
// than fail the whole run.
func (a *Analyzer) Analyze(ctx context.Context, rctx *rc.ReleaseContext) (*rc.EngineeringContext, error) {
	if a.Model == nil {
		return nil, fmt.Errorf("engineeringanalysis: no model configured")
	}
	if rctx == nil {
		return nil, fmt.Errorf("engineeringanalysis: nil release context")
	}

	prompt := a.buildPrompt(rctx)
	raw, err := a.Model.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("engineeringanalysis: model generate: %w", err)
	}

	ec, err := parse(raw)
	if err != nil {
		return nil, fmt.Errorf("engineeringanalysis: %w", err)
	}

	// If the primary model produced nothing usable, retry once on the fallback
	// model (a stronger/managed model) so the writer is not left grounding on an
	// empty analysis. A fallback error or a still-empty result is non-fatal — the
	// original (empty) analysis is kept and the run proceeds on the factual context.
	usedFallback := false
	if ec.IsZero() && a.Fallback != nil {
		if raw2, ferr := a.Fallback.Generate(ctx, prompt); ferr == nil {
			if ec2, perr := parse(raw2); perr == nil && !ec2.IsZero() {
				ec = ec2
				usedFallback = true
			}
		} else if a.Logger != nil {
			a.Logger.Warn("engineeringanalysis: fallback model failed", slog.String("error", ferr.Error()))
		}
	}

	// Stamp the release identity from the authoritative context, not the model,
	// so the analysis is always correctly attributed even if the model omits it.
	ec.Release = rc.ReleaseRef{Version: rctx.Release.Tag, Name: rctx.Release.Name}

	if a.Logger != nil {
		a.Logger.Info("engineering analysis extracted",
			slog.String("repository", rctx.Repository.FullName),
			slog.String("release", rctx.Release.Tag),
			slog.Int("decisions", len(ec.EngineeringDecisions)),
			slog.Int("tradeoffs", len(ec.Tradeoffs)),
			slog.Int("awsServices", len(ec.AWSServices)),
			slog.Bool("empty", ec.IsZero()),
			slog.Bool("usedFallback", usedFallback),
		)
	}
	return ec, nil
}

// parse extracts the JSON object from a model response (which may wrap it in
// prose or a ```json fence) and unmarshals it into an EngineeringContext.
func parse(raw string) (*rc.EngineeringContext, error) {
	blob := extractJSONObject(raw)
	if blob == "" {
		return nil, fmt.Errorf("no JSON object found in model output")
	}
	var ec rc.EngineeringContext
	if err := json.Unmarshal([]byte(blob), &ec); err != nil {
		return nil, fmt.Errorf("parse analysis JSON: %w", err)
	}
	return &ec, nil
}

// extractJSONObject returns the substring from the first '{' to its matching
// closing '}', ignoring braces inside JSON strings. This tolerates models that
// prepend prose or wrap the JSON in a Markdown code fence.
func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
