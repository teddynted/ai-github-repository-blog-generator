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
	tags = dedupeTags(dropPlatformConcepts(dropJunk(dropGeneric(tags), pkg), pkg))
	return slugTags(conciseTags(capServiceShare(tags, pkg, blogTagMax), pkg))
}

// conciseTags shortens verbose phrase-tags to searchable taxonomy tags: a
// multi-word topic phrase is reduced to its head noun compound (its last two
// words) and a trailing plural is singularized — "boot-time UserData
// provisioning" → "userdata provisioning", "Pre-Baked Custom AMIs" →
// "custom AMI". AWS service names are kept whole ("Amazon EventBridge Scheduler"
// stays intact).
func conciseTags(tags []string, pkg ReleasePackage) []string {
	awsSet := lowerSet(awsServices(pkg))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if isAWSServiceName(t, awsSet) {
			out = append(out, t)
			continue
		}
		words := strings.Fields(t)
		if len(words) > 2 {
			words = words[len(words)-2:]
		}
		if n := len(words); n > 0 {
			last := words[n-1]
			lc := strings.ToLower(last)
			if len(last) > 3 && strings.HasSuffix(lc, "s") && !strings.HasSuffix(lc, "ss") {
				words[n-1] = last[:len(last)-1]
			}
		}
		out = append(out, strings.Join(words, " "))
	}
	return out
}

// slugTags renders tags in the conventional tag form: lowercase, hyphen-
// separated slugs (no spaces, no ellipses, no marketing capitalization), deduped.
// It formats the topic-led tag set without changing which tags are chosen.
func slugTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		s := slugify(t)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
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
	tags = dedupeTags(dropPlatformConcepts(dropJunk(dropGeneric(tags), pkg), pkg))
	return slugTags(conciseTags(capServiceShare(tags, pkg, 12), pkg))
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
