// Package localgen builds the model used by the local, per-artifact development
// CLIs (cmd/blog, cmd/storyboard, …). Local development uses the Anthropic API
// (Claude) — there is no local LLM inference. cmd/content additionally supports
// Claude Code and Bedrock; these single-artifact tools default to Anthropic.
package localgen

import (
	"context"
	"errors"
	"os"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/anthropic"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// Default returns the local development model — the Anthropic API. When
// ANTHROPIC_API_KEY is unset (or the client fails to init), it returns a model
// whose Generate reports the misconfiguration, so callers need no extra error
// handling at wiring time.
func Default(model string) releasegen.Model {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return errModel{errors.New("ANTHROPIC_API_KEY is not set — local development uses the Anthropic API / Claude Code")}
	}
	if model == "" {
		model = anthropic.DefaultModel
	}
	m, err := anthropic.New(anthropic.Config{APIKey: key, Model: model})
	if err != nil {
		return errModel{err}
	}
	return m
}

type errModel struct{ err error }

func (e errModel) Generate(context.Context, string) (string, error) { return "", e.err }
