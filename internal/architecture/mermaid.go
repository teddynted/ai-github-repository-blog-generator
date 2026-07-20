package architecture

import (
	"fmt"
	"strings"
)

// renderMermaid renders the graph as Mermaid source for the given diagram type.
func renderMermaid(g graph, mermaidType string) string {
	if mermaidType == "sequenceDiagram" {
		return renderSequence(g)
	}
	return renderFlowchart(g)
}

// renderFlowchart renders a Mermaid flowchart with category subgraphs and edges.
func renderFlowchart(g graph) string {
	var b strings.Builder
	dir := g.Direction
	if dir == "" {
		dir = "TD"
	}
	fmt.Fprintf(&b, "flowchart %s\n", dir)

	grouped := map[string]bool{}
	for _, grp := range g.Groups {
		fmt.Fprintf(&b, "    subgraph %s[\"%s\"]\n", sanitizeID(grp.Name), escapeLabel(grp.Name))
		for _, id := range grp.NodeIDs {
			if n := nodeByID(g, id); n != nil {
				fmt.Fprintf(&b, "        %s[\"%s\"]\n", n.ID, escapeLabel(n.Label))
				grouped[id] = true
			}
		}
		b.WriteString("    end\n")
	}
	// Ungrouped nodes.
	for _, n := range g.Nodes {
		if !grouped[n.ID] {
			fmt.Fprintf(&b, "    %s[\"%s\"]\n", n.ID, escapeLabel(n.Label))
		}
	}
	// Edges.
	for _, e := range g.Edges {
		if e.Label != "" {
			fmt.Fprintf(&b, "    %s -->|%s| %s\n", e.From, escapeLabel(e.Label), e.To)
		} else {
			fmt.Fprintf(&b, "    %s --> %s\n", e.From, e.To)
		}
	}
	// Category class styling for AWS nodes.
	writeMermaidStyles(&b, g)
	return strings.TrimRight(b.String(), "\n")
}

func writeMermaidStyles(b *strings.Builder, g graph) {
	for _, n := range g.Nodes {
		if n.Color != "" {
			fmt.Fprintf(b, "    style %s fill:%s,stroke:#232F3E,color:#fff\n", n.ID, n.Color)
		}
	}
}

// renderSequence renders the graph's linear edge chain as a sequenceDiagram.
func renderSequence(g graph) string {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")
	// Participants in node order.
	for _, n := range g.Nodes {
		fmt.Fprintf(&b, "    participant %s as %s\n", n.ID, escapeLabel(n.Label))
	}
	for _, e := range g.Edges {
		label := e.Label
		if label == "" {
			label = "sends to"
		}
		fmt.Fprintf(&b, "    %s->>%s: %s\n", e.From, e.To, escapeLabel(label))
	}
	return strings.TrimRight(b.String(), "\n")
}

func nodeByID(g graph, id string) *gnode {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}
