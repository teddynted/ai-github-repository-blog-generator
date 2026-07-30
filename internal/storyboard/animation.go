package storyboard

// maxSceneAnimations bounds the animation steps in a single scene so a scene
// stays watchable (no exhaustive node enumeration). maxAnimHighlights bounds the
// node highlights within that budget, leaving room for the build and arrow cues.
const (
	maxSceneAnimations = 6
	maxAnimHighlights  = 3
)

// planAnimations builds a sequenced animation plan for a scene from its type and
// the diagrams/code it shows. Node-highlight animations are grounded in the
// real parsed diagram nodes (never invented) and follow the narrative path
// rather than enumerating every node. The result is capped to maxSceneAnimations.
func planAnimations(typ string, diagrams []DiagramRef, code []CodeRef, repeat bool) []Animation {
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
		// One primary diagram per scene; highlight the key nodes on the narrative
		// path, not every node. Build the diagram from scratch only the first time
		// it appears — later architecture scenes recall it and re-focus, so the
		// same graph isn't rebuilt edge by edge three times in one video.
		if len(diagrams) > 0 {
			d := diagrams[0]
			if repeat {
				add("Recall Diagram", d.Source, "Bring the existing diagram back; pan/zoom to this scene's region rather than rebuilding it.")
			} else {
				add("Diagram Build", d.Source, "Build the diagram edge by edge.")
			}
			for i, node := range d.HighlightNodes {
				if i >= maxAnimHighlights {
					break
				}
				add("Highlight Node", node, "Highlight and label the node as narration reaches it.")
			}
			add("Draw Arrow", d.Source, "Trace the primary flow between the key nodes.")
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
	// Keep scenes watchable: never emit more than maxSceneAnimations steps.
	if len(out) > maxSceneAnimations {
		out = out[:maxSceneAnimations]
	}
	return out
}
