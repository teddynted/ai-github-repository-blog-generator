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
	services := evidenceServices(rctx, pkg.Blog.Markdown)

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
	scoped := serviceSet(evidenceServices(rctx, pkg.Blog.Markdown))
	var b strings.Builder

	// The RELEASE BLOG is the primary source of truth — the spec must describe only
	// what THIS release documents.
	b.WriteString("=== RELEASE BLOG (primary source — describe only what this evidences) ===\n")
	fmt.Fprintf(&b, "Repository: %s\n", rctx.Repository.FullName)
	fmt.Fprintf(&b, "Release/milestone: %s\n", firstNonEmpty(rctx.Release.Tag, rctx.Release.Name))
	if t := releaseTopic(pkg.Blog.Title); t != "" {
		fmt.Fprintf(&b, "Release topic: %s\n", t)
	}
	if body := strings.TrimSpace(pkg.Blog.Markdown); body != "" {
		b.WriteString("\nBlog content:\n")
		b.WriteString(body + "\n")
	}
	// The release's OWN diagrams (from the blog), not the repository's.
	if diagrams := blogMermaid(pkg.Blog.Markdown); len(diagrams) > 0 {
		b.WriteString("\nDiagrams in the release blog:\n")
		for _, d := range diagrams {
			fmt.Fprintf(&b, "```\n%s\n```\n", d)
		}
	}
	b.WriteString("=== END RELEASE BLOG ===\n\n")

	// Supporting repository evidence, SCOPED to the release: only CloudFormation
	// resources whose service the blog discusses — so unrelated stacks/roles never
	// enter the spec.
	b.WriteString("=== SUPPORTING REPOSITORY EVIDENCE (only where it corroborates the blog) ===\n")
	var rels []string
	for _, r := range rctx.CloudFormation.Resources {
		if scoped[strings.ToLower(strings.TrimSpace(r.Service))] {
			rels = append(rels, fmt.Sprintf("- %s — %s — %s — %s — %s",
				r.LogicalID, dash(r.Type), dash(r.Service), dash(r.Category), dash(r.Template)))
		}
	}
	if len(rels) > 0 {
		b.WriteString("CloudFormation resources for release-relevant services (LogicalID — Type — Service — Category — Template):\n")
		b.WriteString(strings.Join(rels, "\n") + "\n")
	}
	b.WriteString("=== END SUPPORTING EVIDENCE ===\n")
	return b.String()
}

// serviceSet returns a lowercase lookup set of service names.
func serviceSet(services []string) map[string]bool {
	set := map[string]bool{}
	for _, s := range services {
		set[strings.ToLower(strings.TrimSpace(s))] = true
	}
	return set
}

// blogMermaid returns the raw contents of every ```mermaid block in the blog —
// the release's own, release-specific diagrams.
func blogMermaid(md string) []string {
	var blocks []string
	var cur []string
	in := false
	for _, ln := range strings.Split(md, "\n") {
		t := strings.TrimSpace(ln)
		switch {
		case !in && strings.HasPrefix(t, "```mermaid"):
			in, cur = true, nil
		case in && strings.HasPrefix(t, "```"):
			in = false
			nonBlank := 0
			for _, c := range cur {
				if strings.TrimSpace(c) != "" {
					nonBlank++
				}
			}
			if nonBlank >= 2 {
				blocks = append(blocks, strings.Join(cur, "\n"))
			}
		case in:
			cur = append(cur, ln)
		}
	}
	return blocks
}

// evidenceServices returns the closed AWS-service vocabulary the specification may
// draw from: services backed by repository evidence (CloudFormation + architecture)
// AND actually discussed in the release blog. Scoping to the blog keeps the spec
// release-specific — a service the repository uses but this release never mentions
// (e.g. Bedrock in an AMI release) is excluded.
func evidenceServices(rctx *rc.ReleaseContext, blogMd string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(list []string) {
		for _, s := range list {
			s = strings.TrimSpace(s)
			if s == "" || seen[strings.ToLower(s)] {
				continue
			}
			seen[strings.ToLower(s)] = true
			if blogMd == "" || blogMentionsService(blogMd, s) {
				out = append(out, s)
			}
		}
	}
	add(rctx.CloudFormation.Services)
	add(rctx.Architecture.AWSServices)
	out = preferCanonical(out)
	sort.Strings(out)
	return out
}

// preferCanonical drops a service name that is a whole-word subset of a longer
// name already present — so raw CloudFormation namespace tokens ("EC2", "Lambda",
// "Scheduler") give way to their canonical forms ("Amazon EC2", "AWS Lambda",
// "Amazon EventBridge Scheduler").
func preferCanonical(services []string) []string {
	var out []string
	for _, s := range services {
		// Always keep vendor-prefixed canonical names — distinct services like
		// "Amazon EventBridge" and "Amazon EventBridge Scheduler" must both survive.
		if strings.HasPrefix(s, "Amazon ") || strings.HasPrefix(s, "AWS ") {
			out = append(out, s)
			continue
		}
		// Drop a bare token that is a whole-word subset of a longer name present.
		ls := strings.ToLower(s)
		subsumed := false
		for _, other := range services {
			if other != s && len(other) > len(s) && archspecContainsWord(strings.ToLower(other), ls) {
				subsumed = true
				break
			}
		}
		if !subsumed {
			out = append(out, s)
		}
	}
	return out
}

// blogMentionsService reports whether the blog discusses the given AWS service,
// matching its distinctive token (the name without the "Amazon"/"AWS" vendor
// prefix) as a whole word — so "Amazon EventBridge" matches "EventBridge" and
// "AWS Lambda" matches "Lambda", without false positives from substrings.
func blogMentionsService(blogMd, service string) bool {
	tok := strings.ToLower(strings.TrimSpace(service))
	for _, p := range []string{"amazon ", "aws ", "amazon", "aws"} {
		tok = strings.TrimSpace(strings.TrimPrefix(tok, p))
	}
	if tok == "" {
		return false
	}
	return archspecContainsWord(strings.ToLower(blogMd), tok)
}

// archspecContainsWord reports whether text contains word bounded by non-word
// characters (letters/digits are word characters).
func archspecContainsWord(text, word string) bool {
	isWord := func(b byte) bool {
		return b >= 'a' && b <= 'z' || b >= '0' && b <= '9'
	}
	for idx := 0; ; {
		i := strings.Index(text[idx:], word)
		if i < 0 {
			return false
		}
		i += idx
		before := i == 0 || !isWord(text[i-1])
		after := i+len(word) >= len(text) || !isWord(text[i+len(word)])
		if before && after {
			return true
		}
		idx = i + 1
	}
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
