package xthread

// engagementPrompt returns a thoughtful, type-appropriate discussion starter.
func engagementPrompt(c threadCandidate) string {
	switch c.Type {
	case "Architecture Walkthrough":
		return "What architecture would you choose for this? 👇"
	case "AWS Best Practices":
		return "What AWS service would you reach for instead — and why?"
	case "AI Engineering Insights":
		return "Where do you draw the line between AI-assisted and manual?"
	case "Engineering Lessons Learned":
		return "Have you implemented something similar? How did it go?"
	case "Performance Improvements":
		return "What's the reliability win you're most proud of?"
	case "Implementation Deep Dive", "Feature Breakdown", "Developer Tips":
		return "How would you approach this problem?"
	case "Open Source Update":
		return "What would you want to see next? Issues and PRs welcome."
	case "Release Announcement":
		return "What would you build with this?"
	default:
		return "What would you improve?"
	}
}
