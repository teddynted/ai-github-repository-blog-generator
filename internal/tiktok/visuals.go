package tiktok

// planVisuals returns the grounded on-screen visuals for a TikTok. Each visual's
// Reference ties back to a real artifact so validation can confirm nothing is
// invented. When adapting a Short, its grounded visuals are reused directly.
func planVisuals(t topic, pkg ReleasePackage) []Visual {
	// Reuse the source Short's grounded visuals when adapting one.
	if t.Short > 0 {
		if vs := shortVisuals(pkg, t.Short); len(vs) > 0 {
			return vs
		}
	}

	var out []Visual
	repo := repoName(pkg)
	switch t.Topic {
	case "Architecture Insight":
		if src := diagramSource(pkg); src != "" {
			out = append(out, Visual{Kind: "Mermaid Diagram", Description: "Build the architecture diagram.", Reference: src})
			out = append(out, Visual{Kind: "Architecture Animation", Description: "Animate the component flow.", Reference: src})
		}
		for _, svc := range awsServices(pkg, 3) {
			out = append(out, Visual{Kind: "AWS Console", Description: "Show " + svc + ".", Reference: svc})
		}
	case "CloudFormation Trick", "AWS Tip", "Deployment Strategy":
		out = append(out, Visual{Kind: "CloudFormation Template", Description: "Highlight the resource in the template.", Reference: repo})
		out = append(out, Visual{Kind: "AWS Console", Description: "Show the provisioned resource.", Reference: firstNonEmpty(firstService(pkg), repo)})
	case "Code Optimization", "Developer Productivity", "Common Mistake", "Performance Improvement":
		out = append(out, Visual{Kind: "Code Walkthrough", Description: "Highlight the key code.", Reference: repo})
		out = append(out, Visual{Kind: "Editor View", Description: "Show it in the editor.", Reference: repo})
	case "GitHub Automation":
		out = append(out, Visual{Kind: "Terminal Recording", Description: "Run the automation end to end.", Reference: repo})
		out = append(out, Visual{Kind: "GitHub Release Page", Description: "Show the release it came from.", Reference: releaseTag(pkg)})
	case "Interesting Statistic":
		out = append(out, Visual{Kind: "Repository Screenshot", Description: "Show the changed files as numbers pop up.", Reference: repo})
	default:
		out = append(out, Visual{Kind: "Code Walkthrough", Description: "Show the relevant snippet.", Reference: repo})
	}

	// Always end on the repo.
	out = append(out, Visual{Kind: "Repository Screenshot", Description: "End card: the repository.", Reference: firstNonEmpty(repoURL(pkg), repo)})
	return out
}

// shortVisuals adapts a source Short's visuals into TikTok visuals, preserving
// their grounded references.
func shortVisuals(pkg ReleasePackage, shortID int) []Visual {
	for _, s := range pkg.Shorts.Shorts {
		if s.ID != shortID {
			continue
		}
		out := make([]Visual, 0, len(s.Visuals))
		for _, v := range s.Visuals {
			out = append(out, Visual{Kind: mapVisualKind(v.Kind), Description: v.Description, Reference: v.Reference})
		}
		return out
	}
	return nil
}

// mapVisualKind maps a Shorts visual kind onto the closest TikTok kind.
func mapVisualKind(kind string) string {
	switch kind {
	case "Mermaid Animation":
		return "Mermaid Diagram"
	case "Architecture Diagram":
		return "Architecture Animation"
	case "Animated Callout":
		return "Editor View"
	case "Code Highlight":
		return "Code Walkthrough"
	case "GitHub Release":
		return "GitHub Release Page"
	default:
		return kind // Repository Screenshot, Terminal Recording, CloudFormation Template, AWS Console already valid
	}
}

func firstService(pkg ReleasePackage) string {
	if s := awsServices(pkg, 1); len(s) > 0 {
		return s[0]
	}
	return ""
}
