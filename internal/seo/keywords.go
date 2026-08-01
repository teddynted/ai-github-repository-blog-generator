package seo

import "strings"

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
	var topicCandidates []string
	if f := firstSentences(featureName(pkg), 1); !isGenericKeyword(f) {
		topicCandidates = append(topicCandidates, f)
	}
	topicCandidates = append(topicCandidates, technical...)
	topicCandidates = dropAWSServices(dropGeneric(dedupe(topicCandidates)), awsSet)
	topicCandidates = topStrings(topicCandidates, 5)

	relevantAWS := centralAWS(aws, topicSignals(pkg))
	primary := buildPrimary(topicCandidates, relevantAWS, aws, awsSet, 5)

	// Secondary: the supporting entities — AWS services first, then technologies
	// and the remaining grounded technical terms.
	secondary := topStrings(dedupe(append(append(append([]string{}, aws...), tech...), technical...)), 10)
	secondary = subtract(secondary, primary)

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
