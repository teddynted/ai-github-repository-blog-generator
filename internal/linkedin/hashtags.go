package linkedin

import "strings"

// maxHashtags keeps LinkedIn hashtags focused — quality over quantity.
const maxHashtags = 6

// baseHashtags always apply to this project's LinkedIn posts.
var baseHashtags = []string{"#SoftwareEngineering", "#OpenSource"}

// typeHashtags are the tags most relevant to each post type.
var typeHashtags = map[string][]string{
	"Architecture Deep Dive":     {"#Architecture", "#CloudComputing", "#SystemDesign"},
	"AWS Best Practice":          {"#AWS", "#CloudComputing", "#DevOps"},
	"AI Engineering Highlight":   {"#AIEngineering", "#MachineLearning"},
	"Engineering Lesson":         {"#SoftwareArchitecture", "#Engineering"},
	"Performance Improvement":    {"#DevOps", "#Reliability"},
	"Feature Spotlight":          {"#Developers", "#Coding"},
	"Developer Productivity Tip": {"#DeveloperProductivity", "#Coding"},
	"Behind-the-Build":           {"#BuildInPublic", "#OpenSource"},
	"Release Announcement":       {"#Release", "#DevOps"},
}

// planHashtags returns focused, deduped LinkedIn hashtags: type tags, the SEO
// engine's LinkedIn hashtags, grounded technology/AWS tags, then the base set.
func planHashtags(pkg ReleasePackage, c postCandidate) []string {
	var tags []string
	tags = append(tags, typeHashtags[c.Type]...)
	tags = append(tags, pkg.SEO.Hashtags.LinkedIn...)
	for _, svc := range awsServices(pkg) {
		tags = append(tags, "#"+hashify(svc))
	}
	for _, l := range programmingLanguages(pkg) {
		tags = append(tags, "#"+hashify(l))
	}
	tags = append(tags, baseHashtags...)
	return topStrings(dedupe(tags), maxHashtags)
}

func hashify(s string) string {
	var b strings.Builder
	for _, f := range strings.Fields(s) {
		f = strings.TrimFunc(f, func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
		})
		if f == "" {
			continue
		}
		b.WriteString(strings.ToUpper(f[:1]))
		b.WriteString(f[1:])
	}
	return b.String()
}
