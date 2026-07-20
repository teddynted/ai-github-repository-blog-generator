package architecture

// gnode is a node in the diagram graph.
type gnode struct {
	ID       string
	Label    string
	Category string
	Icon     string
	Color    string
}

// gedge is a directed relationship. It is only ever added when the relationship
// is explicit in the Release Context.
type gedge struct {
	From, To, Label string
}

// ggroup is a visual grouping (a Mermaid subgraph / DOT cluster). Grouping is
// not a relationship — it is layout only.
type ggroup struct {
	Name    string
	NodeIDs []string
}

// graph is the format-agnostic diagram model that every renderer consumes, so
// the Mermaid, Graphviz, and SVG outputs stay synchronized.
type graph struct {
	Direction string // TD | LR
	Nodes     []gnode
	Edges     []gedge
	Groups    []ggroup
}

func (g *graph) hasNode(id string) bool {
	for _, n := range g.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}

// addNode adds a node if its ID is new, returning the (possibly existing) ID.
func (g *graph) addNode(n gnode) string {
	if n.ID == "" {
		n.ID = sanitizeID(n.Label)
	}
	if g.hasNode(n.ID) {
		return n.ID
	}
	g.Nodes = append(g.Nodes, n)
	return n.ID
}

// addEdge adds a directed edge only when both endpoints already exist (so no
// relationship is invented between undefined nodes).
func (g *graph) addEdge(from, to, label string) bool {
	if from == to || !g.hasNode(from) || !g.hasNode(to) {
		return false
	}
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			return true // already present
		}
	}
	g.Edges = append(g.Edges, gedge{From: from, To: to, Label: label})
	return true
}

// addGroup records a visual grouping of existing nodes.
func (g *graph) addGroup(name string, ids []string) {
	var valid []string
	for _, id := range ids {
		if g.hasNode(id) {
			valid = append(valid, id)
		}
	}
	if len(valid) > 0 {
		g.Groups = append(g.Groups, ggroup{Name: name, NodeIDs: valid})
	}
}

// awsServiceLabels returns the canonical AWS service labels among the nodes.
func (g *graph) awsServiceLabels() []string {
	var out []string
	for _, n := range g.Nodes {
		if n.Icon != "" { // only catalogued AWS services carry an icon
			out = append(out, n.Label)
		}
	}
	return dedupe(out)
}
