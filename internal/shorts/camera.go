package shorts

// sceneKind labels the narrative role of a scene within a Short.
type sceneKind string

const (
	kindHook     sceneKind = "hook"
	kindCore     sceneKind = "core"
	kindTakeaway sceneKind = "takeaway"
)

// cameraFor returns the camera direction for a scene, by angle and role.
func cameraFor(angle string, kind sceneKind) string {
	switch kind {
	case kindHook:
		return "Zoom In"
	case kindTakeaway:
		return "Zoom Out"
	default: // core
		switch angle {
		case "Architecture Reveal", "AWS Best Practice", "Optimization":
			return "Diagram Focus"
		case "CloudFormation Tip":
			return "Highlight Resource"
		case "Code Walkthrough", "Developer Tip", "Common Mistake", "Lesson Learned":
			return "Code Focus"
		case "Demo Highlight":
			return "Terminal Focus"
		case "Interesting Statistic":
			return "Screen Recording"
		default:
			return "Pan"
		}
	}
}
