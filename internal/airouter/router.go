// Package airouter is the AI Provider Router. Every AI request flows through an
// ordered provider chain and is served by the first provider that succeeds:
//
//   - Cloud:  AWS Bedrock (us.anthropic.claude-opus-4-8) primary, with automatic
//     fallback to the Anthropic API when Bedrock cannot fulfil the request
//     because of quota/throttling.
//   - Local:  Anthropic API (Claude Code) only.
//
// There is no local LLM inference. The router handles provider selection,
// fallback, and error handling, and emits structured logs on every call showing
// which provider was used, the model, whether a fallback occurred, and the reason.
package airouter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// Provider names.
const (
	ProviderBedrock   = "bedrock"
	ProviderAnthropic = "anthropic"
)

// Model is the inference port shared across the platform's generators
// (Generate(ctx, prompt) -> text). Every provider satisfies it.
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Provider is a registered provider in the chain: its name, the model id it
// serves (for logging/provenance), and the client.
type Provider struct {
	Name   string
	Model  string
	Client Model
}

// Router routes every AI request through the provider chain. The first entry is
// the primary (Bedrock in the cloud); later entries are fallbacks (Anthropic).
type Router struct {
	chain  []Provider
	logger *slog.Logger
}

// New builds a Router from an ordered provider chain (primary first).
func New(chain []Provider, logger *slog.Logger) *Router {
	return &Router{chain: chain, logger: logger}
}

// ModelFor returns the model that generates the given content kind. Routing is
// uniform — every kind uses the same Bedrock→Anthropic chain — so the kind is
// carried only for per-call logging.
func (r *Router) ModelFor(kind string) Model {
	return &chainModel{chain: r.chain, kind: canonical(kind), logger: r.logger}
}

// Primary returns the primary provider's name and model id — used for release
// provenance (the provider a run is expected to use).
func (r *Router) Primary() (name, model string) {
	if len(r.chain) == 0 {
		return "", ""
	}
	return r.chain[0].Name, r.chain[0].Model
}

// Chain returns the provider chain as "name:model" strings, in order, for
// start-up logging.
func (r *Router) Chain() []string {
	out := make([]string, 0, len(r.chain))
	for _, p := range r.chain {
		out = append(out, p.Name+":"+p.Model)
	}
	return out
}

// chainModel serves one request by trying each provider in order until one
// succeeds, logging the outcome.
type chainModel struct {
	chain  []Provider
	kind   string
	logger *slog.Logger
}

func (m *chainModel) Generate(ctx context.Context, prompt string) (string, error) {
	var errs []error
	var reason string // why we left the previous provider (for the fallback log)
	for i, p := range m.chain {
		out, err := p.Client.Generate(ctx, prompt)
		if err == nil {
			m.log(slog.LevelInfo, "ai request served", p, i > 0, reason, nil)
			return out, nil
		}
		reason = fallbackReason(err)
		m.log(slog.LevelWarn, "ai provider failed", p, i > 0, reason, err)
		errs = append(errs, fmt.Errorf("%s: %w", p.Name, err))
		// fall through to the next provider (if any)
	}
	return "", fmt.Errorf("airouter: all providers failed for %s: %w", m.kind, errors.Join(errs...))
}

func (m *chainModel) log(level slog.Level, msg string, p Provider, fallback bool, reason string, err error) {
	if m.logger == nil {
		return
	}
	attrs := []any{
		slog.String("content_type", m.kind),
		slog.String("provider", p.Name),
		slog.String("model", p.Model),
		slog.Bool("fallback", fallback),
	}
	if reason != "" {
		attrs = append(attrs, slog.String("reason", reason))
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	m.logger.Log(context.Background(), level, msg, attrs...)
}

// fallbackReason classifies a provider error for the structured log: "quota"
// when the request cannot be fulfilled because of quota/throttling/capacity
// limits (the case the router is designed to fall back on), otherwise "error".
func fallbackReason(err error) string {
	s := strings.ToLower(err.Error())
	for _, marker := range []string{
		"throttl", "quota", "servicequotaexceeded", "too many requests",
		"rate exceeded", "rate limit", "capacity", "429",
	} {
		if strings.Contains(s, marker) {
			return "quota"
		}
	}
	return "error"
}
