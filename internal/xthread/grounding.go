package xthread

import "strings"

func repoName(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.FullName != "" {
		return pkg.Context.Repository.FullName
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Repository, pkg.YouTube.Metadata.Repository, "this project")
}

func repoShort(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.Name != "" {
		return pkg.Context.Repository.Name
	}
	full := repoName(pkg)
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[i+1:]
	}
	return full
}

func tag(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.Tag != "" {
		return pkg.Context.Release.Tag
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Release, pkg.YouTube.Metadata.Release)
}

func repoURL(pkg ReleasePackage) string {
	if pkg.Context != nil {
		if pkg.Context.Repository.URL != "" {
			return pkg.Context.Repository.URL
		}
		if pkg.Context.Repository.FullName != "" {
			return "https://github.com/" + pkg.Context.Repository.FullName
		}
	}
	return ""
}

func blogURL(pkg ReleasePackage) string { return pkg.SEO.Blog.Canonical.URL }

func awsServices(pkg ReleasePackage) []string {
	if pkg.Context == nil {
		return nil
	}
	return dedupe(pkg.Context.Architecture.AWSServices)
}

func technologies(pkg ReleasePackage) []string {
	var out []string
	if c := pkg.Context; c != nil {
		if c.Repository.Language != "" {
			out = append(out, c.Repository.Language)
		}
		for _, t := range c.Technologies {
			out = append(out, t.Name)
		}
	}
	return dedupe(out)
}

func programmingLanguages(pkg ReleasePackage) []string {
	var out []string
	if c := pkg.Context; c != nil {
		if c.Repository.Language != "" {
			out = append(out, c.Repository.Language)
		}
		for _, t := range c.Technologies {
			switch strings.ToLower(t.Name) {
			case "go", "golang", "python", "typescript", "javascript", "rust", "java":
				out = append(out, t.Name)
			}
		}
	}
	return dedupe(out)
}

func summary(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if s := firstSentences(c.ContentIntelligence.Summary, 1); s != "" {
			return s
		}
		if s := firstSentences(c.Architecture.Overview, 1); s != "" {
			return s
		}
	}
	return firstNonEmpty(pkg.Blog.MetaDescription, repoShort(pkg)+" "+tag(pkg))
}

func featureName(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if len(c.Changelog.Features) > 0 {
			return firstSentences(c.Changelog.Features[0], 1)
		}
		if len(c.Implementation.WhatChanged) > 0 {
			return firstSentences(c.Implementation.WhatChanged[0], 1)
		}
	}
	return "this release"
}

func hasAI(pkg ReleasePackage) bool {
	terms := []string{"bedrock", "ollama", "ai ", "llm", "claude", "machine learning", "genai", "generative"}
	hay := strings.ToLower(strings.Join(append(awsServices(pkg), technologies(pkg)...), " ") + " " + summary(pkg))
	for _, t := range terms {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}

// groundedTerms is the lowercase grounded-term set used by validation.
func groundedTerms(pkg ReleasePackage) map[string]bool {
	set := map[string]bool{}
	add := func(s string) {
		if s = strings.ToLower(collapse(s)); s != "" {
			set[s] = true
		}
	}
	addAll := func(items []string) {
		for _, s := range items {
			add(s)
		}
	}
	add(repoShort(pkg))
	add(tag(pkg))
	addAll(awsServices(pkg))
	addAll(technologies(pkg))
	if c := pkg.Context; c != nil {
		addAll(c.Changelog.Features)
		ci := c.ContentIntelligence
		addAll(ci.TechnicalHighlights)
		addAll(ci.InfrastructureHighlights)
		addAll(ci.ArchitectureHighlights)
		add(ci.DeveloperValue)
		im := c.Implementation
		addAll(im.WhatChanged)
		addAll(im.TechnicalImprovements)
		addAll(im.InfrastructureImprovements)
		addAll(im.DeveloperExperience)
		add(im.WhyItMatters)
	}
	return set
}
