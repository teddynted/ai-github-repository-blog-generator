package seo

import (
	"context"
	"fmt"
	"strings"
)

// blogTitleMax is the SEO-preferred blog title length (search snippets truncate
// beyond this).
const blogTitleMax = 60

// planBlogTitle returns the blog SEO title. It prefers the already-generated
// blog title, optionally polished by the Model for search, and bounded to the
// SEO length. Grounded — the fallback is the existing title.
func (g *Generator) planBlogTitle(ctx context.Context, pkg ReleasePackage) string {
	base := firstNonEmpty(pkg.Blog.Title, pkg.YouTube.ContentIntelligence.SuggestedTitle,
		repoShortName(pkg)+" "+releaseTag(pkg))
	title := base
	if g.Model != nil {
		if out, err := g.Model.Generate(ctx, blogTitlePrompt(base, releaseTag(pkg))); err == nil {
			if r := collapse(strings.TrimSpace(out)); r != "" {
				title = r
			}
		}
	}
	return boundTitle(title, blogTitleMax, base)
}

func blogTitlePrompt(base, tag string) string {
	return fmt.Sprintf(
		"Rewrite this technical blog title to be SEO-optimized and click-worthy (NOT clickbait), under 60 characters, "+
			"keeping it accurate to the release %s. Use ONLY the facts in the title — invent nothing. Output only the title.\n\nTITLE: %s",
		tag, base)
}

// boundTitle caps a title to max characters, falling back to a bounded base when
// the candidate is empty or too long to salvage.
func boundTitle(title string, max int, base string) string {
	title = collapse(title)
	if title == "" {
		title = collapse(base)
	}
	if len(title) <= max {
		return title
	}
	return truncateChars(title, max)
}

// blogCategory classifies the release into a content category.
func blogCategory(pkg ReleasePackage) string {
	if len(awsServices(pkg)) > 0 {
		return "Cloud & DevOps"
	}
	if c := pkg.Context; c != nil && len(c.Architecture.Components) > 0 {
		return "Software Architecture"
	}
	return "Software Engineering"
}

// topicClusters groups the release into SEO topic clusters (grounded).
func topicClusters(pkg ReleasePackage) []string {
	var out []string
	if len(awsServices(pkg)) > 0 {
		out = append(out, "AWS & Cloud Infrastructure")
	}
	out = append(out, "AI-Powered Content Generation", "Clean Architecture in Go", "GitHub Release Automation")
	return topStrings(dedupe(out), 5)
}
