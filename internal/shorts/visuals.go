package shorts

// planVisuals returns the grounded on-screen visuals for a Short. Each visual's
// Reference ties back to a real artifact (a Mermaid source, the repo, an AWS
// service, a chapter) so validation can confirm nothing is invented.
func planVisuals(c candidate, pkg ReleasePackage) []Visual {
	var out []Visual
	repo := repoName(pkg)

	switch c.Angle {
	case "Architecture Reveal":
		if src := diagramSource(pkg); src != "" {
			out = append(out, Visual{Kind: "Mermaid Animation", Description: "Build the architecture diagram node by node.", Reference: src})
			out = append(out, Visual{Kind: "Architecture Diagram", Description: "Full architecture on screen.", Reference: src})
		}
		for _, svc := range awsServices(pkg, 3) {
			out = append(out, Visual{Kind: "Animated Callout", Description: "Highlight " + svc + ".", Reference: svc})
		}
	case "CloudFormation Tip":
		out = append(out, Visual{Kind: "CloudFormation Template", Description: "Scroll the template; highlight the resource.", Reference: repo})
		out = append(out, Visual{Kind: "AWS Console", Description: "Show the provisioned resource.", Reference: firstNonEmpty(firstService(pkg), repo)})
	case "Code Walkthrough":
		out = append(out, Visual{Kind: "Code Highlight", Description: "Highlight the key function.", Reference: repo})
		out = append(out, Visual{Kind: "Terminal Recording", Description: "Run the tests; show them pass.", Reference: repo})
	case "Demo Highlight":
		out = append(out, Visual{Kind: "Terminal Recording", Description: "Run the CLI end to end.", Reference: repo})
		out = append(out, Visual{Kind: "GitHub Release", Description: "Show the release the output came from.", Reference: releaseTag(pkg)})
	case "Interesting Statistic":
		out = append(out, Visual{Kind: "Animated Callout", Description: "Count the numbers up on screen.", Reference: repo})
		out = append(out, Visual{Kind: "Repository Screenshot", Description: "Show the changed files.", Reference: repo})
	case "AWS Best Practice", "Optimization":
		if src := diagramSource(pkg); src != "" {
			out = append(out, Visual{Kind: "Architecture Diagram", Description: "Point to where it applies.", Reference: src})
		}
		out = append(out, Visual{Kind: "Animated Callout", Description: "Callout the practice.", Reference: repo})
	default: // Developer Tip, Lesson Learned, Common Mistake
		out = append(out, Visual{Kind: "Code Highlight", Description: "Show the relevant snippet.", Reference: repo})
		out = append(out, Visual{Kind: "Animated Callout", Description: "Reinforce the point on screen.", Reference: repo})
	}

	// Always close on the repo so viewers can find it.
	out = append(out, Visual{Kind: "Repository Screenshot", Description: "End card: the repository.", Reference: firstNonEmpty(repoURL(pkg), repo)})
	return out
}

func diagramSource(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		for _, d := range c.Mermaid {
			if d.Source != "" {
				return d.Source
			}
		}
	}
	for _, sc := range pkg.Storyboard.Scenes {
		for _, d := range sc.Diagrams {
			if d.Source != "" {
				return d.Source
			}
		}
	}
	return ""
}

func awsServices(pkg ReleasePackage, n int) []string {
	if pkg.Context == nil {
		return nil
	}
	return topStrings(pkg.Context.Architecture.AWSServices, n)
}

func firstService(pkg ReleasePackage) string {
	if s := awsServices(pkg, 1); len(s) > 0 {
		return s[0]
	}
	return ""
}
