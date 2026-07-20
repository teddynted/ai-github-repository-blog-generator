package seo

import "strings"

// slugMaxWords keeps slugs readable — long slugs hurt SEO and shareability.
const slugMaxWords = 8

// planSlug builds a deterministic, SEO-friendly slug and a couple of grounded
// alternatives. The primary slug prefers the blog title; alternatives lead with
// the repo/feature/AWS angle.
func planSlug(pkg ReleasePackage) (string, []string) {
	primary := boundSlug(firstNonEmpty(pkg.Blog.Title, featureName(pkg), repoShortName(pkg)))

	var alts []string
	repo := repoShortName(pkg)
	feat := firstSentences(featureName(pkg), 1)
	alts = append(alts, boundSlug(repo+" "+feat))
	if aws := awsServices(pkg); len(aws) > 0 {
		alts = append(alts, boundSlug(aws[0]+" "+feat))
	}
	alts = append(alts, boundSlug(feat+" release "+releaseTag(pkg)))

	// De-duplicate and drop the primary from the alternatives.
	alts = dedupe(alts)
	filtered := alts[:0]
	for _, a := range alts {
		if a != "" && a != primary {
			filtered = append(filtered, a)
		}
	}
	if primary == "" {
		primary = "release-" + slugify(releaseTag(pkg))
	}
	return primary, filtered
}

// boundSlug slugifies and caps the slug to slugMaxWords words.
func boundSlug(s string) string {
	sl := slugify(s)
	parts := strings.Split(sl, "-")
	// Drop common stop words for a cleaner slug.
	kept := parts[:0]
	for _, p := range parts {
		if p == "" || stopWords[p] {
			continue
		}
		kept = append(kept, p)
	}
	if len(kept) > slugMaxWords {
		kept = kept[:slugMaxWords]
	}
	return strings.Join(kept, "-")
}

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "of": true,
	"to": true, "for": true, "in": true, "on": true, "with": true, "is": true,
	"it": true, "its": true, "this": true, "that": true, "what": true, "why": true,
}
