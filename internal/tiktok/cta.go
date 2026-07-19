package tiktok

// planCTA returns a short, relevant call to action. TikTok drives viewers to the
// long-form video, the blog, and the repo; CTAs rotate to stay fresh across a
// batch.
func planCTA(t topic, index int) string {
	options := []string{
		"Full breakdown on YouTube — link in bio.",
		"Repo's in the description. Go build it.",
		"Read the deep-dive blog — link in bio.",
		"Follow for more AI engineering.",
	}
	switch t.Topic {
	case "GitHub Automation", "Developer Productivity":
		return "Grab the repo from my bio and try it."
	case "Architecture Insight":
		return "Full architecture walkthrough on YouTube — link in bio."
	default:
		return options[index%len(options)]
	}
}
