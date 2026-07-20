package seo

import "strings"

func repoName(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.FullName != "" {
		return pkg.Context.Repository.FullName
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Repository, pkg.YouTube.Metadata.Repository, "this project")
}

func repoShortName(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.Name != "" {
		return pkg.Context.Repository.Name
	}
	full := repoName(pkg)
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[i+1:]
	}
	return full
}

func releaseTag(pkg ReleasePackage) string {
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

func awsServices(pkg ReleasePackage) []string {
	if pkg.Context == nil {
		return nil
	}
	return dedupe(pkg.Context.Architecture.AWSServices)
}

func technologies(pkg ReleasePackage) []string {
	var out []string
	if c := pkg.Context; c != nil {
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
			if isLanguage(t.Name) {
				out = append(out, t.Name)
			}
		}
	}
	return dedupe(out)
}

func isLanguage(name string) bool {
	switch strings.ToLower(name) {
	case "go", "golang", "python", "typescript", "javascript", "rust", "java", "c#", "ruby", "kotlin", "swift":
		return true
	default:
		return false
	}
}

// summary returns a grounded one-liner about the release.
func summary(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if s := firstSentences(c.ContentIntelligence.Summary, 1); s != "" {
			return s
		}
		if s := firstSentences(c.Architecture.Overview, 1); s != "" {
			return s
		}
	}
	if pkg.Blog.MetaDescription != "" {
		return firstSentences(pkg.Blog.MetaDescription, 1)
	}
	return repoShortName(pkg) + " " + releaseTag(pkg)
}

// featureName is a short, grounded name for what shipped.
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

// blogProse returns the blog body as plain prose (front matter, headings, code,
// and tables stripped) — the source for excerpts and reading time.
func blogProse(pkg ReleasePackage) string {
	md := pkg.Blog.Markdown
	if md == "" {
		return firstNonEmpty(summary(pkg), pkg.Blog.MetaDescription)
	}
	md = stripFrontMatter(md)
	lines := strings.Split(md, "\n")
	in := false
	var out []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") {
			in = !in
			continue
		}
		if in || t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "|") || strings.HasPrefix(t, ">") {
			continue
		}
		out = append(out, t)
	}
	return cleanInline(strings.Join(out, " "))
}

func stripFrontMatter(md string) string {
	s := strings.TrimLeft(md, "\n")
	if !strings.HasPrefix(s, "---\n") {
		return md
	}
	if i := strings.Index(s[4:], "\n---"); i >= 0 {
		return strings.TrimLeft(s[4+i+4:], "\n")
	}
	return md
}

func cleanInline(s string) string {
	r := strings.NewReplacer("**", "", "`", "", "__", "", "*", "", "[", "", "]", "")
	return collapse(r.Replace(s))
}

// groundedTerms is the lowercase set of terms considered grounded (repo, tag,
// AWS services, technologies, context SEO keywords). Validation checks metadata
// against it.
func groundedTerms(pkg ReleasePackage) map[string]bool {
	set := map[string]bool{}
	add := func(s string) {
		if s = strings.ToLower(collapse(s)); s != "" {
			set[s] = true
		}
	}
	add(repoShortName(pkg))
	add(releaseTag(pkg))
	for _, s := range awsServices(pkg) {
		add(s)
	}
	for _, t := range technologies(pkg) {
		add(t)
	}
	if c := pkg.Context; c != nil {
		for _, k := range c.ContentIntelligence.SEOKeywords {
			add(k)
		}
		for _, t := range c.ContentIntelligence.TechnicalHighlights {
			add(t)
		}
	}
	for _, t := range pkg.Blog.Tags {
		add(t)
	}
	return set
}
