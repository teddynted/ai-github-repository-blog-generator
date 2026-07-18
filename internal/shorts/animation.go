package shorts

// animationFor returns the animation for a scene, by angle and role.
func animationFor(angle string, kind sceneKind) string {
	switch kind {
	case kindHook:
		return "Zoom"
	case kindTakeaway:
		return "Slide"
	default: // core
		switch angle {
		case "Architecture Reveal":
			return "Diagram Build"
		case "AWS Best Practice", "Optimization", "CloudFormation Tip":
			return "Highlight"
		case "Code Walkthrough", "Developer Tip":
			return "Typing"
		case "Common Mistake", "Lesson Learned":
			return "Pulse"
		case "Demo Highlight":
			return "Sequential Reveal"
		case "Interesting Statistic":
			return "Fade"
		default:
			return "Highlight"
		}
	}
}

// transitionFor returns the cut into the next scene (last scene closes on Fade).
func transitionFor(kind sceneKind, isLast bool) string {
	if isLast {
		return "Fade"
	}
	switch kind {
	case kindHook:
		return "Zoom"
	default:
		return "Slide"
	}
}
