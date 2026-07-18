package storyboard

import rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"

// maxHighlightNodes bounds how many nodes a single diagram highlights.
const maxHighlightNodes = 6

// planDiagrams attaches real parsed Mermaid diagrams from the Release Context to
// architecture/diagram scenes. It NEVER invents architecture: sources, types,
// and node names come straight from the parsed diagrams. Returns nil for scene
// types that don't show a diagram.
func planDiagrams(typ string, diagrams []rc.MermaidDiagram) []DiagramRef {
	if typ != "architecture" && typ != "diagram" {
		return nil
	}
	refs := make([]DiagramRef, 0, len(diagrams))
	for _, d := range diagrams {
		nodes := topStrings(d.Nodes, maxHighlightNodes)
		zoom := ""
		if len(d.Nodes) > 0 {
			zoom = d.Nodes[0]
		}
		anim := "Diagram Build"
		if d.Type == "sequence" {
			anim = "Sequential Reveal"
		}
		refs = append(refs, DiagramRef{
			Source:         d.Source,
			Type:           d.Type,
			HighlightNodes: nodes,
			Animation:      anim,
			ZoomTarget:     zoom,
		})
	}
	return refs
}
