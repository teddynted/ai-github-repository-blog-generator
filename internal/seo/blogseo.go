package seo

import (
	"context"
	"fmt"
)

// wordsPerMinute is a standard silent reading rate for reading-time estimates.
const wordsPerMinute = 200

// planBlog assembles the blog SEO (and its cross-posts). Title, meta
// description, and excerpts may be LLM-polished; everything else is
// deterministic and grounded.
func (g *Generator) planBlog(ctx context.Context, pkg ReleasePackage, k Keywords) BlogSEO {
	title := g.planBlogTitle(ctx, pkg)
	meta := g.planMetaDescription(ctx, pkg)
	excerpts := g.planExcerpts(ctx, pkg)
	slug, altSlugs := planSlug(pkg)

	minutes := readingMinutes(pkg)
	ogTitle := title
	ogDesc := ogDescription(pkg, meta)

	return BlogSEO{
		Title:                title,
		MetaDescription:      meta,
		Keywords:             topStrings(dropJunk(allKeywords(k), pkg), 12),
		Tags:                 planBlogTags(pkg, k),
		Slug:                 slug,
		AlternativeSlugs:     altSlugs,
		Excerpt:              excerpts.Short,
		Excerpts:             excerpts,
		OpenGraphTitle:       ogTitle,
		OpenGraphDescription: ogDesc,
		Twitter: TwitterCard{
			Card: "summary_large_image", Title: truncateChars(title, 70),
			Description: truncateChars(ogDesc, TwitterDescMax),
			ImageAlt:    thumbnailAlt(pkg),
		},
		Canonical:          Canonical{URL: canonicalURL(pkg, slug), Robots: "index, follow"},
		ReadingTime:        fmt.Sprintf("%d min read", minutes),
		ReadingTimeMinutes: minutes,
		Category:           blogCategory(pkg),
		TopicClusters:      topicClusters(pkg),
		Platforms:          []string{"Blog", "Dev.to", "Medium", "Documentation"},
	}
}

func readingMinutes(pkg ReleasePackage) int {
	words := wordCount(blogProse(pkg))
	if words == 0 {
		words = pkg.Blog.WordCount
	}
	m := words / wordsPerMinute
	if m < 1 {
		m = 1
	}
	return m
}

// canonicalURL builds a canonical blog URL from the repo homepage when known,
// and otherwise emits a template placeholder ("{{site_url}}/<slug>/") so the
// publishing system always has a canonical to fill — never a blank "—".
func canonicalURL(pkg ReleasePackage, slug string) string {
	if pkg.Context != nil && pkg.Context.Repository.Homepage != "" {
		return trimSlash(pkg.Context.Repository.Homepage) + "/blog/" + slug
	}
	if slug != "" {
		return "{{site_url}}/" + slug + "/"
	}
	return ""
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// thumbnailAlt returns grounded alt text for the social image, reusing a visual
// asset thumbnail placeholder when present.
func thumbnailAlt(pkg ReleasePackage) string {
	// Evergreen, topic-based alt text — describes the content, not the repo/version.
	if f := firstSentences(featureName(pkg), 1); f != "" && !isGenericKeyword(f) {
		return f
	}
	return "AWS architecture overview"
}
