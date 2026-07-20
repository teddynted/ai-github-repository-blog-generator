package seo

// blogTagMax caps blog tags (Dev.to allows 4; Medium 5) — keep it tight.
const blogTagMax = 6

// planBlogTags returns readable blog tags, grounded and deduped. It reuses the
// blog's own tags, then fills from the keyword taxonomy.
func planBlogTags(pkg ReleasePackage, k Keywords) []string {
	var tags []string
	tags = append(tags, pkg.Blog.Tags...)
	tags = append(tags, k.Primary...)
	tags = append(tags, k.AWS...)
	tags = append(tags, k.Technology...)
	return topStrings(dedupe(tags), blogTagMax)
}

// planYouTubeTags returns YouTube tags (reusing the YouTube script's suggested
// tags, filled from keywords), deduped and capped for the ~500-char tag budget.
func planYouTubeTags(pkg ReleasePackage, k Keywords) []string {
	var tags []string
	tags = append(tags, pkg.YouTube.ContentIntelligence.SuggestedTags...)
	tags = append(tags, k.Primary...)
	tags = append(tags, k.AWS...)
	tags = append(tags, k.Technical...)
	tags = append(tags, k.Developer...)
	return topStrings(dedupe(tags), 15)
}
