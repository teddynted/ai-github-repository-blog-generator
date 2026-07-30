package releasecontext

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// flowchart / graph arrow operators, longest-first so "-.->" beats "-->".
var flowArrows = []string{"-.->", "==>", "-->", "---", "-.-"}

// sequence-diagram message operators, longest-first.
var seqArrows = []string{"-->>", "->>", "-->", "->", "--x", "-x"}

// idRe captures a leading Mermaid node identifier (before any [] () {} label).
var idRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+`)

// nodeDefRe captures a node id immediately followed by a bracketed label —
// "GH[GitHub Release]", "APIGW(API Gateway)", "T{Valid?}" — group 1 = id,
// group 2 = label text (up to the first closing bracket or quote).
var nodeDefRe = regexp.MustCompile(`([A-Za-z0-9_.-]+)[\[({]"?([^"\]})]+)`)

// cleanMermaidLabel normalises a captured node label: it drops line-break tags
// and strips stray shape/quote characters left by special node syntaxes such as
// cylinders ("[(text)]") or rounded nodes ("(text)").
func cleanMermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "<br/>", " ")
	s = strings.ReplaceAll(s, "<br>", " ")
	s = strings.Trim(s, "[](){}\"' \t")
	return strings.TrimSpace(s)
}

// parseParticipant reads a sequence-diagram "participant <id> as <label>" or
// "actor <id> as <label>" line (label optional), returning the id and cleaned
// label. ok is false for any other line.
func parseParticipant(line string) (id, label string, ok bool) {
	for _, kw := range []string{"participant ", "actor "} {
		if !strings.HasPrefix(line, kw) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, kw))
		if i := strings.Index(rest, " as "); i >= 0 {
			return strings.TrimSpace(rest[:i]), cleanMermaidLabel(rest[i+4:]), true
		}
		if rest != "" {
			return rest, "", true
		}
	}
	return "", "", false
}

// analyzeMermaid extracts and parses every fenced ```mermaid block from the
// markdown inventory.
// AnalyzeMarkdown parses the Mermaid diagrams from a single markdown document,
// labelling each with the given source. It lets consumers scope to one
// document's own diagrams (e.g. a release blog) instead of the whole-repository
// set captured in ReleaseContext.Mermaid.
func AnalyzeMarkdown(content, source string) []MermaidDiagram {
	var out []MermaidDiagram
	for _, block := range mermaidBlocks(content) {
		d := parseMermaid(block)
		d.Source = source
		out = append(out, d)
	}
	return out
}

func analyzeMermaid(files []RawFile) []MermaidDiagram {
	var out []MermaidDiagram
	for _, f := range files {
		if f.Content == "" || !isMarkdownPath(f.Path) {
			continue
		}
		for _, block := range mermaidBlocks(f.Content) {
			d := parseMermaid(block)
			d.Source = f.Path
			out = append(out, d)
		}
	}
	return out
}

// mermaidBlocks returns the bodies of each ```mermaid fenced code block.
func mermaidBlocks(content string) []string {
	var blocks []string
	lines := strings.Split(content, "\n")
	in := false
	var cur []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if !in && (t == "```mermaid" || t == "~~~mermaid") {
			in, cur = true, nil
			continue
		}
		if in && (t == "```" || t == "~~~") {
			blocks = append(blocks, strings.Join(cur, "\n"))
			in = false
			continue
		}
		if in {
			cur = append(cur, ln)
		}
	}
	return blocks
}

func parseMermaid(block string) MermaidDiagram {
	lines := strings.Split(block, "\n")
	header := ""
	for _, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			header = strings.TrimSpace(ln)
			break
		}
	}
	d := MermaidDiagram{Type: diagramType(header)}
	nodes := map[string]bool{}
	labels := map[string]string{}
	seq := d.Type == "sequence"

	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" || t == header || strings.HasPrefix(t, "%%") {
			continue
		}
		// Sequence-diagram participant declarations: "participant B as Builder"
		// (or "actor …"). Capture the human label so consumers surface "Builder",
		// not the cryptic id "B", and register the participant as a node.
		if seq {
			if id, lbl, ok := parseParticipant(t); ok {
				nodes[id] = true
				if lbl != "" && labels[id] == "" {
					labels[id] = lbl
				}
				continue
			}
		}
		// Capture human labels from node definitions (inline or standalone):
		// "GH[GitHub Release]", "APIGW(API Gateway)", "T{Valid?}".
		for _, m := range nodeDefRe.FindAllStringSubmatch(t, -1) {
			if id, lbl := m[1], cleanMermaidLabel(m[2]); lbl != "" && labels[id] == "" {
				labels[id] = lbl
			}
		}
		arrows := flowArrows
		if seq {
			arrows = seqArrows
		}
		from, to, label, ok := splitEdge(t, arrows, seq)
		if !ok {
			continue
		}
		if from != "" {
			nodes[from] = true
		}
		if to != "" {
			nodes[to] = true
		}
		d.Edges = append(d.Edges, MermaidEdge{From: from, To: to, Label: label})
	}

	for n := range nodes {
		d.Nodes = append(d.Nodes, n)
	}
	// Keep only labels for nodes that appear in the graph.
	for id := range labels {
		if !nodes[id] {
			delete(labels, id)
		}
	}
	if len(labels) > 0 {
		d.NodeLabels = labels
	}
	sort.Strings(d.Nodes)
	d.NodeCount = len(d.Nodes)
	d.EdgeCount = len(d.Edges)
	d.Summary = fmt.Sprintf("%s diagram with %d nodes and %d relationships.", d.Type, d.NodeCount, d.EdgeCount)
	return d
}

func diagramType(header string) string {
	h := strings.ToLower(header)
	switch {
	case strings.HasPrefix(h, "flowchart"), strings.HasPrefix(h, "graph"):
		return "flowchart"
	case strings.HasPrefix(h, "sequencediagram"):
		return "sequence"
	case strings.HasPrefix(h, "statediagram"):
		return "state"
	case strings.HasPrefix(h, "classdiagram"):
		return "class"
	case strings.HasPrefix(h, "erdiagram"):
		return "er"
	default:
		return "other"
	}
}

// splitEdge finds the first arrow operator in a line and returns the from/to
// node ids plus an optional label. For sequence lines the label follows a colon
// ("A->>B: msg"); for flowcharts it is piped ("A -->|msg| B").
func splitEdge(line string, arrows []string, seq bool) (from, to, label string, ok bool) {
	idx, arrow := firstArrow(line, arrows)
	if idx < 0 {
		return "", "", "", false
	}
	left := strings.TrimSpace(line[:idx])
	right := strings.TrimSpace(line[idx+len(arrow):])

	if seq {
		if c := strings.IndexByte(right, ':'); c >= 0 {
			label = strings.TrimSpace(right[c+1:])
			right = strings.TrimSpace(right[:c])
		}
	} else if strings.HasPrefix(right, "|") {
		if c := strings.IndexByte(right[1:], '|'); c >= 0 {
			label = strings.TrimSpace(right[1 : 1+c])
			right = strings.TrimSpace(right[2+c:])
		}
	}
	from = nodeID(left)
	to = nodeID(right)
	if from == "" || to == "" {
		return "", "", "", false
	}
	return from, to, label, true
}

func firstArrow(line string, arrows []string) (int, string) {
	best, bestArrow := -1, ""
	for _, a := range arrows {
		if i := strings.Index(line, a); i >= 0 {
			if best < 0 || i < best || (i == best && len(a) > len(bestArrow)) {
				best, bestArrow = i, a
			}
		}
	}
	return best, bestArrow
}

// nodeID extracts the identifier from a node token, dropping label decorations
// like A[Label with spaces], B(Text), C{Cond}. Returns "" when no id is present.
func nodeID(tok string) string {
	tok = strings.TrimSpace(tok)
	// Cut at the first label opener or whitespace so a spaced label ("A[API
	// Gateway]") doesn't leak into the id.
	if i := strings.IndexAny(tok, "[({ \t"); i >= 0 {
		tok = tok[:i]
	}
	return idRe.FindString(tok)
}

func isMarkdownPath(p string) bool {
	lp := strings.ToLower(p)
	return strings.HasSuffix(lp, ".md") || strings.HasSuffix(lp, ".mmd") || strings.HasSuffix(lp, ".mdx")
}
