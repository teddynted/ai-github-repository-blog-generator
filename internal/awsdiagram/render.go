package awsdiagram

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

// Layout constants (pixels).
const (
	nodeW      = 176
	nodeH      = 66
	hGap       = 96
	vGap       = 30
	margin     = 44
	titleH     = 54
	legendH    = 34
	bandLabelW = 54 // left gutter holding a rotated plane label
	bandPadY   = 16
	bandGap    = 22
)

// band is a horizontal plane strip in the two-plane layout.
type band struct {
	label string
	y, h  int
	cross bool
}

// Render lays the diagram out and returns a self-contained SVG document. When the
// spec groups components into two or more planes it draws stacked plane bands
// (build above runtime, cross-cutting IAM/observability below); otherwise it
// falls back to a left-to-right dependency-layer layout. Returns "" when there is
// nothing to draw, so the caller can fall back to another renderer.
func Render(d Diagram) string {
	if len(d.Nodes) == 0 {
		return ""
	}
	if ok, pos, width, height, bands := planeLayout(d); ok {
		return draw(d, pos, width, height, bands)
	}
	pos, width, height := layeredLayout(d)
	return draw(d, pos, width, height, nil)
}

// draw emits the SVG for a computed set of node positions and optional bands.
func draw(d Diagram, pos map[string][2]int, width, height int, bands []band) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="Helvetica,Arial,sans-serif">`, width, height, width, height)
	b.WriteString(defs())
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#FFFFFF"/>`, width, height)
	fmt.Fprintf(&b, `<text x="%d" y="34" font-size="20" font-weight="700" fill="#232F3E">%s</text>`, margin, esc(d.Title))

	// Plane bands (under everything).
	for _, bd := range bands {
		b.WriteString(renderBand(bd, width))
	}
	// Edges under nodes.
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
		if p, ok := pos[n.ID]; ok {
			b.WriteString(renderNode(n, p[0], p[1]))
		}
	}
	b.WriteString(legend(margin, height-legendH+8))
	b.WriteString("</svg>")
	return b.String()
}

// planeLayout stacks the flow planes as horizontal bands with left-to-right flow
// inside each, and lays cross-cutting components (IAM/observability) in a band
// below. Returns ok=false when the spec has fewer than two flow planes.
func planeLayout(d Diagram) (ok bool, pos map[string][2]int, width, height int, bands []band) {
	flow, byPlane, cross := groupByPlane(d)
	if len(flow) < 2 {
		return false, nil, 0, 0, nil
	}
	colW := nodeW + hGap

	ordered := map[string][]string{}
	maxSlots := len(cross)
	for _, p := range flow {
		ord := intraPlaneOrder(byPlane[p], d.Edges)
		ordered[p] = ord
		if len(ord) > maxSlots {
			maxSlots = len(ord)
		}
	}

	pos = map[string][2]int{}
	xBase := margin + bandLabelW
	fullW := maxSlots*colW - hGap
	place := func(ord []string, cy int) {
		rowW := len(ord)*colW - hGap
		xoff := (fullW - rowW) / 2
		for i, id := range ord {
			pos[id] = [2]int{xBase + xoff + i*colW + nodeW/2, cy}
		}
	}

	y := titleH + margin
	for _, p := range flow {
		place(ordered[p], y+bandPadY+nodeH/2)
		bands = append(bands, band{label: p, y: y, h: nodeH + 2*bandPadY})
		y += nodeH + 2*bandPadY + bandGap
	}
	if len(cross) > 0 {
		place(cross, y+bandPadY+nodeH/2)
		bands = append(bands, band{label: "Cross-cutting", y: y, h: nodeH + 2*bandPadY, cross: true})
		y += nodeH + 2*bandPadY
	}
	width = xBase + fullW + margin
	height = y + margin + legendH
	return true, pos, width, height, bands
}

// groupByPlane splits nodes into ordered flow planes and a cross-cutting set.
// Cross-cutting = a plane named "Cross-cutting", or (when planes are absent) a
// security/observability node.
func groupByPlane(d Diagram) (flow []string, byPlane map[string][]string, cross []string) {
	byPlane = map[string][]string{}
	seen := map[string]bool{}
	for _, n := range d.Nodes {
		p := strings.ToLower(n.Plane)
		if strings.Contains(p, "cross") || strings.Contains(p, "cutting") ||
			(n.Plane == "" && (n.Category == CatSecurity || n.Category == CatObservability)) {
			cross = append(cross, n.ID)
			continue
		}
		key := n.Plane
		if key == "" {
			key = "_"
		}
		if !seen[key] {
			seen[key] = true
			flow = append(flow, key)
		}
		byPlane[key] = append(byPlane[key], n.ID)
	}
	return flow, byPlane, cross
}

// intraPlaneOrder orders a plane's nodes left-to-right by longest-path depth over
// the edges internal to the plane, breaking ties by original order.
func intraPlaneOrder(ids []string, edges []Edge) []string {
	set := map[string]bool{}
	orig := map[string]int{}
	for i, id := range ids {
		set[id] = true
		orig[id] = i
	}
	layer := map[string]int{}
	for pass := 0; pass < len(ids)+1; pass++ {
		changed := false
		for _, e := range edges {
			if !set[e.From] || !set[e.To] || e.From == e.To {
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
	out := append([]string(nil), ids...)
	sort.SliceStable(out, func(i, j int) bool {
		if layer[out[i]] != layer[out[j]] {
			return layer[out[i]] < layer[out[j]]
		}
		return orig[out[i]] < orig[out[j]]
	})
	return out
}

// layeredLayout is the fallback left-to-right dependency-layer layout.
func layeredLayout(d Diagram) (map[string][2]int, int, int) {
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
	pos := map[string][2]int{}
	colW := nodeW + hGap
	rowH := nodeH + vGap
	contentH := maxRows * rowH
	for l := 0; l <= maxLayer; l++ {
		ids := byLayer[l]
		x := margin + l*colW + nodeW/2
		offset := (contentH - len(ids)*rowH) / 2
		for i, id := range ids {
			pos[id] = [2]int{x, titleH + margin + offset + i*rowH + nodeH/2}
		}
	}
	width := margin*2 + (maxLayer+1)*colW - hGap
	height := titleH + margin*2 + contentH + legendH
	return pos, width, height
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
	return layer
}

func renderBand(bd band, width int) string {
	fill := "#F4F6F9"
	stroke := "#DCE3EC"
	if bd.cross {
		fill = "#FBF3F6"
		stroke = "#EBD3DE"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="12" fill="%s" stroke="%s" stroke-width="1"/>`,
		margin, bd.y, width-2*margin, bd.h, fill, stroke)
	lx, ly := margin+18, bd.y+bd.h/2
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" font-weight="700" letter-spacing="1" fill="#6B7A90" text-anchor="middle" transform="rotate(-90 %d %d)">%s</text>`,
		lx, ly, lx, ly, esc(strings.ToUpper(bd.label)))
	return b.String()
}

func renderNode(n Node, cx, cy int) string {
	x := cx - nodeW/2
	y := cy - nodeH/2
	fill, stroke := n.Category.fill(), n.Category.stroke()
	var b strings.Builder
	fmt.Fprintf(&b, `<g>`)
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="10" fill="#FFFFFF" stroke="%s" stroke-width="2"/>`, x, y, nodeW, nodeH, stroke)
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="34" height="%d" rx="10" fill="%s"/>`, x, y, nodeH, fill)
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="14" height="%d" fill="%s"/>`, x+20, y, nodeH, fill)
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="9" font-weight="700" fill="#FFFFFF" text-anchor="middle" transform="rotate(-90 %d %d)">%s</text>`,
		x+17, y+nodeH/2, x+17, y+nodeH/2, esc(n.Category.glyph()))
	lines := wrapLabel(n.Label, 20)
	ty := cy - (len(lines)-1)*7
	for _, ln := range lines {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="12" font-weight="600" fill="#232F3E" text-anchor="middle">%s</text>`, x+34+(nodeW-34)/2, ty+4, esc(ln))
		ty += 14
	}
	b.WriteString(`</g>`)
	return b.String()
}

// renderEdge draws a curved connector, choosing horizontal or vertical entry/exit
// points by the dominant direction so cross-plane (vertical) handoffs read cleanly.
func renderEdge(p1, p2 [2]int, e Edge) string {
	dx := p2[0] - p1[0]
	dy := p2[1] - p1[1]
	var x1, y1, x2, y2, c1x, c1y, c2x, c2y int
	if abs(dy) > abs(dx) { // vertical-dominant (cross-plane)
		x1, x2 = p1[0], p2[0]
		if dy > 0 {
			y1, y2 = p1[1]+nodeH/2, p2[1]-nodeH/2
		} else {
			y1, y2 = p1[1]-nodeH/2, p2[1]+nodeH/2
		}
		mid := (y1 + y2) / 2
		c1x, c1y, c2x, c2y = x1, mid, x2, mid
	} else { // horizontal-dominant
		y1, y2 = p1[1], p2[1]
		if dx >= 0 {
			x1, x2 = p1[0]+nodeW/2, p2[0]-nodeW/2
		} else {
			x1, x2 = p1[0]-nodeW/2, p2[0]+nodeW/2
		}
		mid := (x1 + x2) / 2
		c1x, c1y, c2x, c2y = mid, y1, mid, y2
	}
	color, width, marker := "#5A6B86", 2, "url(#arrow)"
	if e.Emphasis {
		color, width, marker = "#16A34A", 3, "url(#arrowHi)"
	}
	dash := ""
	if e.Dashed {
		dash = ` stroke-dasharray="6 5"`
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<path d="M %d %d C %d %d, %d %d, %d %d" fill="none" stroke="%s" stroke-width="%d"%s marker-end="%s"/>`,
		x1, y1, c1x, c1y, c2x, c2y, x2, y2, color, width, dash, marker)
	if e.Label != "" {
		mx, my := (x1+x2)/2, (y1+y2)/2
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="15" rx="3" fill="#FFFFFF" opacity="0.85"/>`, mx-len(e.Label)*3, my-11, len(e.Label)*6)
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" fill="#3C485C" text-anchor="middle">%s</text>`, mx, my, esc(e.Label))
	}
	return b.String()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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
		if len(lines) == 1 && len(cur) > max {
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
