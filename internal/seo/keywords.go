package seo

import (
	"regexp"
	"strings"
)

// primaryStopwords are tokens too generic or infrastructural to be a PRIMARY
// keyword (search intent). They may still appear as tags or secondary keywords —
// "aws" and "go" are useful there — but they carry no article-topic signal on
// their own.
var primaryStopwords = map[string]bool{
	"aws": true, "go": true, "golang": true, "cloud": true, "api": true,
	"sdk": true, "cli": true, "framework": true, "library": true,
	"open source": true, "github": true, "repository": true, "repo": true,
	"serverless": true, "backend": true, "infrastructure": true,
}

// versionRE matches version numbers ("1.22", "v0.6", "3.11"); langVersionRE
// matches a language paired with a version ("Go 1.22", "python3.11", "node 18").
var (
	versionRE     = regexp.MustCompile(`\bv?\d+\.\d+`)
	langVersionRE = regexp.MustCompile(`(?i)\b(go|golang|python|node|nodejs|java|ruby|rust|php|dotnet|\.net|typescript)\s*v?\d`)
)

// isJunkKeyword reports whether s is dependency/repo/release noise that must not
// appear in ANY keyword field (primary, secondary, tags, JSON-LD): stop words
// ("an", "the"), changelog/runtime/SDK identifiers, parenthetical runtime tags
// ("aws lambda (go runtime)"), version strings, and repository-name fragments
// ("designing", "platform"). It is the shared, lenient filter — it keeps valid
// technology names like "Go" and "AWS Lambda".
func isJunkKeyword(s string, pkg ReleasePackage) bool {
	k := strings.ToLower(collapse(s))
	if k == "" || stopWords[k] {
		return true
	}
	if strings.Contains(k, "changelog") || strings.Contains(k, "runtime") || strings.Contains(k, "sdk") {
		return true
	}
	if strings.ContainsAny(k, "()") {
		return true
	}
	// A keyword is a search term, not a sentence — drop long prose (e.g. a full
	// changelog feature line leaking in as a "keyword").
	if len(strings.Fields(k)) > 6 {
		return true
	}
	if versionRE.MatchString(k) || langVersionRE.MatchString(k) {
		return true
	}
	return repoFragments(pkg)[k]
}

// dropJunk removes junk keywords from any field, preserving order.
func dropJunk(in []string, pkg ReleasePackage) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !isJunkKeyword(s, pkg) {
			out = append(out, s)
		}
	}
	return out
}

// platformConcepts are engineering/tooling/architecture concepts that describe
// HOW a project is built, not what an article is about — they are supporting
// technologies, not search intent, so they never lead the primary keyword set
// (or JSON-LD "about"). They remain eligible as tags/secondary keywords. The one
// exception is the headline rule in isPlatformConcept: a concept the article is
// specifically about (named in the title headline) is allowed.
var platformConcepts = map[string]bool{
	"github actions": true, "github action": true, "github automation": true,
	"go modules": true, "go module": true, "go generics": true,
	"infrastructure as code": true, "iac": true,
	"ci/cd": true, "cicd": true, "continuous integration": true,
	"continuous deployment": true, "continuous delivery": true,
	"event-driven architecture": true, "event driven architecture": true,
	"clean architecture": true, "hexagonal architecture": true,
	"domain-driven design": true, "test-driven development": true,
	"microservices": true, "monorepo": true, "rest api": true, "graphql": true,
	"devops": true, "serverless": true, "open source": true,
	"technical blogging": true, "ai content generation": true,
	"developer productivity": true, "make": true, "makefile": true,
}

// titleHeadline is the lower-cased headline of the blog title — the part before
// the first colon (the article's actual subject, before qualifiers). For
// "Optimizing Spot Startup on AWS: Pre-Baked Custom AMIs …" it is "optimizing
// spot startup on aws".
func titleHeadline(pkg ReleasePackage) string {
	t := strings.ToLower(collapse(pkg.Blog.Title))
	if i := strings.Index(t, ":"); i > 0 {
		t = t[:i]
	}
	return t
}

// normKey normalizes a keyword for blocklist lookups: lower-cased, hyphens and
// other separators folded to single spaces, so "github-actions", "GitHub
// Actions", and "github_actions" all match the same entry.
func normKey(s string) string {
	return strings.ToLower(strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' ' || r == '\t' || r == '/'
	}), " "))
}

// isPlatformConcept reports whether s is a supporting platform/tooling concept
// unfit to be a primary keyword — UNLESS the article is specifically about it
// (it appears in the title headline).
func isPlatformConcept(s string, pkg ReleasePackage) bool {
	k := normKey(s)
	if !platformConcepts[k] {
		return false
	}
	return !strings.Contains(normKey(titleHeadline(pkg)), k)
}

// isNoisyPrimaryKeyword is the STRICT filter for primary keywords (search
// intent): junk, plus anything under 3 chars, a primary stopword ("aws", "go",
// "cloud"), a generic term, a supporting platform concept, or the repository
// name. Primary keywords must describe article intent, not the dependency/repo
// inventory or how the project is built.
func isNoisyPrimaryKeyword(s string, pkg ReleasePackage) bool {
	k := strings.ToLower(collapse(s))
	if isJunkKeyword(s, pkg) {
		return true
	}
	if len(k) < 3 || primaryStopwords[k] || isGenericKeyword(k) {
		return true
	}
	if isPlatformConcept(s, pkg) {
		return true
	}
	if r := strings.ToLower(collapse(repoShortName(pkg))); r != "" && (k == r || strings.Contains(k, r)) {
		return true
	}
	if r := strings.ToLower(collapse(repoName(pkg))); r != "" && k == r {
		return true
	}
	return false
}

// dropNoisyPrimary removes primary-unfit keywords, preserving order.
func dropNoisyPrimary(in []string, pkg ReleasePackage) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !isNoisyPrimaryKeyword(s, pkg) {
			out = append(out, s)
		}
	}
	return out
}

// repoFragments is the set of individual words in the repository name — e.g.
// "designing-an-ai-agent-platform-on-aws" → {designing, an, ai, agent, platform,
// on, aws}. These are repository metadata, never article keywords, so they are
// filtered from keyword fields (matched whole-word, so multi-word topics like
// "AI Agent Platforms" are unaffected).
func repoFragments(pkg ReleasePackage) map[string]bool {
	set := map[string]bool{}
	for _, name := range []string{repoName(pkg), repoShortName(pkg)} {
		for _, w := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
		}) {
			if len(w) >= 2 {
				set[w] = true
			}
		}
	}
	return set
}

// baseDeveloperKeywords are evergreen developer/SEO terms this project's content
// legitimately targets. They are only emitted alongside grounded terms.
var baseDeveloperKeywords = []string{
	"GitHub automation", "AI content generation", "technical blogging",
	"developer productivity", "Clean Architecture", "DevOps",
}

// planKeywords builds the grounded keyword taxonomy. All keywords come from the
// Release Context (AWS services, technologies, SEO keywords, highlights) plus a
// small evergreen developer set — nothing is invented.
func planKeywords(pkg ReleasePackage) Keywords {
	aws := awsServices(pkg)
	tech := technologies(pkg)
	awsSet := lowerSet(aws)

	var technical []string
	if c := pkg.Context; c != nil {
		technical = append(technical, c.ContentIntelligence.SEOKeywords...)
		technical = append(technical, c.ContentIntelligence.TechnicalHighlights...)
	}
	technical = append(technical, pkg.Blog.Tags...)
	technical = dropGeneric(dedupe(technical))

	developer := dedupe(baseDeveloperKeywords)

	// Primary keywords represent SEARCH INTENT — the engineering problem the
	// release solves and its technical approach — NOT the AWS service inventory.
	// Topic candidates come from the headline feature and the grounded technical/
	// SEO terms, with bare AWS service names excluded (they are supporting
	// entities, so they belong in Secondary). buildPrimary then caps AWS service
	// names at half of the primary set: a service leads only when the release is
	// genuinely about it. Generic terms ("agent", "ai", "release") are dropped —
	// no search signal, and they dilute the set.
	// Search-intent terms the author already curated (SEO keywords) lead — they
	// are concise, searchable phrases. The headline feature follows as a fallback
	// topic; the rest of the technical terms fill in.
	var topicCandidates []string
	if c := pkg.Context; c != nil {
		topicCandidates = append(topicCandidates, c.ContentIntelligence.SEOKeywords...)
	}
	if f := firstSentences(featureName(pkg), 1); !isGenericKeyword(f) {
		topicCandidates = append(topicCandidates, f)
	}
	topicCandidates = append(topicCandidates, technical...)
	topicCandidates = dropAWSServices(dropGeneric(dedupe(topicCandidates)), awsSet)
	topicCandidates = dropNoisyPrimary(topicCandidates, pkg)
	topicCandidates = topStrings(topicCandidates, 5)

	relevantAWS := centralAWS(aws, topicSignals(pkg))
	primary := buildPrimary(topicCandidates, relevantAWS, aws, awsSet, 5)

	// Secondary: the supporting entities — AWS services first, then technologies
	// and the remaining grounded technical terms (junk filtered out).
	secondary := dropJunk(dedupe(append(append(append([]string{}, aws...), tech...), technical...)), pkg)
	secondary = topStrings(subtract(secondary, primary), 10)

	return Keywords{
		Primary:    primary,
		Secondary:  secondary,
		LongTail:   longTail(pkg, aws),
		Technical:  technical,
		Technology: tech,
		AWS:        aws,
		Developer:  developer,
	}
}

// longTail builds grounded long-tail search phrases.
func longTail(pkg ReleasePackage, aws []string) []string {
	repo := repoShortName(pkg)
	tag := releaseTag(pkg)
	feat := lowerFirst(firstSentences(featureName(pkg), 1))
	var out []string
	if feat != "" && feat != "this release" {
		out = append(out, "how to build "+feat)
		out = append(out, feat+" tutorial")
	}
	out = append(out, repo+" "+tag+" architecture")
	if len(aws) >= 2 {
		out = append(out, "event-driven pipeline with "+strings.ToLower(aws[0])+" and "+strings.ToLower(aws[1]))
	} else if len(aws) == 1 {
		out = append(out, strings.ToLower(aws[0])+" architecture walkthrough")
	}
	out = append(out, "AI-powered content generation from GitHub releases")
	return topStrings(dedupe(out), 6)
}

// allKeywords merges the taxonomy into one deduped list (for tags/JSON-LD).
func allKeywords(k Keywords) []string {
	var all []string
	all = append(all, k.Primary...)
	all = append(all, k.Secondary...)
	all = append(all, k.AWS...)
	all = append(all, k.Technology...)
	all = append(all, k.Technical...)
	all = append(all, k.Developer...)
	return dedupe(all)
}

func subtract(from, remove []string) []string {
	drop := map[string]bool{}
	for _, r := range remove {
		drop[strings.ToLower(strings.TrimSpace(r))] = true
	}
	var out []string
	for _, s := range from {
		if !drop[strings.ToLower(strings.TrimSpace(s))] {
			out = append(out, s)
		}
	}
	return out
}

// genericKeywords are too broad to target: they add no search signal and dilute
// keyword sets. "this release" is the featureless-release placeholder.
var genericKeywords = map[string]bool{
	"this release": true, "the release": true, "release": true,
	"agent": true, "ai": true, "software": true, "code": true,
	"app": true, "tool": true, "system": true,
}

func isGenericKeyword(s string) bool {
	return genericKeywords[strings.ToLower(strings.TrimSpace(s))]
}

// dropGeneric removes generic keywords, preserving order.
func dropGeneric(kw []string) []string {
	out := make([]string, 0, len(kw))
	for _, k := range kw {
		if !isGenericKeyword(k) {
			out = append(out, k)
		}
	}
	return out
}
