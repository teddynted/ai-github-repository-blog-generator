package linkedin

// engagementPrompt returns a thoughtful, type-appropriate discussion prompt that
// invites meaningful technical conversation.
func engagementPrompt(c postCandidate) string {
	switch c.Type {
	case "Architecture Deep Dive":
		return "How would you approach this architecture? I'd genuinely like to hear other takes."
	case "AWS Best Practice":
		return "What AWS service would you reach for here — and why?"
	case "AI Engineering Highlight":
		return "Where are you drawing the line between AI-assisted and fully manual in your own work?"
	case "Engineering Lesson":
		return "Have you run into a similar problem? Curious how you solved it."
	case "Performance Improvement":
		return "What's the reliability win you're most proud of shipping?"
	case "Feature Spotlight", "Developer Productivity Tip":
		return "Would this fit into your workflow? What would you change?"
	case "Behind-the-Build":
		return "What would you have built differently?"
	case "Release Announcement":
		return "What would you want to see in the next release?"
	default:
		return "What would you improve?"
	}
}
