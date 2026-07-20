package architecture

import (
	"fmt"
	"strings"
)

// SVG layout constants (pixels).
const (
	svgPad     = 24
	svgColW    = 190
	svgColGap  = 48
	svgBoxH    = 48
	svgBoxGap  = 22
	svgHeaderH = 30
	svgTitleH  = 52
)

type box struct {
	x, y, w, h int
	cx, cy     int
}

// renderSVG renders the graph as a self-contained, accessible SVG. Nodes are
// laid out in columns (one per group, or a grid when ungrouped); grounded edges
// are drawn as arrows between node centers. It embeds no external assets.
func renderSVG(g graph, title string) string {
	cols := layoutColumns(g)
	boxes := map[string]box{}

	maxRows := 0
	for _, c := range cols {
		if len(c.ids) > maxRows {
			maxRows = len(c.ids)
		}
	}
	width := svgPad*2 + len(cols)*svgColW + max(0, len(cols)-1)*svgColGap
	if width < 480 {
		width = 480
	}
	height := svgTitleH + svgHeaderH + maxRows*(svgBoxH+svgBoxGap) + svgPad
	if height < 200 {
		height = 200
	}

	// Compute positions.
	for ci, c := range cols {
		x := svgPad + ci*(svgColW+svgColGap)
		y := svgTitleH + svgHeaderH
		for _, id := range c.ids {
			b := box{x: x, y: y, w: svgColW, h: svgBoxH}
			b.cx = x + b.w/2
			b.cy = y + b.h/2
			boxes[id] = b
			y += svgBoxH + svgBoxGap
		}
	}

	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-label="%s">`, width, height, width, height, xmlEscape(title))
	s.WriteString("\n")
	fmt.Fprintf(&s, "  <title>%s</title>\n", xmlEscape(title))
	fmt.Fprintf(&s, "  <desc>Architecture diagram with %d components and %d relationships, grounded in the release context.</desc>\n", len(g.Nodes), len(g.Edges))
	s.WriteString(`  <defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="#546174"/></marker></defs>` + "\n")
	fmt.Fprintf(&s, `  <rect width="%d" height="%d" fill="#FFFFFF"/>`+"\n", width, height)
	fmt.Fprintf(&s, `  <text x="%d" y="34" font-family="Helvetica, Arial, sans-serif" font-size="20" font-weight="700" fill="#232F3E">%s</text>`+"\n", svgPad, xmlEscape(title))

	// Edges first (under nodes).
	for _, e := range g.Edges {
		a, ok1 := boxes[e.From]
		b, ok2 := boxes[e.To]
		if !ok1 || !ok2 {
			continue
		}
		fmt.Fprintf(&s, `  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#546174" stroke-width="2" marker-end="url(#arrow)"/>`+"\n", a.cx, a.cy, b.cx, b.cy)
	}

	// Column headers.
	for ci, c := range cols {
		if c.name == "" {
			continue
		}
		x := svgPad + ci*(svgColW+svgColGap)
		fmt.Fprintf(&s, `  <text x="%d" y="%d" font-family="Helvetica, Arial, sans-serif" font-size="13" font-weight="700" fill="#8A94A6">%s</text>`+"\n", x, svgTitleH+18, xmlEscape(strings.ToUpper(c.name)))
	}

	// Node boxes.
	for _, n := range g.Nodes {
		b, ok := boxes[n.ID]
		if !ok {
			continue
		}
		fill := "#EEF1F5"
		fg := "#232F3E"
		if n.Color != "" {
			fill = n.Color
			fg = "#FFFFFF"
		}
		fmt.Fprintf(&s, `  <rect x="%d" y="%d" width="%d" height="%d" rx="8" fill="%s" stroke="#232F3E" stroke-width="1.5"/>`+"\n", b.x, b.y, b.w, b.h, fill)
		fmt.Fprintf(&s, `  <text x="%d" y="%d" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-size="13" font-weight="600" fill="%s">%s</text>`+"\n", b.cx, b.cy+5, fg, xmlEscape(truncateLabel(n.Label, 24)))
	}

	s.WriteString("</svg>")
	return s.String()
}

type column struct {
	name string
	ids  []string
}

// layoutColumns arranges nodes into columns: one per group (ordered by
// categoryOrder), then a trailing column for any ungrouped nodes.
func layoutColumns(g graph) []column {
	grouped := map[string]bool{}
	var cols []column

	// Order groups by categoryOrder, then any remaining groups.
	byName := map[string]ggroup{}
	for _, grp := range g.Groups {
		byName[grp.Name] = grp
	}
	for _, cat := range categoryOrder {
		if grp, ok := byName[cat]; ok {
			cols = append(cols, column{name: grp.Name, ids: grp.NodeIDs})
			for _, id := range grp.NodeIDs {
				grouped[id] = true
			}
			delete(byName, cat)
		}
	}
	for _, grp := range g.Groups {
		if _, ok := byName[grp.Name]; ok {
			cols = append(cols, column{name: grp.Name, ids: grp.NodeIDs})
			for _, id := range grp.NodeIDs {
				grouped[id] = true
			}
			delete(byName, grp.Name)
		}
	}

	// Ungrouped nodes: chunk into columns of a reasonable height.
	var ungrouped []string
	for _, n := range g.Nodes {
		if !grouped[n.ID] {
			ungrouped = append(ungrouped, n.ID)
		}
	}
	const perCol = 6
	for i := 0; i < len(ungrouped); i += perCol {
		end := i + perCol
		if end > len(ungrouped) {
			end = len(ungrouped)
		}
		cols = append(cols, column{ids: ungrouped[i:end]})
	}
	if len(cols) == 0 {
		cols = append(cols, column{})
	}
	return cols
}

func truncateLabel(s string, max int) string {
	s = collapse(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;")
	return r.Replace(collapse(s))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
