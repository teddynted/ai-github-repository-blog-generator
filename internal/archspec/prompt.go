package archspec

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// platformSuffixRe matches a trailing permanent-platform classification clause on
// a title/topic — "… for an Event-Driven AI Agent Platform", "… on a Hybrid AI
// platform" — so it can be stripped, keeping the topic scoped to the release.
var platformSuffixRe = regexp.MustCompile(`(?i)[\s,:]+(for|on|within|in|of)\s+an?\s+[^,.:]*\bplatform\b\s*$`)

// releaseTopic returns a title/topic scoped to the release, with any trailing
// permanent-platform classification removed.
func releaseTopic(s string) string {
	s = strings.TrimSpace(s)
	for {
		stripped := strings.TrimSpace(platformSuffixRe.ReplaceAllString(s, ""))
		if stripped == s {
			return strings.TrimRight(stripped, " :,-")
		}
		s = stripped
	}
}

// prompt builds the strictly-grounded specification prompt. It leads with a
// deterministic evidence block assembled from the Release Context (the source of
// truth), then constrains the model to that evidence and the exact output
// format, and finally forbids any rendering output.
func (g *Generator) prompt(pkg ReleasePackage) string {
	rctx := pkg.Context
	services := evidenceServices(rctx)

	var b strings.Builder
	b.WriteString("You are a senior AWS solutions architect producing an AWS Architecture Diagram Specification for a repository. ")
	b.WriteString("You are responsible for the engineering design of the diagram, NOT for rendering it. A downstream pipeline turns this specification into an AWS Architecture Icons SVG and PNG.\n\n")

	b.WriteString("The specification must describe the architecture THIS repository implements or documents for the engineering topic below — never a generic AWS diagram. ")
	b.WriteString("Use ONLY the repository evidence provided. Every AWS service you show must be backed by that evidence. If the repository does not implement or document a service, omit it. Do not speculate and do not invent services, resources, connections, or security controls.\n\n")

	if topic := releaseTopic(pkg.Blog.Title); topic != "" {
		fmt.Fprintf(&b, "Engineering topic (focus the diagram on this): %s\n\n", topic)
	}

	if len(services) > 0 {
		b.WriteString("You may reference ONLY these AWS services (each is supported by the evidence below); do not name any other AWS service:\n")
		b.WriteString("  " + strings.Join(services, ", ") + "\n\n")
	}

	b.WriteString(safeTruncate(evidenceBlock(pkg), g.maxPromptBytes()))
	b.WriteString("\n")

	// The exact output contract.
	b.WriteString("OUTPUT — return structured Markdown, and nothing else, in exactly this shape:\n\n")
	b.WriteString(specHeading + "\n\n")
	b.WriteString("## Diagram Metadata\n")
	b.WriteString("- Title: a specific title describing the architectural CHANGE this release introduces (not \"AWS Architecture\"). Do NOT append a permanent platform description or classification — e.g. write \"Pre-Baked Custom AMIs for Fast Spot Startup on AWS\", NOT \"… on an Event-Driven AI Agent Platform\".\n")
	b.WriteString("- Purpose: what the diagram communicates about this release's change.\n")
	b.WriteString("- Primary Engineering Topic: the release's engineering focus in one sentence — NOT the overall repository mission or a platform classification.\n")
	fmt.Fprintf(&b, "- Repository: %s\n", rctx.Repository.FullName)
	fmt.Fprintf(&b, "- Milestone: %s\n", firstNonEmpty(rctx.Release.Tag, rctx.Release.Name))
	fmt.Fprintf(&b, "- Diagram Version: %s\n", DiagramVersion)
	b.WriteString("- Output Formats: SVG\n\n")
	b.WriteString("## Components\n")
	b.WriteString("List every component the evidence supports. For each, provide: Name; Type (e.g. compute, storage, messaging, serverless, networking, IAM, external); AWS Service (or External System); Purpose (why it exists); Relationships (adjacent components, inputs, outputs, dependencies); Repository Evidence (the specific template/resource/file that proves it exists).\n\n")
	b.WriteString("## Connections\n")
	b.WriteString("Describe every connection between components. For each: Source; Target; Protocol/mechanism (e.g. event, SQS message, HTTPS, IAM-scoped API call); Purpose; Direction; Repository Evidence.\n\n")
	b.WriteString("## Security\n")
	b.WriteString("IAM boundaries; network boundaries (VPC/subnets/security groups, if present); which resources are private vs public; encryption; authentication; authorization. Only what the evidence supports.\n\n")
	b.WriteString("## Operational Flow\n")
	b.WriteString("The end-to-end request/data flow, step by step, from the triggering event to the published output.\n\n")
	b.WriteString("## Failure Handling\n")
	b.WriteString("Retries; dead-letter queues; fallbacks; timeouts; monitoring. Only what the evidence supports; omit categories with no evidence.\n\n")
	b.WriteString("## Rendering Notes\n")
	b.WriteString("Notes a deterministic renderer needs to lay this out as an SVG using AWS Architecture Icons: suggested grouping (e.g. VPC boundary, account boundary), left-to-right vs top-down flow, which components are primary vs supporting, and a suggested SVG canvas/viewBox aspect ratio. Be concrete enough that no further interpretation is required.\n\n")

	b.WriteString("CRITICAL RULES:\n")
	b.WriteString("- Do NOT output an SVG, XML, Graphviz/DOT, Mermaid, or any rendered or diagram-markup form. Output ONLY the Markdown specification above.\n")
	b.WriteString("- Do NOT render the diagram. Describe it structurally so a renderer can draw it deterministically.\n")
	b.WriteString("- Ground every component, connection, and security boundary in the evidence; cite the specific repository evidence. If you cannot cite evidence, omit the item.\n")
	b.WriteString("- Never present planned or roadmap work as implemented. Describe only what exists in the evidence.\n")
	b.WriteString("- Do NOT emit permanent, repository-wide platform labels or classifications (e.g. \"Event-driven AI Agent Platform\", \"Serverless hybrid AI platform\", \"AWS-native AI platform\", \"Repository-level architecture overview\", \"overall platform architecture\") in the Title, Primary Engineering Topic, or anywhere else — UNLESS the engineering topic itself is explicitly about that. Scope every statement to the change this release introduces, so the same instructions work for any future release regardless of its architecture style.\n")

	return b.String()
}

// evidenceBlock assembles the deterministic, repository-grounded evidence the
// model is constrained to. It never invents: it reflects exactly what the
// Release Context captured.
func evidenceBlock(pkg ReleasePackage) string {
	rctx := pkg.Context
	var b strings.Builder
	b.WriteString("=== REPOSITORY EVIDENCE (source of truth) ===\n")
	fmt.Fprintf(&b, "Repository: %s\n", rctx.Repository.FullName)
	if s := strings.TrimSpace(rctx.Repository.Summary); s != "" {
		fmt.Fprintf(&b, "Summary: %s\n", s)
	}
	fmt.Fprintf(&b, "Release/milestone: %s\n", firstNonEmpty(rctx.Release.Tag, rctx.Release.Name))

	cfn := rctx.CloudFormation
	if len(cfn.Resources) > 0 {
		b.WriteString("\nCloudFormation resources (LogicalID — Type — Service — Category — Template):\n")
		for _, r := range cfn.Resources {
			fmt.Fprintf(&b, "- %s — %s — %s — %s — %s\n",
				r.LogicalID, dash(r.Type), dash(r.Service), dash(r.Category), dash(r.Template))
		}
	}
	if len(cfn.Services) > 0 {
		fmt.Fprintf(&b, "CloudFormation services detected: %s\n", strings.Join(cfn.Services, ", "))
	}
	if c := cfn.Counts; c.Resources > 0 {
		fmt.Fprintf(&b, "CloudFormation counts: %d resources — %d serverless, %d compute, %d storage, %d networking, %d IAM, %d messaging\n",
			c.Resources, c.Serverless, c.Compute, c.Storage, c.Networking, c.IAM, c.Messaging)
	}

	arch := rctx.Architecture
	if strings.TrimSpace(arch.Overview) != "" {
		fmt.Fprintf(&b, "\nArchitecture overview: %s\n", arch.Overview)
	}
	if len(arch.AWSServices) > 0 {
		fmt.Fprintf(&b, "AWS services (architecture): %s\n", strings.Join(arch.AWSServices, ", "))
	}
	if len(arch.Components) > 0 {
		b.WriteString("Architecture components (Name — Responsibility [kind]):\n")
		for _, c := range arch.Components {
			kind := ""
			if c.Kind != "" {
				kind = " [" + c.Kind + "]"
			}
			fmt.Fprintf(&b, "- %s — %s%s\n", c.Name, c.Responsibility, kind)
		}
	}
	writeList(&b, "Event-driven flows:", arch.EventDrivenFlows)
	if strings.TrimSpace(arch.Security) != "" {
		fmt.Fprintf(&b, "Security: %s\n", arch.Security)
	}
	if strings.TrimSpace(arch.Reliability) != "" {
		fmt.Fprintf(&b, "Reliability: %s\n", arch.Reliability)
	}
	if strings.TrimSpace(arch.Scalability) != "" {
		fmt.Fprintf(&b, "Scalability: %s\n", arch.Scalability)
	}

	if len(rctx.Mermaid) > 0 {
		b.WriteString("\nMermaid diagrams found in the repository (summary — source — edges):\n")
		for _, d := range rctx.Mermaid {
			var edges []string
			for _, e := range d.Edges {
				if e.Label != "" {
					edges = append(edges, fmt.Sprintf("%s->%s(%s)", e.From, e.To, e.Label))
				} else {
					edges = append(edges, fmt.Sprintf("%s->%s", e.From, e.To))
				}
			}
			fmt.Fprintf(&b, "- %s — %s — %s\n", d.Summary, dash(d.Source), strings.Join(edges, ", "))
		}
	}

	b.WriteString("=== END REPOSITORY EVIDENCE ===\n")
	return b.String()
}

// evidenceServices returns the deduplicated, sorted set of AWS services backed
// by evidence (CloudFormation + architecture). This is the closed vocabulary the
// specification may draw from.
func evidenceServices(rctx *rc.ReleaseContext) []string {
	seen := map[string]bool{}
	var out []string
	add := func(list []string) {
		for _, s := range list {
			s = strings.TrimSpace(s)
			if s == "" || seen[strings.ToLower(s)] {
				continue
			}
			seen[strings.ToLower(s)] = true
			out = append(out, s)
		}
	}
	add(rctx.CloudFormation.Services)
	add(rctx.Architecture.AWSServices)
	sort.Strings(out)
	return out
}

func writeList(b *strings.Builder, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(heading + "\n")
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (g *Generator) maxPromptBytes() int {
	if g.MaxPromptBytes > 0 {
		return g.MaxPromptBytes
	}
	return DefaultMaxPromptBytes
}

// safeTruncate trims s to at most n bytes on a line boundary, appending a marker
// so the model knows the evidence was cut rather than complete.
func safeTruncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	cut := s[:n]
	if i := strings.LastIndexByte(cut, '\n'); i > 0 {
		cut = cut[:i]
	}
	return cut + "\n… (evidence truncated to fit)\n"
}
