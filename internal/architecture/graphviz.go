package architecture

import (
	"fmt"
	"strings"
)

// renderGraphviz renders the graph as Graphviz DOT source. Clusters mirror the
// Mermaid subgraphs so the two formats stay visually consistent.
func renderGraphviz(g graph) string {
	var b strings.Builder
	rankdir := "TB"
	if g.Direction == "LR" {
		rankdir = "LR"
	}
	b.WriteString("digraph Architecture {\n")
	fmt.Fprintf(&b, "  rankdir=%s;\n", rankdir)
	b.WriteString("  graph [fontname=\"Helvetica\", splines=true, nodesep=0.5, ranksep=0.6];\n")
	b.WriteString("  node [shape=box, style=\"rounded,filled\", fontname=\"Helvetica\", fillcolor=\"#EEF1F5\", color=\"#232F3E\"];\n")
	b.WriteString("  edge [fontname=\"Helvetica\", color=\"#546174\"];\n\n")

	grouped := map[string]bool{}
	for i, grp := range g.Groups {
		fmt.Fprintf(&b, "  subgraph cluster_%d {\n    label=\"%s\";\n    style=\"rounded\";\n    color=\"#B7C0CD\";\n", i, dotEscape(grp.Name))
		for _, id := range grp.NodeIDs {
			if n := nodeByID(g, id); n != nil {
				writeDotNode(&b, "    ", *n)
				grouped[id] = true
			}
		}
		b.WriteString("  }\n")
	}
	for _, n := range g.Nodes {
		if !grouped[n.ID] {
			writeDotNode(&b, "  ", n)
		}
	}
	b.WriteString("\n")
	for _, e := range g.Edges {
		if e.Label != "" {
			fmt.Fprintf(&b, "  %s -> %s [label=\"%s\"];\n", e.From, e.To, dotEscape(e.Label))
		} else {
			fmt.Fprintf(&b, "  %s -> %s;\n", e.From, e.To)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func writeDotNode(b *strings.Builder, indent string, n gnode) {
	fill := "#EEF1F5"
	font := "#232F3E"
	if n.Color != "" {
		fill = n.Color
		font = "#FFFFFF"
	}
	fmt.Fprintf(b, "%s%s [label=\"%s\", fillcolor=\"%s\", fontcolor=\"%s\"];\n", indent, n.ID, dotEscape(n.Label), fill, font)
}

func dotEscape(s string) string {
	return strings.ReplaceAll(collapse(s), "\"", "\\\"")
}
