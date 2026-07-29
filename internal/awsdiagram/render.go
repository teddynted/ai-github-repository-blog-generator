package awsdiagram

import (
	"fmt"
	"html"
	"strings"
)

// Layout constants (pixels).
const (
	nodeW   = 176
	nodeH   = 66
	hGap    = 96
	vGap    = 30
	margin  = 44
	titleH  = 54
	legendH = 34
)

// Render lays the diagram out left-to-right by dependency layer and returns a
// self-contained SVG document. Returns "" when there is nothing to draw, so the
// caller can fall back to another renderer.
func Render(d Diagram) string {
	if len(d.Nodes) == 0 {
		return ""
	}
	layer := assignLayers(d)
	byLayer := map[int][]string{}
	maxLayer := 0
	for _, n := range d.Nodes {
		l := layer[n.ID]
		byLayer[l] = append(byLayer[l], n.ID)
		if l > maxLayer {
			maxLayer = l
		}
	}
	maxRows := 0
	for _, ids := range byLayer {
		if len(ids) > maxRows {
			maxRows = len(ids)
		}
	}

	pos := map[string][2]int{} // node ID → center (x,y)
	colW := nodeW + hGap
	rowH := nodeH + vGap
	contentH := maxRows * rowH
	for l := 0; l <= maxLayer; l++ {
		ids := byLayer[l]
		x := margin + l*colW + nodeW/2
		// center this column's rows vertically within contentH
		offset := (contentH - len(ids)*rowH) / 2
		for i, id := range ids {
			y := titleH + margin + offset + i*rowH + nodeH/2
			pos[id] = [2]int{x, y}
		}
	}
	width := margin*2 + (maxLayer+1)*colW - hGap
	height := titleH + margin*2 + contentH + legendH

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="Helvetica,Arial,sans-serif">`, width, height, width, height)
	b.WriteString(defs())
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#FFFFFF"/>`, width, height)
	fmt.Fprintf(&b, `<text x="%d" y="34" font-size="20" font-weight="700" fill="#232F3E">%s</text>`, margin, esc(d.Title))

	nodeByID := map[string]Node{}
	for _, n := range d.Nodes {
		nodeByID[n.ID] = n
	}
	// Edges first (under the nodes).
	for _, e := range d.Edges {
		p1, ok1 := pos[e.From]
		p2, ok2 := pos[e.To]
		if !ok1 || !ok2 {
			continue
		}
		b.WriteString(renderEdge(p1, p2, e))
	}
	// Nodes on top.
	for _, n := range d.Nodes {
		p := pos[n.ID]
		b.WriteString(renderNode(n, p[0], p[1]))
	}
	b.WriteString(legend(margin, height-legendH+8))
	b.WriteString("</svg>")
	return b.String()
}

// assignLayers gives each node a dependency layer via longest-path relaxation,
// bounded so cycles terminate.
func assignLayers(d Diagram) map[string]int {
	layer := map[string]int{}
	for _, n := range d.Nodes {
		layer[n.ID] = 0
	}
	for pass := 0; pass < len(d.Nodes)+1; pass++ {
		changed := false
		for _, e := range d.Edges {
			if e.From == e.To {
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
	// Keep IDs stable within a layer by original node order (handled by caller
	// iterating d.Nodes), so no extra sort needed here.
	return layer
}

func renderNode(n Node, cx, cy int) string {
	x := cx - nodeW/2
	y := cy - nodeH/2
	fill, stroke := n.Category.fill(), n.Category.stroke()
	var b strings.Builder
	fmt.Fprintf(&b, `<g>`)
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="10" fill="#FFFFFF" stroke="%s" stroke-width="2"/>`, x, y, nodeW, nodeH, stroke)
	// Category badge (left colour bar + glyph).
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="34" height="%d" rx="10" fill="%s"/>`, x, y, nodeH, fill)
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="14" height="%d" fill="%s"/>`, x+20, y, nodeH, fill)
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="9" font-weight="700" fill="#FFFFFF" text-anchor="middle" transform="rotate(-90 %d %d)">%s</text>`,
		x+17, y+nodeH/2, x+17, y+nodeH/2, esc(n.Category.glyph()))
	// Label (up to two wrapped lines).
	lines := wrapLabel(n.Label, 20)
	ty := cy - (len(lines)-1)*7
	for _, ln := range lines {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="12" font-weight="600" fill="#232F3E" text-anchor="middle">%s</text>`, x+34+(nodeW-34)/2, ty+4, esc(ln))
		ty += 14
	}
	b.WriteString(`</g>`)
	return b.String()
}

func renderEdge(p1, p2 [2]int, e Edge) string {
	x1, y1 := p1[0]+nodeW/2, p1[1]
	x2, y2 := p2[0]-nodeW/2, p2[1]
	if x2 < x1 { // back/same-layer edge: route from bottoms
		x1, y1 = p1[0], p1[1]+nodeH/2
		x2, y2 = p2[0], p2[1]+nodeH/2
	}
	color := "#5A6B86"
	width := 2
	marker := "url(#arrow)"
	if e.Emphasis {
		color = "#16A34A"
		width = 3
		marker = "url(#arrowHi)"
	}
	dash := ""
	if e.Dashed {
		dash = ` stroke-dasharray="6 5"`
	}
	midx, midy := (x1+x2)/2, (y1+y2)/2
	var b strings.Builder
	fmt.Fprintf(&b, `<path d="M %d %d C %d %d, %d %d, %d %d" fill="none" stroke="%s" stroke-width="%d"%s marker-end="%s"/>`,
		x1, y1, midx, y1, midx, y2, x2, y2, color, width, dash, marker)
	if e.Label != "" {
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="15" rx="3" fill="#FFFFFF" opacity="0.85"/>`, midx-len(e.Label)*3, midy-11, len(e.Label)*6)
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" fill="#3C485C" text-anchor="middle">%s</text>`, midx, midy, esc(e.Label))
	}
	return b.String()
}

func defs() string {
	return `<defs>` +
		`<marker id="arrow" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto"><path d="M0,0 L8,3 L0,6 Z" fill="#5A6B86"/></marker>` +
		`<marker id="arrowHi" markerWidth="12" markerHeight="12" refX="8" refY="3" orient="auto"><path d="M0,0 L8,3 L0,6 Z" fill="#16A34A"/></marker>` +
		`</defs>`
}

func legend(x, y int) string {
	return fmt.Sprintf(
		`<g font-size="11" fill="#3C485C">`+
			`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#5A6B86" stroke-width="2" marker-end="url(#arrow)"/>`+
			`<text x="%d" y="%d">data / control flow</text>`+
			`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#5A6B86" stroke-width="2" stroke-dasharray="6 5"/>`+
			`<text x="%d" y="%d">governance / observability</text>`+
			`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#16A34A" stroke-width="3" marker-end="url(#arrowHi)"/>`+
			`<text x="%d" y="%d">primary change</text>`+
			`</g>`,
		x, y+8, x+30, y+8, x+36, y+12,
		x+180, y+8, x+210, y+8, x+216, y+12,
		x+420, y+8, x+450, y+8, x+456, y+12,
	)
}

func wrapLabel(label string, max int) []string {
	words := strings.Fields(label)
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if len(cur)+1+len(w) <= max {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
		if len(lines) == 1 && len(cur) > max { // cap at 2 lines
			cur = truncate(cur, max)
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > 2 {
		lines = []string{lines[0], truncate(strings.Join(lines[1:], " "), max)}
	}
	if len(lines) == 0 {
		lines = []string{label}
	}
	return lines
}

func esc(s string) string { return html.EscapeString(s) }
