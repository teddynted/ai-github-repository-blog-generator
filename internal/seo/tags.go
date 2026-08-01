package seo

import "strings"

// blogTagMax caps blog tags (Dev.to allows 4; Medium 5) — keep it tight.
const blogTagMax = 6

// planBlogTags returns readable blog tags, grounded and deduped. It reuses the
// blog's own tags, then fills from the keyword taxonomy.
func planBlogTags(pkg ReleasePackage, k Keywords) []string {
	// Topic tags first (primary + the blog's own tags), then technologies, then
	// AWS services — so the article topic leads and services fill.
	var tags []string
	tags = append(tags, k.Primary...)
	tags = append(tags, pkg.Blog.Tags...)
	tags = append(tags, k.Technology...)
	tags = append(tags, k.AWS...)
	tags = dedupeTags(dropJunk(dropGeneric(tags), pkg))
	return capServiceShare(tags, pkg, blogTagMax)
}

// planYouTubeTags returns YouTube tags (reusing the YouTube script's suggested
// tags, filled from keywords), deduped and capped for the ~500-char tag budget.
func planYouTubeTags(pkg ReleasePackage, k Keywords) []string {
	var tags []string
	// Topic first (primary), then the video's own suggested tags and technical
	// terms, then AWS services, then evergreen developer tags.
	tags = append(tags, k.Primary...)
	tags = append(tags, pkg.YouTube.ContentIntelligence.SuggestedTags...)
	tags = append(tags, k.Technical...)
	tags = append(tags, k.AWS...)
	tags = append(tags, k.Developer...)
	tags = dedupeTags(dropJunk(dropGeneric(tags), pkg))
	return capServiceShare(tags, pkg, 15)
}

// capServiceShare caps the list at max, keeping topic (non-service) tags freely
// but admitting an AWS service tag only while service names stay at or below half
// of the result — so at least half of the tags describe the article topic, not
// the service inventory (order preserved).
func capServiceShare(tags []string, pkg ReleasePackage, max int) []string {
	awsSet := lowerSet(awsServices(pkg))
	out := make([]string, 0, max)
	for _, t := range tags {
		if len(out) >= max {
			break
		}
		if isAWSServiceName(t, awsSet) && (countAWSServiceNames(out, awsSet)+1)*2 > len(out)+1 {
			continue
		}
		out = append(out, t)
	}
	// A service-only release yields no topic tags to anchor the cap — fall back to
	// the (service) tags rather than emitting none.
	if len(out) == 0 {
		return topStrings(tags, max)
	}
	return out
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
