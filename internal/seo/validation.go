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

	// Duplicate chapter timestamps are a hard failure (a chapter list with two
	// "02:40" markers is broken metadata, not a warning).
	if hasDuplicateChapters(m.YouTube.ChapterTitles) {
		problems = append(problems, "youtube: duplicate chapter timestamps")
	}

	// Tags must be topic-led: no more than half may be AWS service names.
	awsTagSet := lowerSet(awsServices(pkg))
	if n := len(m.Blog.Tags); n > 0 && countAWSServiceNames(m.Blog.Tags, awsTagSet)*2 > n {
		problems = append(problems, "blog: more than 50% of tags are AWS service names")
	}
	if n := len(m.YouTube.Tags); n > 0 && countAWSServiceNames(m.YouTube.Tags, awsTagSet)*2 > n {
		problems = append(problems, "youtube: more than 50% of tags are AWS service names")
	}

	// Primary keywords must represent search intent, not the service inventory:
	// at most half of them may be AWS service names.
	if n := len(m.Keywords.Primary); n > 0 {
		awsSet := lowerSet(awsServices(pkg))
		if c := countAWSServiceNames(m.Keywords.Primary, awsSet); c*2 > n {
			problems = append(problems, fmt.Sprintf("keywords: %d of %d primary keywords are AWS service names (max 50%%)", c, n))
		}
	}

	// Primary keywords must not be generic/repo/version/SDK tokens.
	for _, kw := range m.Keywords.Primary {
		if isNoisyPrimaryKeyword(kw, pkg) {
			problems = append(problems, fmt.Sprintf("keywords: primary keyword %q is a generic/repo/version/SDK token", kw))
			break
		}
	}

	// JSON-LD "about" must share the article's topic: at least one entry must
	// overlap a significant word in the blog title.
	if about := jsonldAbout(m.StructuredData.JSONLD); len(about) > 0 && collapse(m.Blog.Title) != "" {
		if !sharesToken(about, m.Blog.Title) {
			problems = append(problems, "structuredData: about shares no topic term with the blog title")
		}
	}

	// Thumbnail text must not conflict with the topic: it should share a term with
	// the primary keywords or the title (not be purely generic/version chrome).
	if tt := m.YouTube.ThumbnailText; len(tt) > 0 && len(m.Keywords.Primary) > 0 {
		if !sharesToken(tt, strings.Join(m.Keywords.Primary, " ")) && !sharesToken(tt, m.Blog.Title) {
			problems = append(problems, "youtube: thumbnail text shares no topic term with the primary keywords or title")
		}
		if bad := offTopicThumbnailToken(tt, pkg, m.Blog.Title); bad != "" {
			problems = append(problems, fmt.Sprintf("youtube: thumbnail text contains off-topic token %q", bad))
		}
	}

	// Topic alignment: the primary keywords must overlap the title / meta
	// description (the article's own nouns), or they are not describing this
	// article.
	if len(m.Keywords.Primary) > 0 {
		if topic := collapse(m.Blog.Title + " " + m.Blog.MetaDescription); topic != "" && !sharesToken(m.Keywords.Primary, topic) {
			problems = append(problems, "keywords: primary keywords share no term with the title or meta description")
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

// offTopicThumbnailToken returns the first thumbnail token that is off-topic — a
// stop word ("AN"), or a repository-name fragment ("designing") that does not
// itself appear in the article title (so a genuinely topical "AWS" is allowed).
// Release-tag chrome ("V0.6.0") and topical words pass.
func offTopicThumbnailToken(tt []string, pkg ReleasePackage, title string) string {
	frags := repoFragments(pkg)
	lt := strings.ToLower(title)
	for _, line := range tt {
		for _, raw := range strings.Fields(strings.ToLower(line)) {
			w := strings.Trim(raw, ".,·/|:—-")
			if len(w) < 2 {
				continue
			}
			if stopWords[w] {
				return w
			}
			if frags[w] && !strings.Contains(lt, w) {
				return w
			}
		}
	}
	return ""
}

// jsonldAbout extracts the JSON-LD "about" list as strings.
func jsonldAbout(ld map[string]interface{}) []string {
	raw, ok := ld["about"].([]string)
	if ok {
		return raw
	}
	var out []string
	if anys, ok := ld["about"].([]interface{}); ok {
		for _, a := range anys {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// significantTokens returns lower-cased words of length ≥ 4 (skips stopwords and
// short chrome like "the", "aws", "v0") for topic-overlap checks.
func significantTokens(text string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(text)) {
		w = strings.Trim(w, ".,:;!?()[]{}\"'`")
		if len(w) >= 4 && !primaryStopwords[w] {
			set[w] = true
		}
	}
	return set
}

// sharesToken reports whether any entry in list shares a significant word with
// text.
func sharesToken(list []string, text string) bool {
	want := significantTokens(text)
	if len(want) == 0 {
		return true // nothing meaningful to match against — don't fail
	}
	for _, item := range list {
		for w := range significantTokens(item) {
			if want[w] {
				return true
			}
		}
	}
	return false
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
