// Package archspec generates the AWS Architecture Diagram Specification — a
// structured, repository-grounded Markdown document describing the architecture
// a repository actually implements, suitable for a downstream renderer to turn
// into a production-quality AWS architecture diagram (SVG/PNG).
//
// It is deliberately NOT a generic AWS diagram and never a renderer: the model
// is given only the architecture evidence the Release Context already captured
// (CloudFormation resources, detected AWS services, architecture components,
// event-driven flows, and Mermaid diagrams) and is told to describe ONLY what
// that evidence supports. When a repository has no groundable AWS evidence the
// generator returns ErrNoEvidence so the orchestrator can skip the artifact
// rather than invent one.
//
// The output is a separate artifact (architecture-diagram-spec.md); it is not
// concatenated into the blog, so the video/SEO generators that parse blog.md
// are unaffected.
package archspec

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// ErrNoEvidence signals that the Release Context has no groundable AWS evidence
// (no CloudFormation resources and no detected AWS services), so no honest
// architecture specification can be produced. Callers treat it as a graceful
// skip, never a failure.
var ErrNoEvidence = errors.New("archspec: release context has no groundable AWS architecture evidence")

// specHeading is the mandatory top-level heading of the artifact.
const specHeading = "# AWS Architecture Diagram Specification"

// DiagramVersion is the spec's schema version, stamped into the metadata so a
// downstream renderer can key off it (SemVer, additive-only).
const DiagramVersion = "1.0.0"

// DefaultMaxPromptBytes bounds the evidence block so the prompt fits the model's
// context window.
const DefaultMaxPromptBytes = 16000

// Model is the inference port (matches the platform-wide generate contract).
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Generator produces the AWS Architecture Diagram Specification.
type Generator struct {
	Model          Model
	MaxPromptBytes int
	Logger         *slog.Logger
}

// Spec is the produced artifact.
type Spec struct {
	Title      string `json:"title"`
	Repository string `json:"repository"`
	Release    string `json:"release"`
	// Body is the complete specification Markdown, beginning with specHeading.
	Body string `json:"-"`
}

// Markdown returns the artifact's Markdown (satisfies the suite's stage contract).
func (s Spec) Markdown() string { return s.Body }

// Spec generates the specification from the Release Context, focused on the
// engineering topic the article covers (via the blog title). It returns
// ErrNoEvidence when the repository has nothing groundable to diagram.
func (g *Generator) Spec(ctx context.Context, pkg ReleasePackage) (Spec, error) {
	if pkg.Context == nil {
		return Spec{}, errors.New("archspec: nil release context")
	}
	if !HasEvidence(pkg.Context) {
		return Spec{}, ErrNoEvidence
	}

	// With a model (routed to Claude), Claude designs the specification from the
	// evidence. Without one (offline/degraded), fall back to a deterministic
	// assembly of the same evidence — still fully grounded, never invented.
	var body string
	if g.Model == nil {
		body = assembleOffline(pkg)
	} else {
		raw, err := g.Model.Generate(ctx, g.prompt(pkg))
		if err != nil {
			return Spec{}, fmt.Errorf("archspec: generate: %w", err)
		}
		body = ensureHeading(strings.TrimSpace(raw))
	}

	spec := Spec{
		Title:      specTitle(pkg),
		Repository: pkg.Context.Repository.FullName,
		Release:    pkg.Context.Release.Tag,
		Body:       body,
	}
	if g.Logger != nil {
		g.Logger.Info("aws architecture diagram spec generated",
			slog.String("repository", spec.Repository),
			slog.String("release", spec.Release),
			slog.Int("cfnResources", len(pkg.Context.CloudFormation.Resources)),
			slog.Int("awsServices", len(evidenceServices(pkg.Context, pkg.Blog.Markdown))),
			slog.Int("bytes", len(spec.Body)),
		)
	}
	return spec, nil
}

// HasEvidence reports whether the context carries AWS architecture evidence
// strong enough to ground a specification. Exported so the orchestrator (and
// tests) can gate the stage the same way the generator does.
func HasEvidence(rctx *rc.ReleaseContext) bool {
	return len(rctx.CloudFormation.Resources) > 0 || len(rctx.Architecture.AWSServices) > 0
}

// ensureHeading guarantees the artifact starts with the mandatory H1, without
// duplicating it when the model already produced it.
func ensureHeading(body string) string {
	if body == "" {
		return specHeading
	}
	if strings.HasPrefix(body, specHeading) {
		return body
	}
	// The model may have led with a stray "# AWS Architecture..." variant or
	// prose; normalise to the canonical heading on top.
	return specHeading + "\n\n" + body
}

// specTitle is the deterministic fallback title, anchored to the article topic
// when available. The model produces the authoritative Title inside the
// metadata; this is only the struct's identity field.
func specTitle(pkg ReleasePackage) string {
	if t := releaseTopic(pkg.Blog.Title); t != "" {
		return t + " — AWS Architecture"
	}
	name := pkg.Context.Repository.Name
	if name == "" {
		name = pkg.Context.Repository.FullName
	}
	return name + " — AWS Architecture"
}
