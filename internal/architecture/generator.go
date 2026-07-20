package architecture

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// Generator produces architecture diagrams from a ReleasePackage. It reuses the
// shared releasegen.Model port; when Model is nil, generation is fully
// deterministic (titles and descriptions are assembled from the grounded
// analysis). It never regenerates repository knowledge and never invents
// infrastructure.
type Generator struct {
	Model  releasegen.Model
	Now    func() time.Time
	Logger *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// Architecture converts a ReleasePackage into a collection of grounded
// architecture diagrams, each rendered as Mermaid, Graphviz DOT, and SVG with
// PNG export metadata. Analysis, diagram construction, rendering, metadata, and
// validation are deterministic; the Model only polishes descriptions. Every node
// and edge is grounded in the Release Context.
func (g *Generator) Architecture(ctx context.Context, pkg ReleasePackage) (ArchitectureCollection, error) {
	if pkg.Context == nil {
		return ArchitectureCollection{}, fmt.Errorf("architecture: release context is required")
	}

	a := analyze(pkg)
	specs := buildDiagrams(pkg, a)
	if len(specs) == 0 {
		return ArchitectureCollection{}, fmt.Errorf("architecture: the release context has no groundable infrastructure to diagram")
	}

	generatedAt := g.now().Format(time.RFC3339)
	collection := ArchitectureCollection{
		SchemaVersion: SchemaVersion,
		Metadata: Metadata{
			Repository:  repoFull(pkg),
			Release:     tag(pkg),
			GeneratedAt: generatedAt,
			SourceSchemas: map[string]string{
				"releaseContext": pkg.Context.SchemaVersion,
				"storyboard":     pkg.Storyboard.SchemaVersion,
			},
		},
	}

	diagrams := make([]Diagram, 0, len(specs))
	for i, spec := range specs {
		diagrams = append(diagrams, g.buildDiagram(ctx, pkg, spec, i, generatedAt))
	}

	collection.Diagrams = diagrams
	collection.Metadata.DiagramCount = len(diagrams)
	collection.ContentIntelligence = planIntelligence(pkg, a, diagrams)
	collection.Warnings = collectWarnings(a)

	if g.Logger != nil {
		g.Logger.Info("architecture diagrams generated",
			slog.String("repository", collection.Metadata.Repository),
			slog.String("release", collection.Metadata.Release),
			slog.Int("diagrams", len(diagrams)),
			slog.Int("warnings", len(collection.Warnings)),
		)
	}
	return collection, nil
}

// buildDiagram renders one diagram spec into all formats and metadata.
func (g *Generator) buildDiagram(ctx context.Context, pkg ReleasePackage, spec diagramSpec, index int, generatedAt string) Diagram {
	description := g.describe(ctx, spec)
	return Diagram{
		ID:          index + 1,
		Title:       spec.Title,
		Subtitle:    spec.Subtitle,
		Description: description,
		Type:        spec.Type,
		MermaidType: spec.MermaidType,
		Mermaid:     renderMermaid(spec.Graph, spec.MermaidType),
		Graphviz:    renderGraphviz(spec.Graph),
		SVG:         renderSVG(spec.Graph, spec.Title),
		PNG:         planPNG(spec.Graph),
		AWSServices: spec.Graph.awsServiceLabels(),
		References:  dedupe(spec.References),
		Metadata:    planDiagramMeta(pkg, spec, generatedAt),
	}
}

// describe optionally polishes the diagram description via the Model, grounded
// in the deterministic description (never adding infrastructure).
func (g *Generator) describe(ctx context.Context, spec diagramSpec) string {
	base := spec.Description
	if g.Model == nil || strings.TrimSpace(base) == "" {
		return base
	}
	out, err := g.Model.Generate(ctx, describePrompt(spec, base))
	if err != nil {
		return base
	}
	if r := collapse(strings.TrimSpace(out)); r != "" {
		return r
	}
	return base
}

func describePrompt(spec diagramSpec, base string) string {
	services := strings.Join(spec.Graph.awsServiceLabels(), ", ")
	return fmt.Sprintf(
		"Write a clear 1–2 sentence description for a %s architecture diagram titled %q. "+
			"The diagram shows exactly these AWS services: [%s]. Use ONLY those services and the notes below — "+
			"do NOT mention any other AWS service, resource, or relationship. Output only the description.\n\nNOTES: %s",
		spec.Type, spec.Title, services, base)
}

func collectWarnings(a analysis) []string {
	var w []string
	if len(a.Services) == 0 {
		w = append(w, "no AWS services in the release context; AWS-service diagrams were skipped")
	}
	if len(a.Mermaid) == 0 && len(a.Flows) == 0 {
		w = append(w, "no parsed Mermaid diagrams or event-driven flows; data-flow and sequence diagrams were skipped")
	}
	return w
}

func repoFull(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.FullName != "" {
		return pkg.Context.Repository.FullName
	}
	return repoShort(pkg)
}
