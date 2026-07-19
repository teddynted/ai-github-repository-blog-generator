package tiktok

// engagementPrompt returns a natural, topic-appropriate prompt that invites
// comments without disrupting the educational flow.
func engagementPrompt(t topic) string {
	switch t.Topic {
	case "AWS Tip", "CloudFormation Trick":
		return "Which AWS service would you reach for here?"
	case "Architecture Insight":
		return "Would you have designed it differently? Tell me how."
	case "GitHub Automation", "Developer Productivity":
		return "What would you automate next?"
	case "Common Mistake":
		return "Have you been caught by this one?"
	case "Code Optimization", "Performance Improvement":
		return "What's the biggest speedup you've shipped?"
	case "Best Practice":
		return "Would you use this in production?"
	case "Deployment Strategy":
		return "How do you deploy yours?"
	case "AI Workflow":
		return "Would you let AI handle this part of your workflow?"
	case "Interesting Statistic":
		return "Does that number surprise you?"
	default:
		return "Have you tried this approach?"
	}
}
