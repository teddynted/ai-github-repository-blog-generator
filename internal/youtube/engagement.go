package youtube

// engagementEvery controls how often a viewer-engagement prompt is inserted:
// one every N body chapters, so prompts feel natural rather than constant.
const engagementEvery = 2

// planEngagement returns viewer-engagement prompts for a chapter. To avoid
// excessive interruptions, prompts are attached only to some chapters (see
// engagementEvery) and never to the intro/conclusion framing chapters.
func planEngagement(chapterType string, index int) []string {
	if chapterType == "introduction" || chapterType == "conclusion" {
		return nil
	}
	if index%engagementEvery != 0 {
		return nil
	}
	switch chapterType {
	case "architecture", "diagram":
		return []string{"Pause here and try to predict how the components talk to each other before I reveal it."}
	case "cloudformation":
		return []string{"How would you have structured this stack? Drop your approach in the comments."}
	case "repository":
		return []string{"Take a second to explore the repo yourself — the link is in the description."}
	case "implementation":
		return []string{"If you'd implement this differently, I'd genuinely like to hear it below."}
	case "results":
		return []string{"What would you measure to prove this is an improvement? Let me know."}
	case "lessons":
		return []string{"Which of these have you run into on your own projects?"}
	default:
		return []string{"Let me know in the comments if this maps to something you're building."}
	}
}
