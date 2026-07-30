package storyboard

import rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"

// maxHighlightNodes bounds how many nodes a single diagram highlights.
const maxHighlightNodes = 6

// planDiagrams attaches the release's own parsed Mermaid diagram to
// architecture/diagram scenes. A scene shows at most ONE primary diagram, so a
// single scene never explodes into dozens of diagram builds. It NEVER invents
// architecture: source, type, and node names come straight from the parsed
// diagram. Returns nil for scene types that don't show a diagram, or when the
// release has no diagram.
func planDiagrams(typ string, diagrams []rc.MermaidDiagram) []DiagramRef {
	if typ != "architecture" && typ != "diagram" {
		return nil
	}
	if len(diagrams) == 0 {
		return nil
	}
	d := diagrams[0]
	nodes := topStrings(d.Nodes, maxHighlightNodes)
	zoom := ""
	if len(d.Nodes) > 0 {
		zoom = d.Nodes[0]
	}
	anim := "Diagram Build"
	if d.Type == "sequence" {
		anim = "Sequential Reveal"
	}
	return []DiagramRef{{
		Source:         d.Source,
		Type:           d.Type,
		HighlightNodes: nodes,
		Animation:      anim,
		ZoomTarget:     zoom,
	}}
}
