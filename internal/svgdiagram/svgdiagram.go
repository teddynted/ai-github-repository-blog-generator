// Package svgdiagram renders a directed graph (nodes + labelled edges) into a
// self-contained, dependency-free SVG document. It draws plain boxes and arrows
// grouped/coloured by category — not AWS-icon artwork — so it needs no external
// tools, no network, and no asset files, and produces byte-for-byte
// deterministic output for the same input.
//
// It is intentionally generic (no repository types) so it can render both the
// architecture graph derived from repository evidence and the flow diagrams
// parsed from a repository's Mermaid.
package svgdiagram

import (
	"fmt"
	"strings"
)

// Node is a box in the diagram. Group is an optional category ("compute",
// "storage", "messaging", …) used for colour; when empty it is inferred from
// the label.
type Node struct {
	ID    string
	Label string
	Group string
}

// Edge is a directed, optionally labelled connection between two node IDs.
type Edge struct {
	From, To string
	Label    string
}

// Graph is one diagram panel.
type Graph struct {
	Title string
	Nodes []Node
	Edges []Edge
}

// Layout constants (pixels).
const (
	nodeW       = 190
	nodeH       = 58
	hGap        = 84
	vGap        = 30
	margin      = 28
	panelTitleH = 30
	docTitleH   = 46
	maxCols     = 6
	labelMax    = 26
)

// Render renders a single graph as a complete SVG document.
func Render(g Graph) string { return RenderDocument(g.Title, []Graph{g}) }

// RenderDocument renders one or more graphs as vertically stacked panels in a
// single self-contained SVG document.
func RenderDocument(title string, graphs []Graph) string {
	panels := make([]string, 0, len(graphs))
	maxW := 0
	y := docTitleH
	for _, g := range graphs {
		body, w, h := renderPanel(g)
		if w > maxW {
			maxW = w
		}
		panels = append(panels, fmt.Sprintf(`<g transform="translate(0,%d)">%s</g>`, y, body))
		y += h
	}
	if maxW == 0 {
		maxW = 320
	}
	totalH := y + margin

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" font-family="Helvetica, Arial, sans-serif">`,
		maxW, totalH, maxW, totalH)
	b.WriteString(`<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" fill="#5a5a5a"/></marker></defs>`)
	b.WriteString(`<rect x="0" y="0" width="100%" height="100%" fill="#ffffff"/>`)
	if title != "" {
		fmt.Fprintf(&b, `<text x="%d" y="30" font-size="20" font-weight="700" fill="#1a1a1a">%s</text>`, margin, escape(truncate(title, 80)))
	}
	for _, p := range panels {
		b.WriteString(p)
	}
	b.WriteString(`</svg>`)
	return b.String()
}

type placed struct {
	Node
	x, y int
}

// renderPanel lays out one graph and returns its SVG body plus the panel's
// width and height. Coordinates are panel-local (origin at the panel top-left).
func renderPanel(g Graph) (body string, width, height int) {
	nodes, index := unionNodes(g)
	if len(nodes) == 0 {
		return "", 0, 0
	}
	layer := assignLayers(nodes, index, g.Edges)

	// Order nodes into columns by layer, rows by input order.
	rows := map[int]int{}
	pos := make(map[string]placed, len(nodes))
	maxLayer, maxRow := 0, 0
	for _, n := range nodes {
		l := layer[n.ID]
		r := rows[l]
		rows[l]++
		x := margin + l*(nodeW+hGap)
		yy := panelTitleH + margin + r*(nodeH+vGap)
		pos[n.ID] = placed{Node: n, x: x, y: yy}
		if l > maxLayer {
			maxLayer = l
		}
		if r > maxRow {
			maxRow = r
		}
	}

	var b strings.Builder
	if g.Title != "" {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="15" font-weight="600" fill="#333">%s</text>`,
			margin, 20, escape(truncate(g.Title, 90)))
	}

	// Edges first (under the boxes).
	for _, e := range g.Edges {
		s, ok1 := pos[e.From]
		t, ok2 := pos[e.To]
		if !ok1 || !ok2 {
			continue
		}
		x1, y1 := s.x+nodeW, s.y+nodeH/2
		x2, y2 := t.x, t.y+nodeH/2
		if t.x < s.x { // right-to-left edge: connect the near sides
			x1, x2 = s.x, t.x+nodeW
		}
		fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#5a5a5a" stroke-width="1.5" marker-end="url(#arrow)"/>`,
			x1, y1, x2, y2)
		if e.Label != "" {
			mx, my := (x1+x2)/2, (y1+y2)/2
			lbl := escape(truncate(e.Label, 22))
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="16" rx="3" fill="#ffffff" opacity="0.85"/>`,
				mx-len(lbl)*3-3, my-12, len(lbl)*6+6)
			fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="11" fill="#444" text-anchor="middle">%s</text>`, mx, my, lbl)
		}
	}

	// Boxes on top.
	for _, n := range nodes {
		p := pos[n.ID]
		fill, stroke := classColor(classify(n.Group, n.Label))
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="8" fill="%s" stroke="%s" stroke-width="1.5"/>`,
			p.x, p.y, nodeW, nodeH, fill, stroke)
		label := escape(truncate(firstNonEmpty(n.Label, n.ID), labelMax))
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="13" font-weight="600" fill="#1a1a1a" text-anchor="middle">%s</text>`,
			p.x+nodeW/2, p.y+nodeH/2, label)
		if grp := classify(n.Group, n.Label); grp != "other" {
			fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" fill="#666" text-anchor="middle">%s</text>`,
				p.x+nodeW/2, p.y+nodeH/2+16, escape(grp))
		}
	}

	width = margin*2 + (maxLayer+1)*(nodeW+hGap) - hGap
	height = panelTitleH + margin*2 + (maxRow+1)*(nodeH+vGap) - vGap
	return b.String(), width, height
}

// unionNodes returns the graph's nodes plus any edge endpoints not declared as
// nodes (so an edge never dangles), de-duplicated, preserving first-seen order.
func unionNodes(g Graph) ([]Node, map[string]bool) {
	index := map[string]bool{}
	var out []Node
	add := func(n Node) {
		if n.ID == "" || index[n.ID] {
			return
		}
		index[n.ID] = true
		out = append(out, n)
	}
	for _, n := range g.Nodes {
		add(n)
	}
	for _, e := range g.Edges {
		add(Node{ID: e.From, Label: e.From})
		add(Node{ID: e.To, Label: e.To})
	}
	return out, index
}

// assignLayers computes a left-to-right column index per node via longest-path
// layering. It is cycle-safe (bounded iterations) and clamps depth so wide
// graphs stay readable.
func assignLayers(nodes []Node, index map[string]bool, edges []Edge) map[string]int {
	layer := make(map[string]int, len(nodes))
	for _, n := range nodes {
		layer[n.ID] = 0
	}
	for i := 0; i < len(nodes); i++ {
		changed := false
		for _, e := range edges {
			if !index[e.From] || !index[e.To] {
				continue
			}
			if layer[e.To] < layer[e.From]+1 {
				layer[e.To] = layer[e.From] + 1
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	for id, l := range layer {
		if l > maxCols-1 {
			layer[id] = maxCols - 1
		}
	}
	return layer
}

// classify maps an explicit group or, failing that, keywords in the label to a
// canonical component class used for colouring.
func classify(group, label string) string {
	s := strings.ToLower(group + " " + label)
	switch {
	case containsAny(s, "lambda", "ec2", "compute", "fargate", "ecs", "worker", "auto scaling", "autoscaling"):
		return "compute"
	case containsAny(s, "s3", "bucket", "efs", "dynamo", "storage", "rds", "database"):
		return "storage"
	case containsAny(s, "sqs", "sns", "eventbridge", "queue", "topic", "event", "messaging", "kinesis"):
		return "messaging"
	case containsAny(s, "vpc", "subnet", "route53", "route 53", "alb", "load balancer", "api gateway", "apigateway", "networking", "nat", "cloudfront"):
		return "networking"
	case containsAny(s, "iam", "role", "policy", "security", "kms", "secret"):
		return "iam"
	case containsAny(s, "cloudwatch", "cloudtrail", "monitor", "log"):
		return "monitoring"
	case containsAny(s, "github", "external", "user", "client", "browser"):
		return "external"
	default:
		return "other"
	}
}

// classColor returns (fill, stroke) for a component class.
func classColor(class string) (string, string) {
	switch class {
	case "compute":
		return "#FBE7D3", "#D45B07"
	case "storage":
		return "#DCEFD8", "#3F8E29"
	case "messaging":
		return "#E9DCF3", "#7A3EA6"
	case "networking":
		return "#D8E6F5", "#1E6FB8"
	case "iam":
		return "#F5DADA", "#B23A3A"
	case "monitoring":
		return "#FBF2CC", "#B58A00"
	case "external":
		return "#E4E4E4", "#555555"
	default:
		return "#ECECEC", "#888888"
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// escape XML-escapes text content/attribute values so the SVG stays well-formed.
func escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}
