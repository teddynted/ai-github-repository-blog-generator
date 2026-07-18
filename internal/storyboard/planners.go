package storyboard

// planCamera returns a professional camera direction for a scene type.
func planCamera(typ string) Camera {
	switch typ {
	case "introduction":
		return Camera{Direction: "Slow Zoom In", Notes: "Ease onto the title to draw the viewer in."}
	case "problem":
		return Camera{Direction: "Focus Shift", Notes: "Rack focus from context to the problem statement."}
	case "architecture", "diagram":
		return Camera{Direction: "Diagram Focus", Notes: "Frame the diagram; move to each highlighted node."}
	case "cloudformation":
		return Camera{Direction: "Code Highlight", Notes: "Hold on the template; highlight each resource."}
	case "repository":
		return Camera{Direction: "Screen Capture", Notes: "Screen-record the repo tree and changed files."}
	case "implementation":
		return Camera{Direction: "Push In", Notes: "Push in on the key code as it is explained."}
	case "results":
		return Camera{Direction: "Pull Back", Notes: "Pull back to reveal the overall outcome."}
	case "lessons":
		return Camera{Direction: "Static", Notes: "Steady frame for guidance and takeaways."}
	case "conclusion":
		return Camera{Direction: "Slow Zoom Out", Notes: "Zoom out to close the video calmly."}
	default:
		return Camera{Direction: "Static", Notes: "Steady frame."}
	}
}

// planMood returns the music mood and sound effects for a scene type.
func planMood(typ string) (string, []string) {
	switch typ {
	case "introduction":
		return "upbeat, energetic intro", []string{"whoosh"}
	case "architecture", "diagram":
		return "calm, focused", []string{"soft click on node highlight"}
	case "cloudformation", "repository", "implementation":
		return "neutral, technical", []string{"keyboard typing"}
	case "results":
		return "uplifting", []string{"success chime"}
	case "conclusion":
		return "warm, resolving", []string{"outro swell"}
	default:
		return "neutral, technical", nil
	}
}

// planTransition returns the cut from the current scene to the next.
func planTransition(curType, nextType string, isLast bool) Transition {
	if isLast {
		return Transition{Type: "Fade", DurationSec: 1.0}
	}
	switch {
	case nextType == "diagram" || nextType == "architecture":
		return Transition{Type: "Diagram Morph", DurationSec: 0.8}
	case curType == "cloudformation" || curType == "repository":
		return Transition{Type: "Terminal Wipe", DurationSec: 0.6}
	case curType == "introduction":
		return Transition{Type: "Fade", DurationSec: 0.7}
	default:
		return Transition{Type: "Cross Dissolve", DurationSec: 0.6}
	}
}

// planAssets returns the visual assets a scene type requires.
func planAssets(typ string) []string {
	switch typ {
	case "introduction":
		return []string{"Repository Logo", "Title Card"}
	case "problem":
		return []string{"Timeline", "Flow Diagram"}
	case "architecture", "diagram":
		return []string{"Architecture Diagram", "AWS Icons"}
	case "cloudformation":
		return []string{"CloudFormation Template", "Code Editor", "AWS Icons"}
	case "repository":
		return []string{"GitHub Screenshot", "Code Editor", "Timeline"}
	case "implementation":
		return []string{"Code Editor", "Terminal Recording"}
	case "results":
		return []string{"Flow Diagram", "Timeline"}
	case "lessons":
		return []string{"Code Editor"}
	case "conclusion":
		return []string{"Repository Logo", "Title Card"}
	default:
		return []string{"Code Editor"}
	}
}
