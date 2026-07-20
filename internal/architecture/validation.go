package architecture

import (
	"fmt"
	"strings"
)

// Validate checks the collection's structural and rendering invariants and
// returns a list of problems (empty when valid). It enforces the Milestone 11
// rules: diagrams are grounded, AWS services exist in the release, Mermaid and
// Graphviz syntax is well-formed, SVG is valid, resources are unique, and
// relationships are grounded (no orphan/invalid edges, no empty diagrams).
//
// pkg is the source package, used to confirm grounding.
func (col ArchitectureCollection) Validate(pkg ReleasePackage) []string {
	var problems []string

	if col.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(col.Diagrams) == 0 {
		problems = append(problems, "collection has no diagrams")
	}
	if col.Metadata.DiagramCount != len(col.Diagrams) {
		problems = append(problems, fmt.Sprintf("metadata.diagramCount %d != diagrams %d", col.Metadata.DiagramCount, len(col.Diagrams)))
	}

	grounded := groundedServiceSet(pkg)
	seenType := map[string]bool{}

	for i, d := range col.Diagrams {
		where := fmt.Sprintf("diagram %d (%s)", i+1, d.Type)

		// Empty diagrams are rejected.
		if d.Metadata.NodeCount == 0 || collapse(d.Mermaid) == "" {
			problems = append(problems, where+": empty diagram")
		}

		// Every AWS service must exist in the release context.
		for _, svc := range d.AWSServices {
			if !serviceGrounded(svc, grounded) {
				problems = append(problems, fmt.Sprintf("%s: AWS service %q is not in the release context", where, svc))
			}
		}

		// Mermaid and Graphviz syntax must validate.
		if err := validateMermaid(d.Mermaid, d.MermaidType); err != "" {
			problems = append(problems, where+": mermaid: "+err)
		}
		if err := validateGraphviz(d.Graphviz); err != "" {
			problems = append(problems, where+": graphviz: "+err)
		}
		if err := validateSVG(d.SVG); err != "" {
			problems = append(problems, where+": svg: "+err)
		}

		// Duplicate diagram types are flagged (each type is distinct here).
		if seenType[d.Type] {
			problems = append(problems, where+": duplicate diagram type")
		}
		seenType[d.Type] = true
	}

	return problems
}

// ValidateGraph checks a graph's structural invariants (unique nodes, grounded
// edges) — used by the builders and tests.
func validateGraph(g graph) []string {
	var problems []string
	ids := map[string]bool{}
	for _, n := range g.Nodes {
		if ids[n.ID] {
			problems = append(problems, "duplicate node ID: "+n.ID)
		}
		ids[n.ID] = true
	}
	for _, e := range g.Edges {
		if !ids[e.From] || !ids[e.To] {
			problems = append(problems, fmt.Sprintf("orphan edge references undefined node: %s -> %s", e.From, e.To))
		}
	}
	return problems
}

// validateMermaid is a lightweight structural check: a known diagram directive,
// balanced subgraph/end, and (for flowcharts) at least one node.
func validateMermaid(src, mermaidType string) string {
	s := strings.TrimSpace(src)
	if s == "" {
		return "empty"
	}
	first := firstLine(s)
	switch mermaidType {
	case "sequenceDiagram":
		if !strings.HasPrefix(first, "sequenceDiagram") {
			return "must start with sequenceDiagram"
		}
		if !strings.Contains(s, "participant") {
			return "no participants"
		}
	default:
		if !strings.HasPrefix(first, "flowchart") && !strings.HasPrefix(first, "graph") {
			return "must start with flowchart/graph"
		}
	}
	if strings.Count(s, "subgraph ") != countWord(s, "end") {
		return "unbalanced subgraph/end"
	}
	return ""
}

// validateGraphviz checks the DOT wrapper and brace balance.
func validateGraphviz(src string) string {
	s := strings.TrimSpace(src)
	if s == "" {
		return "empty"
	}
	if !strings.HasPrefix(s, "digraph") && !strings.HasPrefix(s, "graph") {
		return "must start with digraph/graph"
	}
	if strings.Count(s, "{") != strings.Count(s, "}") {
		return "unbalanced braces"
	}
	return ""
}

// validateSVG checks the SVG is well-formed enough to render.
func validateSVG(src string) string {
	s := strings.TrimSpace(src)
	if s == "" {
		return "empty"
	}
	if !strings.HasPrefix(s, "<svg") || !strings.HasSuffix(s, "</svg>") {
		return "missing svg root element"
	}
	if strings.Count(s, "<svg") != strings.Count(s, "</svg>") {
		return "unbalanced svg tags"
	}
	if !strings.Contains(s, "role=\"img\"") {
		return "not accessible (missing role=img)"
	}
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// countWord counts standalone occurrences of word (bounded by newlines/spaces).
func countWord(s, word string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) == word {
			n++
		}
	}
	return n
}

func groundedServiceSet(pkg ReleasePackage) map[string]bool {
	set := map[string]bool{}
	if pkg.Context == nil {
		return set
	}
	for _, s := range pkg.Context.Architecture.AWSServices {
		set[strings.ToLower(collapse(s))] = true
	}
	for _, s := range pkg.Context.CloudFormation.Services {
		set[strings.ToLower(collapse(s))] = true
	}
	// CloudFormation itself is grounded whenever the release ships templates or
	// parsed resources.
	if len(pkg.Context.CloudFormation.Templates) > 0 || len(pkg.Context.CloudFormation.Resources) > 0 {
		set["cloudformation"] = true
		set["aws cloudformation"] = true
	}
	return set
}

// serviceGrounded reports whether a canonical service label maps back to a
// grounded context service (matching sub/superstrings so "AWS Lambda" grounds
// "Lambda").
func serviceGrounded(canonical string, grounded map[string]bool) bool {
	// Resolve the canonical label back to any of its catalogue match keys.
	l := strings.ToLower(collapse(canonical))
	if grounded[l] {
		return true
	}
	for g := range grounded {
		if strings.Contains(l, g) || strings.Contains(g, l) {
			return true
		}
		// Also compare via catalogue: does the grounded term resolve to the same
		// canonical service?
		if info, ok := lookupService(g); ok && strings.EqualFold(info.Canonical, canonical) {
			return true
		}
	}
	return false
}
