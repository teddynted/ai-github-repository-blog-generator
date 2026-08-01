package seo

import "strings"

// planStructuredData builds machine-readable metadata: a Schema.org TechArticle
// JSON-LD object, an RSS item, and a sitemap entry. All values are grounded in
// the assembled SEO and the Release Context.
func planStructuredData(pkg ReleasePackage, blog BlogSEO, k Keywords, generatedAt string) StructuredData {
	url := blog.Canonical.URL
	published := publishedDate(pkg)

	jsonld := map[string]interface{}{
		"@context":       "https://schema.org",
		"@type":          "TechArticle",
		"headline":       blog.Title,
		"description":    blog.MetaDescription,
		"keywords":       strings.Join(topStrings(allKeywords(k), 12), ", "),
		"articleSection": blog.Category,
		"inLanguage":     "en",
		"wordCount":      wordCount(blogProse(pkg)),
		"author": map[string]interface{}{
			"@type": "Organization",
			"name":  repoShortName(pkg),
		},
		"publisher": map[string]interface{}{
			"@type": "Organization",
			"name":  repoShortName(pkg),
		},
		"about":         aboutEntities(k),
		"datePublished": firstNonEmpty(published, generatedAt),
		"dateModified":  generatedAt,
	}
	if url != "" {
		jsonld["url"] = url
		// mainEntityOfPage should be a WebPage object, not a bare URL string
		// (schema.org best practice; some validators warn on the string form).
		jsonld["mainEntityOfPage"] = map[string]interface{}{
			"@type": "WebPage",
			"@id":   url,
		}
	}

	rss := RSSItem{
		Title:       blog.Title,
		Link:        firstNonEmpty(url, repoURL(pkg)),
		Description: firstNonEmpty(blog.Excerpt, blog.MetaDescription),
		Categories:  topStrings(blog.Tags, 5),
		PubDate:     firstNonEmpty(published, generatedAt),
		GUID:        firstNonEmpty(url, repoURL(pkg)+"/releases/tag/"+releaseTag(pkg)),
	}

	sitemap := SitemapEntry{
		Loc:        url,
		ChangeFreq: "monthly",
		Priority:   "0.8",
		LastMod:    generatedAt,
	}

	return StructuredData{SchemaType: "TechArticle", JSONLD: jsonld, RSS: rss, Sitemap: sitemap}
}

// aboutEntities is the schema.org "about" set: what the article is about. It
// leads with the topic (primary keywords) and then names the concrete AWS
// service entities from the architecture, so "about" represents both the subject
// and its supporting entities — not a raw service inventory.
func aboutEntities(k Keywords) []string {
	var out []string
	out = append(out, topStrings(k.Primary, 3)...)
	out = append(out, topStrings(k.AWS, 3)...)
	return topStrings(dedupe(out), 5)
}

func publishedDate(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.PublishedAt != "" {
		return pkg.Context.Release.PublishedAt
	}
	return ""
}
