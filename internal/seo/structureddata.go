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
		"keywords":       strings.Join(topStrings(dropJunk(allKeywords(k), pkg), 12), ", "),
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
		"about":         aboutEntities(k, centralAWS(k.AWS, topicSignals(pkg)), topicClusters(pkg)),
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

// aboutEntities is the schema.org "about" set: the article's concepts. It leads
// with the topic (primary keywords), names only the CENTRAL AWS entities — the
// services the article is actually about — and adds the domain topic clusters as
// concept-level entities. It never emits the full detected dependency inventory
// (go.mod, SDK imports, CloudFormation resources).
func aboutEntities(k Keywords, central, clusters []string) []string {
	var out []string
	out = append(out, topStrings(k.Primary, 3)...)
	out = append(out, topStrings(central, 2)...)
	out = append(out, topStrings(clusters, 2)...)
	return topStrings(dedupe(out), 6)
}

func publishedDate(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.PublishedAt != "" {
		return pkg.Context.Release.PublishedAt
	}
	return ""
}
