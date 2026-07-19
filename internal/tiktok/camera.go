package tiktok

// sceneKind labels the narrative role of a scene within a TikTok.
type sceneKind string

const (
	kindHook     sceneKind = "hook"
	kindProblem  sceneKind = "problem"
	kindSolution sceneKind = "solution"
	kindOutro    sceneKind = "outro"
)

// cameraFor returns the camera direction for a scene, by topic and role.
func cameraFor(topicLabel string, kind sceneKind) string {
	switch kind {
	case kindHook:
		return "Push In"
	case kindProblem:
		return "Zoom"
	case kindOutro:
		return "Pull Out"
	default: // solution
		switch topicLabel {
		case "Architecture Insight":
			return "Highlight Diagram"
		case "CloudFormation Trick", "AWS Tip", "Deployment Strategy":
			return "Screen Recording"
		case "Code Optimization", "Developer Productivity", "Common Mistake", "Performance Improvement":
			return "Focus Code"
		case "GitHub Automation":
			return "Repository Tour"
		default:
			return "Pan"
		}
	}
}
