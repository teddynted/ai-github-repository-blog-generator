package voiceover

// direction bundles the deterministic voice-acting guidance for a scene type.
type direction struct {
	Direction string // narration style (Professional, Educational, ...)
	Emotion   string
	Energy    string // low | medium | high
	Pace      string // Slow | Conversational | Medium | Fast
}

// directionFor returns the voice direction for a storyboard scene type. The
// guidance keeps narration clear and human — an experienced engineer teaching a
// peer — never robotic or theatrical.
func directionFor(sceneType string) direction {
	switch sceneType {
	case "introduction":
		return direction{Direction: "Warm and inviting, confident", Emotion: "enthusiastic", Energy: "high", Pace: "Conversational"}
	case "problem":
		return direction{Direction: "Empathetic and grounded", Emotion: "concerned", Energy: "medium", Pace: "Medium"}
	case "architecture", "diagram":
		return direction{Direction: "Clear and instructive", Emotion: "focused", Energy: "medium", Pace: "Slow"}
	case "cloudformation", "repository", "implementation":
		return direction{Direction: "Precise and technical", Emotion: "confident", Energy: "medium", Pace: "Slow"}
	case "results":
		return direction{Direction: "Confident and positive", Emotion: "proud", Energy: "high", Pace: "Medium"}
	case "lessons":
		return direction{Direction: "Reflective and advisory", Emotion: "thoughtful", Energy: "medium", Pace: "Conversational"}
	case "conclusion":
		return direction{Direction: "Warm and resolving", Emotion: "satisfied", Energy: "medium", Pace: "Conversational"}
	default:
		return direction{Direction: "Professional and clear", Emotion: "neutral", Energy: "medium", Pace: "Medium"}
	}
}

// openingCue is a director's note for when to begin speaking relative to the
// scene's opening visual, so the voice never leads the picture.
func openingCue(sceneType string, isFirst bool) string {
	if isFirst {
		return "Open on the title card; begin speaking as it settles into frame."
	}
	switch sceneType {
	case "architecture", "diagram":
		return "Hold a beat as the diagram begins to build, then start narrating."
	case "cloudformation", "repository", "implementation":
		return "Let the editor come into focus, then begin the walkthrough."
	case "results":
		return "Land on the outcome visual, then deliver the payoff line."
	case "conclusion":
		return "Ease in as the closing card appears."
	default:
		return "Begin as the scene settles."
	}
}

// closingCue is a director's note for how to hand off into the transition.
func closingCue(sceneType string, isLast bool) string {
	if isLast {
		return "Slow the final line and let it breathe before the fade to black."
	}
	switch sceneType {
	case "results":
		return "Hold the confident tone through the last word, then release into the transition."
	default:
		return "Settle the last word cleanly so the transition can carry the cut."
	}
}
