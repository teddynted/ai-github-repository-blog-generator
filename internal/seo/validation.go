package seo

import (
	"fmt"
	"strings"
)

// Validate checks the SEO metadata's structural invariants and returns a list of
// problems (empty when valid). It enforces the Milestone 10 rules: required
// metadata exists for every channel and is non-empty; the blog meta description
// fits its limit; the slug is valid; keywords/hashtags are deduped; and the
// metadata is grounded in the Release Context.
//
// pkg is the source package, used to confirm grounding.
func (m SEOMetadata) Validate(pkg ReleasePackage) []string {
	var problems []string

	if m.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}

	// Required, non-empty metadata per channel.
	if collapse(m.Blog.Title) == "" {
		problems = append(problems, "blog: empty title")
	}
	if collapse(m.Blog.MetaDescription) == "" {
		problems = append(problems, "blog: empty meta description")
	}
	if collapse(m.Blog.Slug) == "" {
		problems = append(problems, "blog: empty slug")
	}
	if collapse(m.YouTube.Title) == "" {
		problems = append(problems, "youtube: empty title")
	}
	if collapse(m.YouTube.Description) == "" {
		problems = append(problems, "youtube: empty description")
	}
	if len(m.Shorts.Items) == 0 {
		problems = append(problems, "shorts: no items")
	}
	if len(m.Social.Items) == 0 {
		problems = append(problems, "social: no items")
	}
	if len(m.Keywords.Primary) == 0 {
		problems = append(problems, "keywords: no primary keywords")
	}
	if m.OpenGraph.Title == "" || m.OpenGraph.Description == "" {
		problems = append(problems, "openGraph: missing title or description")
	}
	if m.StructuredData.JSONLD == nil {
		problems = append(problems, "structuredData: missing JSON-LD")
	}

	// Hard limit: the blog meta description must fit.
	if l := len(m.Blog.MetaDescription); l > BlogDescMax {
		problems = append(problems, fmt.Sprintf("blog: meta description %d chars exceeds the %d limit", l, BlogDescMax))
	}
	// Hard limit: the YouTube title must fit its platform maximum.
	if l := len(m.YouTube.Title); l > YouTubeTitleMax {
		problems = append(problems, fmt.Sprintf("youtube: title %d chars exceeds the %d limit", l, YouTubeTitleMax))
	}

	// Slug must be valid: lowercase, hyphen-separated, no spaces/underscores.
	if s := m.Blog.Slug; s != "" && !validSlug(s) {
		problems = append(problems, fmt.Sprintf("blog: invalid slug %q", s))
	}

	// Keywords and hashtags must be deduplicated.
	if hasDup(m.Keywords.Primary) {
		problems = append(problems, "keywords: duplicate primary keywords")
	}
	if hasDup(m.Hashtags.All) {
		problems = append(problems, "hashtags: duplicate hashtags")
	}

	// Primary keywords must represent search intent, not the service inventory:
	// at most half of them may be AWS service names.
	if n := len(m.Keywords.Primary); n > 0 {
		awsSet := lowerSet(awsServices(pkg))
		if c := countAWSServiceNames(m.Keywords.Primary, awsSet); c*2 > n {
			problems = append(problems, fmt.Sprintf("keywords: %d of %d primary keywords are AWS service names (max 50%%)", c, n))
		}
	}

	// Grounding: at least one primary keyword must be grounded in the release.
	grounded := groundedTerms(pkg)
	if !anyGrounded(m.Keywords.Primary, grounded) && !anyGrounded(m.Blog.Tags, grounded) {
		problems = append(problems, "keywords: no primary keyword or tag is grounded in the release context")
	}

	return problems
}

// validSlug reports whether s is a valid SEO slug.
func validSlug(s string) bool {
	if s != strings.ToLower(s) || strings.Contains(s, " ") || strings.Contains(s, "_") {
		return false
	}
	if strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") || strings.Contains(s, "--") {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

func hasDup(list []string) bool {
	seen := map[string]bool{}
	for _, s := range list {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" {
			continue
		}
		if seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
}

// anyGrounded reports whether any term (or a word within it) is in the grounded
// set.
func anyGrounded(terms []string, grounded map[string]bool) bool {
	for _, t := range terms {
		lc := strings.ToLower(collapse(t))
		if grounded[lc] {
			return true
		}
		for word := range grounded {
			if word != "" && strings.Contains(lc, word) {
				return true
			}
		}
	}
	return false
}
