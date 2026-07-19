package tiktok

// animationFor returns the animation for a scene, by topic and role.
func animationFor(topicLabel string, kind sceneKind) string {
	switch kind {
	case kindHook:
		return "Zoom Effects"
	case kindProblem:
		return "Pulse"
	case kindOutro:
		return "Slide"
	default: // solution
		switch topicLabel {
		case "Architecture Insight":
			return "Diagram Build"
		case "CloudFormation Trick", "AWS Tip", "Best Practice", "Deployment Strategy":
			return "Callout Popups"
		case "Code Optimization", "Developer Productivity", "Performance Improvement":
			return "Typing Animation"
		case "Common Mistake":
			return "Arrow Highlights"
		case "GitHub Automation":
			return "Code Highlight"
		default:
			return "Fade"
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
