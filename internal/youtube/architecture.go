package youtube

import (
	"fmt"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// walkthroughDetail returns extra, grounded narration for chapters that warrant
// a deep walkthrough (architecture, cloudformation, repository). Every fact is
// pulled from the Release Context — architecture and structure are never
// invented. Returns "" for chapter types that don't need it.
func walkthroughDetail(chapterType string, c *rc.ReleaseContext) string {
	if c == nil {
		return ""
	}
	switch chapterType {
	case "architecture", "diagram":
		return architectureWalkthrough(c)
	case "cloudformation":
		return cloudformationWalkthrough(c)
	case "repository":
		return repositoryWalkthrough(c)
	default:
		return ""
	}
}

func architectureWalkthrough(c *rc.ReleaseContext) string {
	var b strings.Builder
	if o := firstSentences(c.Architecture.Overview, 2); o != "" {
		b.WriteString(o)
	}
	if svcs := c.Architecture.AWSServices; len(svcs) > 0 {
		fmt.Fprintf(&b, " On the AWS side we lean on %s.", joinAnd(topStrings(svcs, 4)))
	}
	for _, comp := range topComponents(c.Architecture.Components, 4) {
		fmt.Fprintf(&b, " %s handles %s.", comp.Name, lowerFirst(firstSentences(comp.Responsibility, 1)))
	}
	if len(c.Architecture.EventDrivenFlows) > 0 {
		fmt.Fprintf(&b, " The flow works like this: %s.", lowerFirst(c.Architecture.EventDrivenFlows[0]))
	}
	return collapse(b.String())
}

func cloudformationWalkthrough(c *rc.ReleaseContext) string {
	var b strings.Builder
	b.WriteString("Everything you're seeing is provisioned as code.")
	if svcs := c.Architecture.AWSServices; len(svcs) > 0 {
		fmt.Fprintf(&b, " The template stands up %s.", joinAnd(topStrings(svcs, 4)))
	}
	if c.Architecture.DeploymentTopology != "" {
		fmt.Fprintf(&b, " %s", firstSentences(c.Architecture.DeploymentTopology, 1))
	}
	if c.Architecture.Security != "" {
		fmt.Fprintf(&b, " Security-wise, %s", lowerFirst(firstSentences(c.Architecture.Security, 1)))
	}
	return collapse(b.String())
}

func repositoryWalkthrough(c *rc.ReleaseContext) string {
	var b strings.Builder
	if c.RepositoryStructure.Overview != "" {
		b.WriteString(firstSentences(c.RepositoryStructure.Overview, 1))
	} else if c.RepositoryStructure.Layout != "" {
		fmt.Fprintf(&b, "The project follows a %s layout.", c.RepositoryStructure.Layout)
	}
	for _, d := range topDirs(c.RepositoryStructure.Directories, 5) {
		if d.Responsibility != "" {
			fmt.Fprintf(&b, " %s is where %s.", d.Path, lowerFirst(firstSentences(d.Responsibility, 1)))
		}
	}
	return collapse(b.String())
}

// visualReferencesFor returns the on-screen assets for a chapter, grounded in
// the storyboard scene's real diagrams/code plus the scene's own visual.
func visualReferencesFor(chapterType string, c *rc.ReleaseContext) []string {
	var refs []string
	switch chapterType {
	case "architecture", "diagram":
		for _, d := range c.Mermaid {
			refs = append(refs, "Diagram: "+d.Source)
		}
		if len(c.Architecture.AWSServices) > 0 {
			refs = append(refs, "AWS service icons: "+strings.Join(topStrings(c.Architecture.AWSServices, 4), ", "))
		}
	case "cloudformation":
		refs = append(refs, "CloudFormation template (code editor)")
	case "repository":
		refs = append(refs, "Repository file tree", "Changed files view")
	case "implementation":
		refs = append(refs, "Source file (code editor)", "Terminal: test run")
	case "results":
		refs = append(refs, "Terminal: full CLI run", "Generated JSON + Markdown outputs")
	}
	return dedupe(refs)
}

func topComponents(in []rc.ArchitectureComponent, n int) []rc.ArchitectureComponent {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func topDirs(in []rc.DirectoryInfo, n int) []rc.DirectoryInfo {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] += 'a' - 'A'
	}
	return string(r)
}
