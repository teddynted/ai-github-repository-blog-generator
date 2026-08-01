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
		"about":         aboutEntities(k, centralAWS(k.AWS, topicSignals(pkg)), topicClusters(pkg), pkg),
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

// aboutEntities is the schema.org "about" set: the article's concepts. It is
// TOPIC-HEAVY — led by the primary keywords (the article's search intent) — then
// names one central AWS entity (a service the article is actually about) and one
// domain cluster. It never emits the full detected dependency inventory (go.mod,
// SDK imports, CloudFormation resources) or supporting platform concepts, and
// stays semantically aligned with the title.
func aboutEntities(k Keywords, central, clusters []string, pkg ReleasePackage) []string {
	var out []string
	out = append(out, topStrings(k.Primary, 4)...)
	out = append(out, topStrings(central, 1)...)
	out = append(out, topStrings(clusters, 1)...)
	// Guard: never let a supporting platform concept describe the article.
	out = dropNoisyAbout(dedupe(out), pkg)
	return topStrings(out, 6)
}

// dropNoisyAbout removes junk and supporting platform concepts from the "about"
// set (topic clusters, unlike primary keywords, can legitimately be multi-word
// domain labels, so only platform-concept/junk terms are filtered — not the
// generic-token rules that apply to primary).
func dropNoisyAbout(in []string, pkg ReleasePackage) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if isJunkKeyword(s, pkg) || isPlatformConcept(s, pkg) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func publishedDate(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.PublishedAt != "" {
		return pkg.Context.Release.PublishedAt
	}
	return ""
}
