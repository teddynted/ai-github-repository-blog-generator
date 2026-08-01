package linkedin

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

func blogURL(pkg ReleasePackage) string {
	return pkg.SEO.Blog.Canonical.URL
}

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
	// Prefer the release's own summary, but SKIP low-value auto-generated
	// release-stats framing ("delivers N analyzed changes (0 features, 0 fixes)",
	// "just maintenance changes") — that frames the release by counts, not by the
	// engineering value. The blog meta description (the article's topic-led
	// description) is the better frame in that case.
	if c := pkg.Context; c != nil {
		if s := firstSentences(c.ContentIntelligence.Summary, 1); s != "" && !isReleaseStatsFraming(s) {
			return s
		}
	}
	if d := firstSentences(pkg.Blog.MetaDescription, 1); d != "" {
		return d
	}
	if c := pkg.Context; c != nil {
		if s := firstSentences(c.Architecture.Overview, 1); s != "" && !isReleaseStatsFraming(s) {
			return s
		}
	}
	return repoShort(pkg) + " " + tag(pkg)
}

// isReleaseStatsFraming flags low-value, count-based auto-generated release
// summaries so they never frame a LinkedIn post.
func isReleaseStatsFraming(s string) bool {
	lc := strings.ToLower(s)
	for _, m := range []string{
		"0 features", "0 fixes", "analyzed changes", "maintenance changes",
		"no new user-facing", "just maintenance",
	} {
		if strings.Contains(lc, m) {
			return true
		}
	}
	return false
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

// groundedTerms is the lowercase set of terms considered grounded (repo, tag,
// AWS services, technologies, features, highlights). Validation uses it.
func groundedTerms(pkg ReleasePackage) map[string]bool {
	set := map[string]bool{}
	add := func(s string) {
		if s = strings.ToLower(collapse(s)); s != "" {
			set[s] = true
		}
	}
	add(repoShort(pkg))
	add(tag(pkg))
	for _, s := range awsServices(pkg) {
		add(s)
	}
	for _, t := range technologies(pkg) {
		add(t)
	}
	if c := pkg.Context; c != nil {
		for _, f := range c.Changelog.Features {
			add(f)
		}
		ci := c.ContentIntelligence
		addAll(add, ci.TechnicalHighlights)
		addAll(add, ci.InfrastructureHighlights)
		addAll(add, ci.ArchitectureHighlights)
		add(ci.DeveloperValue)
		im := c.Implementation
		addAll(add, im.WhatChanged)
		addAll(add, im.TechnicalImprovements)
		addAll(add, im.InfrastructureImprovements)
		addAll(add, im.DeveloperExperience)
		add(im.WhyItMatters)
	}
	return set
}

func addAll(add func(string), items []string) {
	for _, s := range items {
		add(s)
	}
}
