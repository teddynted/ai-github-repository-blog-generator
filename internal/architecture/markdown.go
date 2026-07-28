package architecture

import (
	"fmt"
	"strings"
)

// Markdown renders the collection as a production-ready document: one section
// per diagram with description, Mermaid (fenced so it renders), AWS services,
// Graphviz, and implementation notes, plus an architecture-intelligence summary.
func (col ArchitectureCollection) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Architecture Diagrams: %s %s\n\n", col.Metadata.Repository, col.Metadata.Release)
	fmt.Fprintf(&b, "_%d diagrams · %s architecture · confidence %d/100_\n\n",
		col.Metadata.DiagramCount, col.ContentIntelligence.ArchitectureStyle, col.ContentIntelligence.DiagramConfidence)

	if len(col.Warnings) > 0 {
		fmt.Fprintf(&b, "> **Notes:** %s\n\n", strings.Join(col.Warnings, "; "))
	}

	if col.PlatformOverview != "" {
		fmt.Fprintf(&b, "## Platform Overview\n\n%s\n\n", col.PlatformOverview)
	}

	for _, d := range col.Diagrams {
		writeDiagram(&b, d)
	}

	writeIntelligence(&b, col.ContentIntelligence)
	return b.String()
}

func writeDiagram(b *strings.Builder, d Diagram) {
	fmt.Fprintf(b, "---\n\n## %s\n\n", d.Title)
	if d.Subtitle != "" {
		fmt.Fprintf(b, "_%s_\n\n", d.Subtitle)
	}
	fmt.Fprintf(b, "**Type:** %s · **Complexity:** %s · **Confidence:** %d/100\n\n", d.Type, d.Metadata.Complexity, d.Metadata.Confidence)
	fmt.Fprintf(b, "%s\n\n", d.Description)

	fmt.Fprintf(b, "```mermaid\n%s\n```\n\n", d.Mermaid)

	if len(d.AWSServices) > 0 {
		fmt.Fprintf(b, "**AWS services:** %s\n\n", strings.Join(d.AWSServices, ", "))
	}
	if len(d.References) > 0 {
		fmt.Fprintf(b, "**Grounded in:** %s\n\n", strings.Join(d.References, ", "))
	}

	b.WriteString("<details><summary>Graphviz (DOT)</summary>\n\n```dot\n")
	b.WriteString(d.Graphviz)
	b.WriteString("```\n\n</details>\n\n")

	fmt.Fprintf(b, "**PNG export:** %s (%s), %d DPI, %s background\n\n",
		d.PNG.RecommendedResolution, d.PNG.AspectRatio, d.PNG.DPI, d.PNG.Background)
}

func writeIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Architecture Intelligence\n\n")
	fmt.Fprintf(b, "- **Style:** %s · **Deployment:** %s\n", ci.ArchitectureStyle, ci.DeploymentPattern)
	fmt.Fprintf(b, "- **Infrastructure complexity:** %s\n", ci.InfrastructureComplexity)
	if ci.PrimaryWorkflow != "" {
		fmt.Fprintf(b, "- **Primary workflow:** %s\n", ci.PrimaryWorkflow)
	}
	writeComp(b, "Local inference", ci.LocalInference)
	writeComp(b, "Cloud inference", ci.CloudInference)
	if ci.OperationalModel != "" {
		fmt.Fprintf(b, "- **Operational model:** %s\n", ci.OperationalModel)
	}
	writeComp(b, "Cloud services", ci.CloudServices)
	writeComp(b, "Compute", ci.ComputeComponents)
	writeComp(b, "Serverless", ci.ServerlessComponents)
	writeComp(b, "Storage", ci.StorageComponents)
	writeComp(b, "Database", ci.DatabaseComponents)
	writeComp(b, "Messaging", ci.MessagingComponents)
	writeComp(b, "Networking", ci.NetworkingComponents)
	writeComp(b, "Security", ci.SecurityComponents)
	writeComp(b, "Integration", ci.IntegrationServices)
	writeComp(b, "Observability", ci.ObservabilityComponents)
	fmt.Fprintf(b, "- **Estimated reading time:** %s · **Diagram confidence:** %d/100\n\n", ci.EstimatedReadingTime, ci.DiagramConfidence)
}

func writeComp(b *strings.Builder, label string, comps []string) {
	if len(comps) > 0 {
		fmt.Fprintf(b, "- **%s:** %s\n", label, strings.Join(comps, ", "))
	}
}
