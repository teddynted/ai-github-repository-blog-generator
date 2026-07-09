// Package archdiagram generates evidence-grounded AWS architecture diagrams for
// a repository (Mermaid), suitable for embedding in the generated blog. It is
// deterministic: services appear only when supported by repository evidence or
// explicit deployment context — never invented — and the Mermaid is valid by
// construction (built as a graph model, then validated and rendered).
package archdiagram

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Graph is a small directed-graph model that renders to a valid Mermaid
// flowchart. Building diagrams through it (rather than string-concatenating
// Mermaid) guarantees no duplicate ids, dangling edges, or orphan nodes.
type Graph struct {
	direction string
	order     []string
	nodes     map[string]string // id -> label
	edges     []edge
	clusters  []cluster
}

type edge struct{ from, to, label string }

type cluster struct {
	id, label string
	nodeIDs   []string
}

var idSanitizer = regexp.MustCompile(`[^A-Za-z0-9_]`)

// NewGraph creates a graph with the given direction ("TD" or "LR").
func NewGraph(direction string) *Graph {
	if direction == "" {
		direction = "TD"
	}
	return &Graph{direction: direction, nodes: map[string]string{}}
}

// AddNode adds (or relabels) a node.
func (g *Graph) AddNode(id, label string) {
	id = sanitizeID(id)
	if _, ok := g.nodes[id]; !ok {
		g.order = append(g.order, id)
	}
	g.nodes[id] = label
}

// AddEdge connects two nodes with an optional label.
func (g *Graph) AddEdge(from, to, label string) {
	g.edges = append(g.edges, edge{from: sanitizeID(from), to: sanitizeID(to), label: label})
}

// AddCluster groups nodes under a labelled subgraph (e.g. "AWS Cloud").
func (g *Graph) AddCluster(id, label string, nodeIDs ...string) {
	c := cluster{id: sanitizeID(id), label: label}
	for _, n := range nodeIDs {
		c.nodeIDs = append(c.nodeIDs, sanitizeID(n))
	}
	g.clusters = append(g.clusters, c)
}

// Validate checks structural integrity: every edge endpoint is a declared node,
// every clustered id exists, and no node is orphaned (unreferenced by any edge).
func (g *Graph) Validate() error {
	referenced := map[string]bool{}
	for _, e := range g.edges {
		if _, ok := g.nodes[e.from]; !ok {
			return fmt.Errorf("edge references undeclared node %q", e.from)
		}
		if _, ok := g.nodes[e.to]; !ok {
			return fmt.Errorf("edge references undeclared node %q", e.to)
		}
		referenced[e.from] = true
		referenced[e.to] = true
	}
	for _, c := range g.clusters {
		for _, n := range c.nodeIDs {
			if _, ok := g.nodes[n]; !ok {
				return fmt.Errorf("cluster %q references undeclared node %q", c.id, n)
			}
		}
	}
	for _, id := range g.order {
		if !referenced[id] {
			return fmt.Errorf("orphan node %q (not referenced by any edge)", id)
		}
	}
	return nil
}

// Mermaid renders the graph as a flowchart. Clustered nodes are emitted inside
// their subgraph; remaining nodes at the top level.
func (g *Graph) Mermaid() string {
	var b strings.Builder
	fmt.Fprintf(&b, "flowchart %s\n", g.direction)

	inCluster := map[string]bool{}
	for _, c := range g.clusters {
		fmt.Fprintf(&b, "  subgraph %s[\"%s\"]\n", c.id, escape(c.label))
		for _, n := range c.nodeIDs {
			fmt.Fprintf(&b, "    %s[\"%s\"]\n", n, escape(g.nodes[n]))
			inCluster[n] = true
		}
		b.WriteString("  end\n")
	}
	for _, id := range g.order {
		if !inCluster[id] {
			fmt.Fprintf(&b, "  %s[\"%s\"]\n", id, escape(g.nodes[id]))
		}
	}
	for _, e := range g.edges {
		if e.label != "" {
			fmt.Fprintf(&b, "  %s -->|%s| %s\n", e.from, escape(e.label), e.to)
		} else {
			fmt.Fprintf(&b, "  %s --> %s\n", e.from, e.to)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func sanitizeID(id string) string {
	s := idSanitizer.ReplaceAllString(id, "_")
	if s == "" {
		s = "n"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "n" + s
	}
	return s
}

// escape makes a label safe inside a Mermaid quoted node/edge.
func escape(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// sortedIDs is a small helper for deterministic output where order is by id.
func sortedIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}
