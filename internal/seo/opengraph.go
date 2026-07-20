package seo

// planOpenGraph builds the Open Graph + Twitter Card metadata from the assembled
// blog SEO. Deterministic and grounded.
func planOpenGraph(pkg ReleasePackage, blog BlogSEO) OpenGraph {
	return OpenGraph{
		Title:         blog.OpenGraphTitle,
		Description:   blog.OpenGraphDescription,
		Type:          "article",
		SiteName:      repoShortName(pkg),
		URL:           blog.Canonical.URL,
		Locale:        "en_US",
		ImageGuidance: "1200x630 social card; use the generated Blog Header / GitHub Social Card visual asset. Alt: " + thumbnailAlt(pkg),
		Twitter:       blog.Twitter,
	}
}
