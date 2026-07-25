// Package airouter is the Hybrid AI Routing layer. It picks which model
// generates each content artifact based on the artifact's importance, so
// high-value public-facing writing (blog, architecture, LinkedIn, X thread) can
// go to a premium model (Claude) while commodity, template-driven artifacts
// (SEO metadata, visual-asset prompts, short-form scripts) stay on the cheap
// local model (Ollama) — maximising quality where it matters and minimising
// inference cost where it does not.
//
// The router is provider-agnostic: providers are registered by name and rules
// map a content kind to a provider name, so adding a provider or changing the
// policy is configuration, not code. Every selection is wrapped in a per-call
// fallback, so an unavailable provider never fails a run — it degrades to the
// fallback provider.
package airouter

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/modelfallback"
)

// Model is the inference port shared across the platform's generators
// (Generate(ctx, prompt) -> text). Every provider satisfies it.
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Router selects a provider per content kind and wraps it with a fallback.
type Router struct {
	providers map[string]Model  // provider name -> model (e.g. "claude", "ollama")
	rules     map[string]string // content kind -> provider name
	def       string            // default provider when a kind has no rule
	fallback  string            // provider used when the selected one errors
	logger    *slog.Logger
}

// New builds a Router. Unknown or missing providers are tolerated: a rule that
// points at a provider not present falls through to the default, and the
// default itself falls through to any single registered provider, so the router
// is always usable even when only one provider is configured.
func New(providers map[string]Model, rules map[string]string, def, fallback string, logger *slog.Logger) *Router {
	r := &Router{
		providers: providers,
		rules:     normaliseRules(rules),
		def:       def,
		fallback:  fallback,
		logger:    logger,
	}
	if _, ok := r.providers[r.def]; !ok {
		r.def = anyProvider(providers, def)
	}
	if _, ok := r.providers[r.fallback]; !ok {
		r.fallback = r.def
	}
	return r
}

// ModelFor returns the model that should generate the given content kind,
// wrapped so a per-call failure falls back to the fallback provider. The
// decision is logged (content type -> provider) for observability.
func (r *Router) ModelFor(kind string) Model {
	name := r.providerName(kind)
	primary := r.providers[name]
	if primary == nil {
		name = r.def
		primary = r.providers[name]
	}
	secondary := r.providers[r.fallback]
	if r.logger != nil {
		r.logger.Info("ai routing decision",
			slog.String("content_type", kind),
			slog.String("provider", name),
			slog.String("fallback", r.fallback))
	}
	// When primary and fallback are the same provider there is nothing to fall
	// back to; return it directly to avoid a pointless double attempt.
	if secondary == nil || name == r.fallback {
		return primary
	}
	return &modelfallback.Fallback{Primary: primary, Secondary: secondary, Label: name, Logger: r.logger}
}

// providerName resolves the configured provider for a kind, defaulting when the
// kind has no rule.
func (r *Router) providerName(kind string) string {
	if p, ok := r.rules[canonical(kind)]; ok {
		if _, present := r.providers[p]; present {
			return p
		}
	}
	return r.def
}

// Decisions returns the resolved provider for each known kind — used for
// start-up logging and tests, so the effective policy is visible.
func (r *Router) Decisions(kinds []string) map[string]string {
	out := make(map[string]string, len(kinds))
	for _, k := range kinds {
		out[k] = r.providerName(k)
	}
	return out
}

// Providers returns the registered provider names, sorted (for logging).
func (r *Router) Providers() []string {
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func anyProvider(providers map[string]Model, prefer string) string {
	if _, ok := providers[prefer]; ok {
		return prefer
	}
	names := make([]string, 0, len(providers))
	for n := range providers {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > 0 {
		return names[0]
	}
	return prefer
}

func normaliseRules(rules map[string]string) map[string]string {
	out := make(map[string]string, len(rules))
	for k, v := range rules {
		out[canonical(k)] = strings.TrimSpace(strings.ToLower(v))
	}
	return out
}
