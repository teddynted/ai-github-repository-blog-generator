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

	// Stamp the deterministic provenance/contract sections that must be identical
	// on every artifact (LLM or offline path) rather than left to the model.
	body = decorate(body, pkg)

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

// graphContract is the fixed graph-definition contract stamped just before the
// Renderer Contract. It states, for automated validation tooling, that Components
// and Connections are the canonical node/edge sets and win over descriptive text.
const graphContract = `## Graph Definition Contract
- **Components** define the canonical node set.
- **Connections** define the canonical edge set.
- Renderers must not infer additional nodes or edges from descriptive text.
- **Operational Flow**, **Security**, **Failure Handling**, and **Rendering Notes** provide semantic annotations only.
- If descriptive text conflicts with the graph definition, **Components + Connections** take precedence.`

// rendererContract is the fixed contract appended to every spec, telling
// downstream renderers which sections are the authoritative graph and how to
// resolve conflicts. It is deterministic (never model-authored) so the guarantee
// is identical on every artifact.
const rendererContract = `## Renderer Contract
- This artifact is intended to be consumed directly by automated SVG / draw.io / Mermaid renderers.
- Renderers should treat the **Components** and **Connections** sections as the authoritative graph definition.
- **Operational Flow**, **Security**, and **Failure Handling** provide semantic annotations and must not introduce additional visual nodes unless explicitly declared in **Components**.
- If a conflict exists, **Components + Connections** take precedence over descriptive sections.`

// decorate stamps the deterministic, non-model-authored sections onto the spec:
// the Source Inputs / Generation Metadata provenance block (right after Diagram
// Metadata) and the Renderer Contract (at the end). Both are idempotent so a
// re-decorated body is unchanged.
func decorate(body string, pkg ReleasePackage) string {
	body = injectProvenance(body, pkg)
	body = appendGraphContract(body)
	body = appendRendererContract(body)
	return body
}

// appendGraphContract adds the fixed Graph Definition Contract to the end of the
// document, unless already present. decorate() runs it before
// appendRendererContract so the graph contract lands immediately above the
// Renderer Contract.
func appendGraphContract(body string) string {
	if strings.Contains(body, "## Graph Definition Contract") {
		return body
	}
	return strings.TrimRight(body, "\n") + "\n\n" + graphContract + "\n"
}

// injectProvenance inserts the Source Inputs and Generation Metadata sections
// immediately after the Diagram Metadata block, making the derived-IR provenance
// explicit and auditable. Source Inputs reflect the inputs actually used.
func injectProvenance(body string, pkg ReleasePackage) string {
	if strings.Contains(body, "## Generation Metadata") {
		return body
	}
	var b strings.Builder
	b.WriteString("## Source Inputs\n")
	b.WriteString("- blog.md\n")
	if strings.TrimSpace(pkg.ArchitectureDoc) != "" {
		b.WriteString("- architecture.md\n")
	}
	b.WriteString("\n## Generation Metadata\n")
	b.WriteString("- Artifact Generator: architecture-diagram-spec\n")
	b.WriteString("- Artifact Role: Derived intermediate representation (IR)\n")
	fmt.Fprintf(&b, "- Generated From Release: %s\n", firstNonEmpty(pkg.Context.Release.Tag, pkg.Context.Release.Name))
	b.WriteString("- Compatibility: Renderer-safe, non-authoritative source artifact")
	return insertAfterSection(body, "## Diagram Metadata", b.String())
}

// appendRendererContract adds the fixed Renderer Contract to the end of the
// document (after Rendering Notes, the trailing section), unless already present.
func appendRendererContract(body string) string {
	if strings.Contains(body, "## Renderer Contract") {
		return body
	}
	return strings.TrimRight(body, "\n") + "\n\n" + rendererContract + "\n"
}

// insertAfterSection returns body with block inserted immediately after the
// section that begins with heading (i.e. before the next "## " top-level heading,
// or at the end of the document if heading is the last section). When the heading
// is absent, block is inserted right after the H1 so provenance still appears near
// the top.
func insertAfterSection(body, heading, block string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == heading {
			start = i
			break
		}
	}
	insertAt := len(lines) // default: end of document
	if start >= 0 {
		for i := start + 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "## ") {
				insertAt = i
				break
			}
		}
	} else {
		// No Diagram Metadata heading — fall back to just after the H1.
		for i, ln := range lines {
			if strings.HasPrefix(ln, "# ") {
				insertAt = i + 1
				break
			}
		}
		if insertAt == len(lines) {
			insertAt = 0
		}
	}
	before := strings.TrimRight(strings.Join(lines[:insertAt], "\n"), "\n")
	after := strings.TrimLeft(strings.Join(lines[insertAt:], "\n"), "\n")
	parts := []string{before, block}
	if after != "" {
		parts = append(parts, after)
	}
	return strings.TrimRight(strings.Join(parts, "\n\n"), "\n") + "\n"
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
