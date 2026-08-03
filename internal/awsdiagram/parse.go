package awsdiagram

import "strings"

// component is one entry parsed from the spec's "## Components" section. It
// carries the canonical Name (used as the node label), the AWS Service, the
// declared Type, and the Plane subsection it was grouped under.
type component struct {
	Name    string
	Service string
	Type    string
	Plane   string
}

// Parse builds a Diagram from an architecture-diagram-spec.md. It first reads
// the "## Components" section for each component's canonical name, AWS service,
// and plane grouping, then reads "## Connections" (bullet OR table form) for the
// edges. Connection endpoints are resolved back to their canonical component so
// that a raw service name ("Amazon S3") or a name with a parenthetical qualifier
// ("Versioned Custom AMI (+ snapshot)") collapses onto the one canonical node.
// Returns a zero-node Diagram when no connections are present.
func Parse(spec, title string) Diagram {
	comps, index := parseComponents(spec)

	d := Diagram{Title: title}
	idOf := map[string]string{}
	catOf := map[string]Category{}
	seenEdge := map[string]bool{}

	addNode := func(label string) string {
		c := resolveComponent(label, index)
		name := label
		typ, plane := "", ""
		if c != nil {
			name, typ, plane = c.Name, c.Type, c.Plane
		}
		name = cleanCell(name)
		if name == "" {
			return ""
		}
		key := strings.ToLower(name)
		if id, ok := idOf[key]; ok {
			return id
		}
		id := sanitizeID(name)
		idOf[key] = id
		cat := categoryFor(name)
		if c != nil {
			cat = categoryForType(typ, name+" "+c.Service)
		}
		catOf[id] = cat
		d.Nodes = append(d.Nodes, Node{ID: id, Label: name, Category: cat, Plane: plane})
		return id
	}

	for _, cn := range parseConnections(spec) {
		text := cn.mech + " " + cn.purpose + " " + cn.direction + " " + cn.arrow
		fromID := addNode(cn.source)
		if fromID == "" {
			continue
		}
		for _, t := range splitTargets(cn.target) {
			toID := addNode(t)
			if toID == "" || toID == fromID {
				continue
			}
			// Collapse repeated connections between the same pair (several control
			// calls Dev→EC2, plus an out-of-band status probe) into one edge.
			if seenEdge[fromID+"\x00"+toID] {
				continue
			}
			seenEdge[fromID+"\x00"+toID] = true
			// Dashed = explicit supporting text, OR either endpoint is cross-cutting
			// (an IAM governance attachment or an observability push).
			dashed := supporting(text) || isSupportCat(catOf[fromID]) || isSupportCat(catOf[toID])
			d.Edges = append(d.Edges, Edge{
				From:     fromID,
				To:       toID,
				Label:    edgeLabel(cn.mech, cn.purpose),
				Dashed:   dashed,
				Emphasis: isPrimaryEdge(cn.source, t),
			})
		}
	}

	// Cross-cutting components (IAM, observability) are frequently described in
	// Components but carry no explicit connection row. Surface them as standalone
	// badges so the diagram shows the governance/observability context.
	for i := range comps {
		c := &comps[i]
		cat := categoryForType(c.Type, c.Name+" "+c.Service)
		isCross := isSupportCat(cat) || strings.Contains(strings.ToLower(c.Plane), "cross")
		if isCross && idOf[strings.ToLower(cleanCell(c.Name))] == "" {
			addNode(c.Name)
		}
	}
	return d
}

// componentIndex resolves a connection endpoint name to a canonical component.
type componentIndex struct {
	byName    map[string]*component // canonical name and its paren-stripped form
	byService map[string]*component // only services owned by exactly one component
}

// resolveComponent finds the canonical component for a connection endpoint,
// trying the name (and its paren-stripped form) first, then a unique AWS service.
func resolveComponent(label string, idx componentIndex) *component {
	name := cleanCell(label)
	if name == "" {
		return nil
	}
	if c, ok := idx.byName[strings.ToLower(name)]; ok {
		return c
	}
	if stripped := stripParen(name); stripped != name {
		if c, ok := idx.byName[strings.ToLower(stripped)]; ok {
			return c
		}
	}
	if c, ok := idx.byService[strings.ToLower(name)]; ok {
		return c
	}
	return nil
}

// parseComponents reads the "## Components" section into components and an index.
func parseComponents(spec string) ([]component, componentIndex) {
	var comps []component
	var cur *component
	plane := ""
	inSection := false

	flush := func() {
		if cur != nil && cur.Name != "" {
			comps = append(comps, *cur)
		}
		cur = nil
	}

	for _, raw := range strings.Split(spec, "\n") {
		t := strings.TrimSpace(raw)
		if strings.HasPrefix(t, "## ") {
			flush()
			inSection = strings.EqualFold(t, "## Components")
			plane = ""
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(t, "### ") {
			flush()
			plane = cleanCell(strings.TrimPrefix(t, "### "))
			continue
		}
		indented := len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t')
		// A component header is either a standalone bold line "**Name**" (the
		// generator's form, with the fields as top-level "- Key: value" bullets
		// below it) or a legacy top-level "- **Name**" bullet. Anything else that
		// is a "- " bullet is a field of the current component.
		bulletBody := ""
		if strings.HasPrefix(t, "- ") {
			bulletBody = strings.TrimSpace(strings.TrimPrefix(t, "- "))
		}
		isBold := func(s string) bool {
			return strings.HasPrefix(s, "**") && strings.HasSuffix(s, "**") && len(s) > 4
		}
		// A leading "Name: <value>" field also declares a component — some specs
		// use it instead of a bold header, with the remaining fields as sibling
		// bullets below. Detect it so those specs still yield nodes.
		nameKey, nameVal, isNameField := "", "", false
		if bulletBody != "" {
			if k, v, ok := splitField(bulletBody); ok {
				nameKey, nameVal, isNameField = k, v, k == "name"
			}
		}
		switch {
		case !indented && isBold(t):
			if name := cleanCell(t); name != "" {
				flush()
				cur = &component{Name: name, Plane: plane}
			}
		case bulletBody != "" && isBold(bulletBody):
			if name := cleanCell(bulletBody); name != "" {
				flush()
				cur = &component{Name: name, Plane: plane}
			}
		case isNameField && nameVal != "":
			flush()
			cur = &component{Name: cleanCell(nameVal), Plane: plane}
		case cur != nil && nameKey != "":
			switch {
			case strings.HasPrefix(nameKey, "type"):
				cur.Type = nameVal
			case strings.HasPrefix(nameKey, "aws service"):
				cur.Service = nameVal
			}
		}
	}
	flush()

	idx := componentIndex{byName: map[string]*component{}, byService: map[string]*component{}}
	serviceCount := map[string]int{}
	for i := range comps {
		c := &comps[i]
		idx.byName[strings.ToLower(c.Name)] = c
		if s := stripParen(c.Name); s != c.Name {
			idx.byName[strings.ToLower(s)] = c
		}
		if svc := cleanCell(c.Service); svc != "" {
			serviceCount[strings.ToLower(svc)]++
		}
	}
	// Only index a service when a single component owns it, so ambiguous services
	// (e.g. several Amazon EC2 components) never mis-resolve a connection endpoint.
	for i := range comps {
		c := &comps[i]
		svc := strings.ToLower(cleanCell(c.Service))
		if svc != "" && serviceCount[svc] == 1 {
			idx.byService[svc] = c
		}
	}
	return comps, idx
}

// rawConn is one connection before endpoints are resolved to canonical nodes.
type rawConn struct {
	source, target   string
	mech, purpose    string
	direction, arrow string
}

// parseConnections reads the "## Connections" section in either bullet form
// ("- Source → Target" followed by indented "- Key: value" sub-bullets) or the
// legacy pipe-table form.
func parseConnections(spec string) []rawConn {
	var conns []rawConn
	var cur *rawConn
	inSection := false

	flush := func() {
		if cur != nil && cur.source != "" && cur.target != "" {
			conns = append(conns, *cur)
		}
		cur = nil
	}

	for _, raw := range strings.Split(spec, "\n") {
		t := strings.TrimSpace(raw)
		if strings.HasPrefix(t, "## ") {
			flush()
			inSection = strings.EqualFold(t, "## Connections")
			continue
		}
		if !inSection {
			continue
		}
		// Table row.
		if strings.HasPrefix(t, "|") {
			cells := splitRow(t)
			if isSeparatorRow(cells) {
				continue
			}
			// Drop a leading index column ("#", "1", ...) if present.
			if len(cells) > 0 && isIndexCell(cells[0]) {
				cells = cells[1:]
			}
			if len(cells) < 2 || strings.EqualFold(cleanCell(cells[0]), "source") {
				continue // too short, or the header row
			}
			c := rawConn{source: cleanCell(cells[0]), target: cleanCell(cells[1])}
			if len(cells) > 2 {
				c.mech = cells[2]
			}
			if len(cells) > 3 {
				c.purpose = cells[3]
			}
			if len(cells) > 4 {
				c.direction = cells[4]
			}
			// Some specs pack "A → B" into the source cell instead of two columns.
			if src, tgt, arrow, ok := splitArrow(c.source); ok {
				c.source, c.target, c.arrow = src, tgt, arrow
			}
			if c.source != "" && c.target != "" {
				conns = append(conns, c)
			}
			continue
		}
		indented := len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t')
		// Top-level bullet starts a new connection: either "- A → B" (arrow form,
		// possibly with Source:/Target: labels) or "- Source: A" (fielded form).
		if !indented && strings.HasPrefix(t, "- ") {
			body := cleanCell(strings.TrimPrefix(t, "- "))
			if src, tgt, arrow, ok := splitArrow(body); ok {
				flush()
				cur = &rawConn{source: src, target: tgt, arrow: arrow}
				continue
			}
			if lb := strings.ToLower(body); strings.HasPrefix(lb, "source:") || strings.HasPrefix(lb, "from:") {
				flush()
				cur = &rawConn{source: stripEndpointLabel(body)}
			} else {
				flush() // a non-connection top-level bullet ends the current one
			}
			continue
		}
		if cur == nil || !strings.HasPrefix(t, "- ") {
			continue
		}
		key, val, ok := splitField(strings.TrimPrefix(t, "- "))
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(key, "protocol"), strings.HasPrefix(key, "mechanism"):
			cur.mech = val
		case strings.HasPrefix(key, "purpose"):
			cur.purpose = val
		case strings.HasPrefix(key, "direction"):
			cur.direction = val
		case strings.HasPrefix(key, "target"), strings.HasPrefix(key, "to"):
			if cur.target == "" {
				cur.target = stripParen(val)
			}
		case strings.HasPrefix(key, "source"), strings.HasPrefix(key, "from"):
			if cur.source == "" {
				cur.source = stripParen(val)
			}
		}
	}
	flush()
	return conns
}

// splitArrow splits "A → B" (or ->, ⇄, ↔, ⇆, <->) into endpoints. It strips a
// trailing parenthetical qualifier ("Versioned Custom AMI (+ snapshot)" →
// "Versioned Custom AMI") and any leading "Source:"/"Target:" field label the
// model prepends ("Source: Build Driver → Target: Build Artifact Bucket").
func splitArrow(s string) (from, to, arrow string, ok bool) {
	for _, a := range []string{"→", "⇄", "↔", "⇆", "<->", "->"} {
		if i := strings.Index(s, a); i >= 0 {
			from = stripParen(stripEndpointLabel(cleanCell(s[:i])))
			to = stripParen(stripEndpointLabel(cleanCell(s[i+len(a):])))
			if from != "" && to != "" {
				return from, to, a, true
			}
		}
	}
	return "", "", "", false
}

// stripEndpointLabel removes a leading "Source:"/"Target:"/"From:"/"To:" label.
func stripEndpointLabel(s string) string {
	for _, p := range []string{"source:", "target:", "from:", "to:"} {
		if len(s) >= len(p) && strings.EqualFold(s[:len(p)], p) {
			return strings.TrimSpace(s[len(p):])
		}
	}
	return s
}

// splitField splits "Key: value" (a sub-bullet). key is lowercased and trimmed.
func splitField(s string) (key, val string, ok bool) {
	i := strings.Index(s, ":")
	if i < 0 {
		return "", "", false
	}
	return strings.ToLower(strings.TrimSpace(cleanCell(s[:i]))), cleanCell(s[i+1:]), true
}

// isIndexCell reports whether a table cell is a leading row-index ("#", "1", …)
// rather than a Source/Target name.
func isIndexCell(c string) bool {
	c = strings.TrimSpace(cleanCell(c))
	if c == "#" {
		return true
	}
	if c == "" {
		return false
	}
	for _, r := range c {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// stripParen removes a trailing "( ... )" qualifier from a component name.
func stripParen(s string) string {
	if i := strings.LastIndex(s, "("); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func splitRow(line string) []string {
	line = strings.Trim(strings.TrimSpace(line), "|")
	parts := strings.Split(line, "|")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func isSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" || strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

// splitTargets splits a target cell that names multiple components.
func splitTargets(cell string) []string {
	cell = cleanCell(cell)
	for _, sep := range []string{" / ", " and ", ", "} {
		if strings.Contains(cell, sep) {
			var out []string
			for _, p := range strings.Split(cell, sep) {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
			return out
		}
	}
	return []string{cell}
}

// supporting reports whether an edge's mechanism/purpose text marks it as a
// governance/observability/out-of-band relationship (drawn dashed). It avoids the
// generic "IAM-scoped API" qualifier that decorates nearly every call — those are
// primary data/control flows; a governance edge is instead detected by an IAM or
// observability endpoint (see the category check in Parse).
func supporting(text string) bool {
	return has(strings.ToLower(text),
		"poll", "out-of-band", "out of band", "status", "observab", "logs", "metric",
		"telemetry", "governance", "⇄", "↔")
}

// isPrimaryEdge marks the release's central relationship (the AMI → runtime EC2
// boot) so the renderer can highlight it.
func isPrimaryEdge(source, target string) bool {
	s, t := strings.ToLower(source), strings.ToLower(target)
	return has(s, "ami", "image") && has(t, "ec2", "host", "runtime", "instance")
}

func edgeLabel(mech, purpose string) string {
	label := cleanCell(mech)
	if label == "" {
		label = cleanCell(purpose)
	}
	return truncate(firstClause(label), 40)
}

// cleanCell strips Markdown emphasis/backticks from a table cell or bullet.
func cleanCell(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func firstClause(s string) string {
	for _, sep := range []string{" (", "; ", ", "} {
		if i := strings.Index(s, sep); i > 0 {
			return s[:i]
		}
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"
}

func sanitizeID(s string) string {
	var b strings.Builder
	prevUS := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUS = false
		default:
			if !prevUS && b.Len() > 0 {
				b.WriteByte('_')
				prevUS = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}
