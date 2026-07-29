package architecture

import (
	"fmt"
	"strings"
)

// repoLevelNotes is the version-independence statement rendered under the title.
const repoLevelNotes = "diagrams are generated from the repository README, documented AWS " +
	"integrations, automation workflows, and the current project structure. The architecture " +
	"document is intentionally **version-independent** so it can be reused across releases, " +
	"branches, and generated documentation workflows."

// repoLevelGenerationContext is the closing statement.
const repoLevelGenerationContext = "This document is generated from the **current repository state**, " +
	"including the README, documentation, workflows, and project structure available at generation " +
	"time. No release-specific version information is embedded in the architecture document."

// RepoLevelMarkdown renders the collection as a repository-level, version-independent
// architecture document: a platform overview, up to three curated diagram sections
// (logical data flow, deployment topology, repository component structure), and an
// architecture-intelligence table — with no release tag, date, or changelog. Every
// section is grounded in the same diagrams as Markdown(); missing diagrams are
// skipped rather than fabricated.
func (col ArchitectureCollection) RepoLevelMarkdown() string {
	var b strings.Builder
	ci := col.ContentIntelligence

	style := firstNonEmpty(ci.ArchitectureStyle, "Application")
	// Repository-level title uses the bare repo name (no owner prefix), so it reads
	// the same whether the context came from a fixture (owner/name) or the tree.
	repo := col.Metadata.Repository
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		repo = repo[i+1:]
	}
	fmt.Fprintf(&b, "# Architecture Diagrams: %s\n\n", repo)
	fmt.Fprintf(&b, "_Repository-level architecture overview · %s_\n\n", style)
	fmt.Fprintf(&b, "> **Notes:** %s\n\n", repoLevelNotes)

	if col.PlatformOverview != "" {
		fmt.Fprintf(&b, "---\n\n## Platform Overview\n\n%s\n\n", col.PlatformOverview)
	}

	used := map[int]bool{}
	deployment := col.pick(used, "High-Level Architecture", "Event-Driven Architecture")
	logical := col.pick(used, "Data Flow Diagram", "Event-Driven Architecture", "Sequence Diagram")
	component := col.pick(used, "Component Diagram")
	cicd := col.pick(used, "CI/CD Pipeline")

	logicalTitle := "Logical Architecture — Data Flow"
	if len(ci.LocalInference) > 0 && len(ci.CloudInference) > 0 {
		logicalTitle = "Logical Architecture — Hybrid AI Data Flow"
	}
	writeRepoSection(&b, deployment, "Deployment Architecture — AWS Integration", "Deployment architecture", "Deployment Diagram", "AWS Services Used", nil)
	writeRepoSection(&b, logical, logicalTitle, "Logical architecture", "Data Flow Diagram", "Key Components", nil)
	writeRepoSection(&b, component, "Repository Structure View", "Top-level package layout", "Component Diagram", "Repository Responsibilities", dirBullets(col.RepoDirectories))
	writeRepoSection(&b, cicd, "CI/CD & Infrastructure Automation", "Delivery and infrastructure automation", "Deployment Diagram", "Deployment Characteristics", nil)

	writeRepoIntelligence(&b, ci)

	fmt.Fprintf(&b, "---\n\n## Generation Context\n\n%s\n", repoLevelGenerationContext)
	return b.String()
}

// pick returns the first unused diagram whose Type matches one of prefs (in order
// of preference), marking it used. Returns nil when none match.
func (col ArchitectureCollection) pick(used map[int]bool, prefs ...string) *Diagram {
	for _, want := range prefs {
		for i := range col.Diagrams {
			if used[i] {
				continue
			}
			if col.Diagrams[i].Type == want {
				used[i] = true
				return &col.Diagrams[i]
			}
		}
	}
	return nil
}

// writeRepoSection renders one curated diagram section with a fixed title, its
// Mermaid, and a component bullet list. When bullets is non-nil it is used for the
// list verbatim; otherwise the diagram's node labels (or AWS services) are listed.
// It is a no-op when the diagram is absent.
func writeRepoSection(b *strings.Builder, d *Diagram, title, subtitle, typeLabel, componentsHeading string, bullets []string) {
	if d == nil {
		return
	}
	fmt.Fprintf(b, "---\n\n## %s\n\n_%s_\n\n", title, subtitle)
	fmt.Fprintf(b, "**Type:** %s · **Complexity:** %s\n\n", typeLabel, d.Metadata.Complexity)
	fmt.Fprintf(b, "```mermaid\n%s\n```\n\n", d.Mermaid)

	components := bullets
	preformatted := bullets != nil
	if components == nil {
		components = d.Nodes
		if len(components) == 0 {
			components = d.AWSServices
		}
	}
	if len(components) > 0 {
		fmt.Fprintf(b, "### %s\n\n", componentsHeading)
		for _, c := range components {
			if !preformatted {
				c = describeComponent(c) // add a one-line role when known
			}
			fmt.Fprintf(b, "- %s\n", c)
		}
		b.WriteString("\n")
	}
}

// componentDescriptions gives a one-sentence role for well-known services and
// platform components, used to turn a bare name list into descriptive bullets.
var componentDescriptions = map[string]string{
	"GitHub":                "source of webhooks, commits, and pull requests that trigger the pipeline",
	"Amazon EventBridge":    "asynchronous event ingestion and routing",
	"AWS Lambda":            "event handling and dispatch to the orchestration layer",
	"OpenClaw Orchestrator": "coordinates inference routing and artifact generation",
	"n8n Automation":        "runs the automation workflows",
	"n8n Workflow Engine":   "runs the automation workflows",
	"Ollama Runtime":        "local inference (primary backend)",
	"Amazon Bedrock":        "managed cloud inference (fallback backend)",
	"Amazon S3":             "durable artifact storage",
	"Amazon EFS":            "shared workspace and workflow state",
	"Amazon CloudWatch":     "centralized logs and metrics",
	"AWS IAM":               "identity and access management",
	"AWS IAM Role":          "assumed by the workflow to grant scoped, least-privilege access",
	"CI/CD Workflow":        "runs validation and build steps for each change",
	"Deployment Artifacts":  "build outputs published for downstream infrastructure automation",
	"AWS CloudFormation":    "provisions infrastructure as code",
	"Amazon EC2":            "on-demand compute for workloads that are not serverless",
	"Amazon API Gateway":    "managed API entry point",
	"Amazon DynamoDB":       "managed NoSQL data store",
	"Amazon SQS":            "message queue buffering asynchronous work",
	"Amazon SNS":            "pub/sub notification fan-out",
	"AWS Secrets Manager":   "secure storage for credentials and secrets",
	"AWS Systems Manager":   "operational configuration and parameter storage",
	"AWS KMS":               "encryption key management",
}

// describeComponent formats a bullet as "**Name** — role" when a role is known,
// otherwise the bare name (e.g. for unrecognised or abbreviated diagram nodes).
func describeComponent(label string) string {
	if d := componentDescriptions[label]; d != "" {
		return "**" + label + "** — " + d
	}
	return label
}

// dirBullets formats directory responsibilities as "**path** — responsibility"
// bullets (path only when no responsibility is grounded). Returns nil when there
// are none, so the section falls back to the diagram's node labels.
func dirBullets(dirs []DirectoryResponsibility) []string {
	if len(dirs) == 0 {
		return nil
	}
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d.Responsibility != "" {
			out = append(out, "**"+d.Path+"** — "+d.Responsibility)
		} else {
			out = append(out, "**"+d.Path+"**")
		}
	}
	return out
}

// writeRepoIntelligence renders the Architecture Intelligence table. Rows appear
// only when grounded — inference rows are omitted for non-AI repositories.
func writeRepoIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Architecture Intelligence\n\n")
	b.WriteString("| Attribute | Value |\n| --- | --- |\n")
	row := func(attr, val string) {
		if strings.TrimSpace(val) != "" {
			fmt.Fprintf(b, "| **%s** | %s |\n", attr, val)
		}
	}
	row("Architecture style", ci.ArchitectureStyle)
	row("Primary workflow", ci.PrimaryWorkflow)
	row("Infrastructure complexity", ci.InfrastructureComplexity)
	row("Local inference", strings.Join(ci.LocalInference, ", "))
	row("Cloud inference", strings.Join(ci.CloudInference, ", "))
	row("Operational model", ci.OperationalModel)
	// Grounded service-role rows — only when the service is actually present.
	row("Shared workspace", ifPresent(ci.CloudServices, "Amazon EFS"))
	row("Durable artifacts", ifPresent(ci.CloudServices, "Amazon S3"))
	row("Centralized observability", strings.Join(ci.ObservabilityComponents, ", "))
	row("Estimated reading time", ci.EstimatedReadingTime)
	b.WriteString("\n")
}

// ifPresent returns want if it is in the list, otherwise "" (so the caller omits
// the row rather than asserting a service that is not grounded).
func ifPresent(list []string, want string) string {
	for _, s := range list {
		if s == want {
			return want
		}
	}
	return ""
}
