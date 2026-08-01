package seo

import "strings"

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
	return topStrings(dedupeTags(dropJunk(dropGeneric(tags), pkg)), blogTagMax)
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
	return topStrings(dedupeTags(dropJunk(dropGeneric(tags), pkg)), 15)
}

// dedupeTags removes tags that are the same term in a different format —
// "amazon-cloudwatch" vs "amazon cloudwatch" — which exact-match dedup keeps as
// two tags, wasting slots. It normalizes on lower-case + hyphen/space and keeps
// the first occurrence.
func dedupeTags(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		key := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(t, "-", " ")), " "))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}
