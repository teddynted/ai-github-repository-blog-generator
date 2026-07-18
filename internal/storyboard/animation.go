package storyboard

// planAnimations builds a sequenced animation plan for a scene from its type and
// the diagrams/code it shows. Node-highlight animations are grounded in the
// real parsed diagram nodes (never invented).
func planAnimations(typ string, diagrams []DiagramRef, code []CodeRef) []Animation {
	var out []Animation
	seq := 1
	add := func(t, target, notes string) {
		out = append(out, Animation{Type: t, Target: target, Sequence: seq, Notes: notes})
		seq++
	}

	add("Fade In", "scene", "")

	switch typ {
	case "introduction":
		add("Scale Up", "title", "Title card scales up into place.")
	case "architecture", "diagram":
		for _, d := range diagrams {
			add("Diagram Build", d.Source, "Build the diagram edge by edge.")
			for _, node := range d.HighlightNodes {
				add("Highlight Node", node, "Highlight and label the node as narration reaches it.")
			}
			add("Draw Arrow", d.Source, "Trace the data/control flow between nodes.")
		}
	case "cloudformation":
		add("Sequential Reveal", "template", "Reveal the template section by section.")
		add("Code Typing", "yaml", "Type/scroll through the resource definitions.")
	case "repository":
		add("Sequential Reveal", "file tree", "Reveal changed files one by one.")
	case "implementation":
		add("Code Typing", "code", "Type the key implementation code.")
	case "results":
		add("Pulse", "metric", "Pulse the headline metric/outcome.")
	case "conclusion":
		add("Fade Out", "scene", "Fade out to the outro.")
	}

	// Any scene that carries code gets a code cue if it didn't already.
	if len(code) > 0 && typ != "cloudformation" && typ != "implementation" {
		add("Highlight", "code", "Highlight the referenced snippet.")
	}
	return out
}
