package youtube

import (
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
)

// planDemonstration returns on-screen demonstration steps for a chapter, chosen
// by scene type and grounded in the storyboard scene's real code/diagram
// references. Non-demo scene types return nil (avoiding forced demos).
func planDemonstration(sc storyboard.Scene) []DemoStep {
	var steps []DemoStep
	add := func(action, detail string) {
		steps = append(steps, DemoStep{Step: len(steps) + 1, Action: action, Detail: detail})
	}

	switch sc.Type {
	case "repository":
		add("Open the repository", "Show the top-level layout: cmd/, internal/, docs/.")
		add("Navigate the packages", "Walk the folder tree and call out the Clean Architecture boundaries.")
		add("Show the changed files", "Highlight what this release actually touched.")
	case "cloudformation":
		add("Open the CloudFormation template", "Scroll the template and point out each resource.")
		add("Trace a resource", "Follow one resource from definition to its role in the flow.")
	case "implementation":
		add("Open the key source file", "Show the entry point and the interface it sits behind.")
		for _, code := range sc.Code {
			add("Walk the "+code.Language+" code", code.Instruction+".")
		}
		add("Run the tests", "Show the unit tests passing for this package.")
	case "architecture", "diagram":
		for _, d := range sc.Diagrams {
			add("Reveal the diagram", "Build "+d.Source+" node by node as you narrate.")
		}
	case "results":
		add("Run the CLI end to end", "Generate the release context, blog, storyboard, and narration in sequence.")
		add("Show the outputs", "Open the generated JSON and Markdown side by side.")
	}
	return steps
}
