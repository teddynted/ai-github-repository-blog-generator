package awsdiagram

import "strings"

// Parse builds a Diagram from an architecture-diagram-spec.md by reading its
// grounded "## Connections" table (the reliable, structured part of the spec).
// Nodes come from the Source/Target columns; edges carry the mechanism as a label
// and are dashed for supporting relationships (governance/observability/polling).
// Returns a zero-node Diagram when no connections table is present.
func Parse(spec, title string) Diagram {
	d := Diagram{Title: title}
	idOf := map[string]string{}
	addNode := func(label string) string {
		label = cleanCell(label)
		if label == "" {
			return ""
		}
		key := strings.ToLower(label)
		if id, ok := idOf[key]; ok {
			return id
		}
		id := sanitizeID(label)
		idOf[key] = id
		d.Nodes = append(d.Nodes, Node{ID: id, Label: label, Category: categoryFor(label)})
		return id
	}

	for _, row := range connectionRows(spec) {
		if len(row) < 6 {
			continue
		}
		source, target, mech, purpose, direction := row[1], row[2], row[3], row[4], row[5]
		dashed := supporting(mech + " " + purpose + " " + direction)
		fromID := addNode(source)
		if fromID == "" {
			continue
		}
		// A target may name more than one node ("AWS Lambda / Amazon EC2").
		for _, t := range splitTargets(target) {
			toID := addNode(t)
			if toID == "" || toID == fromID {
				continue
			}
			d.Edges = append(d.Edges, Edge{
				From:     fromID,
				To:       toID,
				Label:    edgeLabel(mech, purpose),
				Dashed:   dashed,
				Emphasis: isPrimaryEdge(source, t),
			})
		}
	}
	return d
}

// connectionRows returns the cells of each data row in the "## Connections" table.
func connectionRows(spec string) [][]string {
	var rows [][]string
	inSection, inTable := false, false
	for _, ln := range strings.Split(spec, "\n") {
		t := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(t, "## "):
			inSection = strings.EqualFold(t, "## Connections")
			inTable = false
			continue
		case !inSection:
			continue
		}
		if !strings.HasPrefix(t, "|") {
			if inTable {
				break // table ended
			}
			continue
		}
		cells := splitRow(t)
		// Skip the header and the |---|---| separator.
		if isSeparatorRow(cells) {
			inTable = true
			continue
		}
		if !inTable {
			continue // header row (before the separator)
		}
		rows = append(rows, cells)
	}
	return rows
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

// supporting reports whether an edge is a governance/observability/out-of-band
// relationship (drawn dashed) rather than a primary data/control flow.
func supporting(text string) bool {
	return has(strings.ToLower(text),
		"poll", "out-of-band", "out of band", "status", "observab", "logs", "metric",
		"iam", "policy", "permission", "role", "scope", "governance", "↔", "attach")
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

// cleanCell strips Markdown emphasis/backticks from a table cell.
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
