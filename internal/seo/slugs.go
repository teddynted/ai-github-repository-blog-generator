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
	feat := strings.TrimSpace(firstSentences(featureName(pkg), 1))
	// A featureless (e.g. maintenance) release yields the placeholder
	// "this release"; grounding slugs in it produced junk like
	// "release-release-v0-6-0" and "aws-iam-release". Fall back to repo + AWS
	// topic + tag instead, and never repeat the word "release".
	// Slugs use the CENTRAL AWS service (the article's actual topic), not the
	// first detected service — which produced off-topic slugs like "aws-iam-...".
	central := centralAWS(awsServices(pkg), topicSignals(pkg))
	if feat == "" || strings.EqualFold(feat, "this release") {
		alts = append(alts, boundSlug(repo+" "+releaseTag(pkg)))
		if len(central) > 0 {
			alts = append(alts, boundSlug(central[0]+" "+releaseTag(pkg)))
		}
	} else {
		alts = append(alts, boundSlug(repo+" "+feat))
		if len(central) > 0 {
			alts = append(alts, boundSlug(central[0]+" "+feat))
		}
		alts = append(alts, boundSlug(feat+" "+releaseTag(pkg)))
	}

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
