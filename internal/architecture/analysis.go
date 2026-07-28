package architecture

import (
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// analysis is the grounded infrastructure summary the diagram builders consume.
type analysis struct {
	Services   []serviceNode // resolved AWS services (grounded)
	ByCategory map[string][]string
	Flows      []string         // event-driven flow descriptions
	Resources  []rc.CFNResource // CloudFormation resources
	Templates  []string
	Dirs       []rc.DirectoryInfo
	Mermaid    []rc.MermaidDiagram
	HasCICD    bool
	Inference  inference // grounded local/cloud AI-inference providers
}

// serviceNode is a resolved AWS service with its catalogue info.
type serviceNode struct {
	Label    string
	Category string
	Icon     string
	Color    string
	Known    bool
}

// analyze resolves the Release Context into a grounded infrastructure summary.
// It never adds a service that is not named in the context.
func analyze(pkg ReleasePackage) analysis {
	a := analysis{ByCategory: map[string][]string{}}
	c := pkg.Context
	if c == nil {
		return a
	}

	// AWS services: prefer the architecture list; fold in CloudFormation-detected
	// services (both are grounded).
	names := dedupe(append(append([]string{}, c.Architecture.AWSServices...), c.CloudFormation.Services...))
	for _, name := range names {
		info, known := lookupService(name)
		sn := serviceNode{Label: info.Canonical, Category: info.Category, Icon: info.Icon, Color: info.Color, Known: known}
		a.Services = append(a.Services, sn)
		a.ByCategory[sn.Category] = append(a.ByCategory[sn.Category], sn.Label)
	}
	// De-duplicate by canonical label.
	a.Services = uniqueServices(a.Services)

	a.Flows = dedupe(c.Architecture.EventDrivenFlows)
	a.Resources = c.CloudFormation.Resources
	a.Templates = dedupe(c.CloudFormation.Templates)
	a.Dirs = c.RepositoryStructure.Directories
	a.Mermaid = c.Mermaid
	a.HasCICD = detectCICD(c)
	a.Inference = detectInference(pkg, a) // after services are resolved
	return a
}

func uniqueServices(in []serviceNode) []serviceNode {
	seen := map[string]bool{}
	var out []serviceNode
	for _, s := range in {
		k := strings.ToLower(s.Label)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}

// detectCICD reports whether the release context has grounded CI/CD signals.
func detectCICD(c *rc.ReleaseContext) bool {
	if strings.TrimSpace(c.Documentation.DevelopmentWorkflow) != "" {
		return true
	}
	for _, f := range c.ChangedFiles {
		if strings.Contains(f.Path, ".github/workflows") || strings.HasSuffix(f.Path, ".yml") && strings.Contains(f.Path, "workflow") {
			return true
		}
	}
	for _, d := range c.RepositoryStructure.Directories {
		if strings.Contains(strings.ToLower(d.Path), ".github") {
			return true
		}
	}
	return len(c.CloudFormation.Templates) > 0
}

// categoryServices returns the grounded service labels in a category (dedup).
func (a analysis) categoryServices(cat string) []string {
	return dedupe(a.ByCategory[cat])
}

// serviceLabels returns all resolved AWS service labels.
func (a analysis) serviceLabels() []string {
	var out []string
	for _, s := range a.Services {
		out = append(out, s.Label)
	}
	return dedupe(out)
}
