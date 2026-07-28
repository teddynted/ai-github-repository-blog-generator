package architecture

import (
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// diagramSpec is a built, grounded graph plus its type and titling seeds. The
// generator renders it into Mermaid/Graphviz/SVG.
type diagramSpec struct {
	Type        string
	MermaidType string // flowchart | sequenceDiagram
	Title       string
	Subtitle    string
	Description string
	Graph       graph
	References  []string
}

// buildDiagrams selects and builds every grounded diagram for a release. Each
// builder returns ok=false when the Release Context lacks the data to ground it,
// so no diagram is ever invented.
func buildDiagrams(pkg ReleasePackage, a analysis) []diagramSpec {
	var specs []diagramSpec
	add := func(s diagramSpec, ok bool) {
		if ok && len(s.Graph.Nodes) > 0 {
			specs = append(specs, s)
		}
	}

	add(highLevel(pkg, a))
	add(dataFlow(pkg, a))
	add(eventDriven(pkg, a))
	add(sequence(pkg, a))
	add(componentDiagram(pkg, a))
	add(cicd(pkg, a))

	// When the release grounds a hybrid AI platform (local + cloud inference),
	// give the key diagrams the curated, publication-ready section titles and split
	// the logical flow into Local vs AWS subgraphs. Applied only when hybrid, so a
	// plain infrastructure release keeps its neutral titles and no AI framing.
	if a.Inference.hybrid() {
		applyHybridFraming(specs, a)
	}
	return specs
}

// applyHybridFraming rewrites the section titles of the primary diagrams to the
// curated hybrid-AI names and adds Local/AWS subgraphs to the logical data-flow
// diagram. It only ever re-labels and groups existing, grounded nodes.
func applyHybridFraming(specs []diagramSpec, a analysis) {
	for i := range specs {
		switch specs[i].Type {
		case "Data Flow Diagram":
			specs[i].Title = "Logical Architecture — Hybrid AI Data Flow"
			specs[i].Subtitle = "Local inference (" + joinAndArch(a.Inference.Local) +
				") and cloud inference (" + joinAndArch(a.Inference.Cloud) + ")"
			applyInferenceSubgraphs(&specs[i].Graph)
		case "High-Level Architecture":
			specs[i].Title = "Deployment Architecture — AWS Integration"
		case "Component Diagram":
			specs[i].Title = "Repository Structure View"
		}
	}
}

// applyInferenceSubgraphs groups a graph's nodes into a self-hosted "Local
// Infrastructure" subgraph (local-inference runtimes) and an "AWS Cloud" subgraph
// (catalogued AWS services). Nodes that are neither stay ungrouped. Grouping is
// layout only — it never adds nodes or edges.
func applyInferenceSubgraphs(g *graph) {
	var local, cloud []string
	for _, n := range g.Nodes {
		switch {
		case matchesLocalInference(n.Label):
			local = append(local, n.ID)
		case n.Icon != "": // only catalogued AWS services carry an icon
			cloud = append(cloud, n.ID)
		}
	}
	if len(local) > 0 {
		g.addGroup("Local Infrastructure", local)
	}
	if len(cloud) > 0 {
		g.addGroup("AWS Cloud", cloud)
	}
}

// matchesLocalInference reports whether a node label names a local-inference
// runtime (Ollama, llama.cpp, …).
func matchesLocalInference(label string) bool {
	ll := strings.ToLower(label)
	for _, k := range localInferenceKeywords {
		if strings.Contains(ll, k.key) {
			return true
		}
	}
	return false
}

// highLevel groups the grounded AWS services by category. Edges are added only
// when the parsed Mermaid provides a mapping between two services.
func highLevel(pkg ReleasePackage, a analysis) (diagramSpec, bool) {
	if len(a.Services) == 0 {
		return diagramSpec{}, false
	}
	g := graph{Direction: "TD"}
	byCat := map[string][]string{}
	for _, s := range a.Services {
		id := g.addNode(gnode{ID: sanitizeID(s.Label), Label: s.Label, Category: s.Category, Icon: s.Icon, Color: s.Color})
		byCat[s.Category] = append(byCat[s.Category], id)
	}
	for _, cat := range categoryOrder {
		if ids := byCat[cat]; len(ids) > 0 {
			g.addGroup(cat, ids)
		}
	}
	// Reuse grounded service-to-service edges from the parsed diagrams.
	addServiceEdges(&g, a)

	return diagramSpec{
		Type:        "High-Level Architecture",
		MermaidType: "flowchart",
		Title:       repoShort(pkg) + " " + tag(pkg) + " — High-Level AWS Architecture",
		Subtitle:    "Services grouped by category",
		Description: "The AWS services this release uses, grouped by category. " + servicesSentence(a),
		Graph:       g,
		References:  a.serviceLabels(),
	}, true
}

// addServiceEdges maps parsed-Mermaid edges onto service nodes when both
// endpoints resolve to services already in the graph.
func addServiceEdges(g *graph, a analysis) {
	for _, d := range a.Mermaid {
		labelByID := map[string]string{}
		for _, n := range d.Nodes {
			labelByID[n] = n
		}
		for _, e := range d.Edges {
			from := resolveServiceID(g, firstNonEmpty(labelByID[e.From], e.From))
			to := resolveServiceID(g, firstNonEmpty(labelByID[e.To], e.To))
			if from != "" && to != "" {
				g.addEdge(from, to, e.Label)
			}
		}
	}
}

// mermaidLabel returns a node's human label when the diagram defines one
// ("GH" → "GitHub Release"), otherwise the node id.
func mermaidLabel(m *rc.MermaidDiagram, id string) string {
	if m.NodeLabels != nil {
		if l := m.NodeLabels[id]; l != "" {
			return l
		}
	}
	return id
}

// resolveServiceID returns the graph node ID whose service matches the label.
func resolveServiceID(g *graph, label string) string {
	info, known := lookupService(label)
	if !known {
		return ""
	}
	id := sanitizeID(info.Canonical)
	if g.hasNode(id) {
		return id
	}
	return ""
}

// dataFlow builds a graph directly from the first parsed Mermaid diagram — its
// real nodes and edges. This is the most faithful topology.
func dataFlow(pkg ReleasePackage, a analysis) (diagramSpec, bool) {
	// Choose the most service-rich diagram: among the parsed Mermaid diagrams that
	// have edges, prefer the one whose nodes resolve to the most catalogued AWS
	// services (so a descriptive pipeline wins over an abstract, cryptic flow).
	// Ties keep the earliest, so single-diagram repos are unaffected.
	var src *rc.MermaidDiagram
	bestScore := -1
	for i := range a.Mermaid {
		m := &a.Mermaid[i]
		if len(m.Nodes) == 0 || len(m.Edges) == 0 {
			continue
		}
		score := 0
		for _, n := range m.Nodes {
			if _, known := lookupService(mermaidLabel(m, n)); known {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			src = m
		}
	}
	if src == nil {
		return diagramSpec{}, false
	}
	g := graph{Direction: "LR"}
	for _, n := range src.Nodes {
		// Prefer the diagram's human label ("GH" → "GitHub Release") over the id.
		label := mermaidLabel(src, n)
		info, known := lookupService(label)
		node := gnode{ID: sanitizeID(n), Label: label}
		if known {
			node.Label = info.Canonical
			node.Category = info.Category
			node.Icon = info.Icon
			node.Color = info.Color
		}
		g.addNode(node)
	}
	for _, e := range src.Edges {
		g.addEdge(sanitizeID(e.From), sanitizeID(e.To), e.Label)
	}
	if len(g.Edges) == 0 {
		return diagramSpec{}, false
	}
	return diagramSpec{
		Type:        "Data Flow Diagram",
		MermaidType: "flowchart",
		Title:       repoShort(pkg) + " " + tag(pkg) + " — Data Flow",
		Subtitle:    "From " + src.Source,
		Description: "The data-flow topology parsed from " + src.Source + ".",
		Graph:       g,
		References:  []string{src.Source},
	}, true
}

// eventDriven builds a flow from the grounded event-driven flow descriptions.
func eventDriven(pkg ReleasePackage, a analysis) (diagramSpec, bool) {
	if len(a.Flows) == 0 {
		return diagramSpec{}, false
	}
	g := graph{Direction: "LR"}
	built := false
	for _, flow := range a.Flows {
		steps := flowSteps(flow)
		if len(steps) < 2 {
			continue
		}
		prev := ""
		for _, step := range steps {
			id := addFlowNode(&g, step)
			if prev != "" {
				g.addEdge(prev, id, "")
			}
			prev = id
			built = true
		}
	}
	if !built {
		return diagramSpec{}, false
	}
	return diagramSpec{
		Type:        "Event-Driven Architecture",
		MermaidType: "flowchart",
		Title:       repoShort(pkg) + " " + tag(pkg) + " — Event-Driven Flow",
		Subtitle:    "Grounded in the release's described flows",
		Description: "How events move through the system: " + lowerFirst(firstSentences(a.Flows[0], 1)),
		Graph:       g,
		References:  a.Flows,
	}, true
}

// sequence renders the primary event flow as a sequence diagram.
func sequence(pkg ReleasePackage, a analysis) (diagramSpec, bool) {
	var steps []string
	for _, flow := range a.Flows {
		if s := flowSteps(flow); len(s) >= 2 {
			steps = s
			break
		}
	}
	if len(steps) < 2 {
		return diagramSpec{}, false
	}
	g := graph{Direction: "LR"}
	prev := ""
	for _, step := range steps {
		id := addFlowNode(&g, step)
		if prev != "" {
			g.addEdge(prev, id, "")
		}
		prev = id
	}
	return diagramSpec{
		Type:        "Sequence Diagram",
		MermaidType: "sequenceDiagram",
		Title:       repoShort(pkg) + " " + tag(pkg) + " — Request Sequence",
		Description: "The ordered interaction between components for the primary flow.",
		Graph:       g,
		References:  a.Flows,
	}, true
}

// componentDiagram builds a containment graph: the repository contains its
// top-level directories (a factual relationship).
func componentDiagram(pkg ReleasePackage, a analysis) (diagramSpec, bool) {
	if len(a.Dirs) == 0 {
		return diagramSpec{}, false
	}
	g := graph{Direction: "TD"}
	root := g.addNode(gnode{ID: "repo", Label: repoShort(pkg)})
	for _, d := range topDirs(a.Dirs, 10) {
		id := g.addNode(gnode{ID: sanitizeID(d.Path), Label: d.Path})
		g.addEdge(root, id, "")
	}
	if len(g.Nodes) < 2 {
		return diagramSpec{}, false
	}
	return diagramSpec{
		Type:        "Component Diagram",
		MermaidType: "flowchart",
		Title:       repoShort(pkg) + " " + tag(pkg) + " — Repository Components",
		Subtitle:    "Top-level package layout",
		Description: "The repository's top-level structure and each directory's responsibility.",
		Graph:       g,
		References:  dirPaths(a.Dirs),
	}, true
}

// cicd builds a grounded CI/CD pipeline: the repo's workflow deploys the
// CloudFormation templates, which provision the AWS services.
func cicd(pkg ReleasePackage, a analysis) (diagramSpec, bool) {
	if !a.HasCICD || (len(a.Templates) == 0 && len(a.Services) == 0) {
		return diagramSpec{}, false
	}
	g := graph{Direction: "LR"}
	gh := g.addNode(gnode{ID: "github", Label: "GitHub Release"})
	wf := g.addNode(gnode{ID: "workflow", Label: "CI/CD Workflow"})
	g.addEdge(gh, wf, "triggers")

	deployTarget := wf
	if len(a.Templates) > 0 {
		cfn := g.addNode(gnode{ID: "cloudformation", Label: "AWS CloudFormation", Category: "Integration", Icon: "Management-Governance/AWS-CloudFormation", Color: categoryColor["Integration"]})
		g.addEdge(wf, cfn, "deploys")
		deployTarget = cfn
	}
	for _, s := range topServices(a.Services, 8) {
		id := g.addNode(gnode{ID: sanitizeID(s.Label), Label: s.Label, Category: s.Category, Icon: s.Icon, Color: s.Color})
		g.addEdge(deployTarget, id, "provisions")
	}
	if len(g.Edges) == 0 {
		return diagramSpec{}, false
	}
	return diagramSpec{
		Type:        "CI/CD Pipeline",
		MermaidType: "flowchart",
		Title:       repoShort(pkg) + " " + tag(pkg) + " — CI/CD & Provisioning",
		Subtitle:    "From release to provisioned infrastructure",
		Description: "How a release flows through CI/CD to provision the AWS infrastructure as code.",
		Graph:       g,
		References:  append(a.Templates, a.serviceLabels()...),
	}, true
}

// --- flow parsing helpers ---

// flowSteps extracts an ordered list of steps from a flow description. It splits
// on explicit arrow/sequence markers; otherwise it falls back to the catalogued
// services mentioned, in order — both are grounded in the flow text.
func flowSteps(flow string) []string {
	f := collapse(flow)
	for _, sep := range []string{"->", "→", "=>", " then ", " to ", " -> "} {
		if strings.Contains(f, sep) {
			var out []string
			for _, p := range strings.Split(f, sep) {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, cleanStep(p))
				}
			}
			if len(out) >= 2 {
				return out
			}
		}
	}
	// Fallback: catalogued services in mention order.
	var out []string
	seen := map[string]bool{}
	lower := strings.ToLower(f)
	type hit struct {
		idx  int
		name string
	}
	var hits []hit
	for _, entry := range catalogueByKeyLen {
		if i := strings.Index(lower, entry.key); i >= 0 && !seen[entry.info.Canonical] {
			seen[entry.info.Canonical] = true
			hits = append(hits, hit{idx: i, name: entry.info.Canonical})
		}
	}
	// order by position
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].idx < hits[j-1].idx; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	for _, h := range hits {
		out = append(out, h.name)
	}
	return out
}

func cleanStep(s string) string {
	s = strings.TrimSpace(s)
	// Prefer the canonical service name when a step names one.
	if info, known := lookupService(s); known {
		return info.Canonical
	}
	return strings.Title(collapse(s)) //nolint:staticcheck // Title is fine for short labels
}

// addFlowNode adds a node for a flow step, tagging it as a service when known.
func addFlowNode(g *graph, step string) string {
	info, known := lookupService(step)
	if known {
		return g.addNode(gnode{ID: sanitizeID(info.Canonical), Label: info.Canonical, Category: info.Category, Icon: info.Icon, Color: info.Color})
	}
	return g.addNode(gnode{ID: sanitizeID(step), Label: step})
}

func servicesSentence(a analysis) string {
	labels := a.serviceLabels()
	if len(labels) == 0 {
		return ""
	}
	return "It uses " + joinAndArch(topStrings(labels, 6)) + "."
}

func joinAndArch(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func topServices(in []serviceNode, n int) []serviceNode {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func topDirs(in []rc.DirectoryInfo, n int) []rc.DirectoryInfo {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func dirPaths(in []rc.DirectoryInfo) []string {
	var out []string
	for _, d := range in {
		out = append(out, d.Path)
	}
	return out
}

func repoShort(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.Name != "" {
		return pkg.Context.Repository.Name
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Repository, "project")
}

func tag(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.Tag != "" {
		return pkg.Context.Release.Tag
	}
	return pkg.Storyboard.Metadata.Release
}
