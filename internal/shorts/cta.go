package shorts

// planCTA returns a brief call to action for a Short. Shorts drive viewers to
// the long-form video and the repo, so the CTA rotates by index to stay fresh
// across a batch while remaining concise.
func planCTA(c candidate, index int) string {
	options := []string{
		"Watch the full breakdown — link in the description.",
		"Full video on the channel. Follow for more.",
		"Star the repo — link below.",
		"Read the deep dive — link in bio.",
	}
	switch c.Angle {
	case "Demo Highlight", "Code Walkthrough":
		return "See the full run in the complete video — link below."
	case "Architecture Reveal":
		return "The full architecture walkthrough is on the channel."
	default:
		return options[index%len(options)]
	}
}
