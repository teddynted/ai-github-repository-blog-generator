package xthread

import "strings"

// maxHashtags keeps X hashtags minimal — 2–4 is the norm; walls of tags hurt.
const maxHashtags = 4

// typeHashtags are the tags most relevant to each thread type.
var typeHashtags = map[string][]string{
	"Architecture Walkthrough":    {"#Architecture", "#CloudComputing"},
	"AWS Best Practices":          {"#AWS", "#DevOps"},
	"AI Engineering Insights":     {"#AIEngineering", "#AI"},
	"Engineering Lessons Learned": {"#SoftwareEngineering"},
	"Performance Improvements":    {"#DevOps"},
	"Implementation Deep Dive":    {"#Coding"},
	"Feature Breakdown":           {"#OpenSource"},
	"Developer Tips":              {"#Coding"},
	"Open Source Update":          {"#OpenSource"},
	"Release Announcement":        {"#OpenSource"},
}

// planHashtags returns a focused, deduped X hashtag set: type tags, the SEO X
// hashtags, grounded AWS/language tags, then a base.
func planHashtags(pkg ReleasePackage, c threadCandidate) []string {
	var tags []string
	tags = append(tags, typeHashtags[c.Type]...)
	tags = append(tags, pkg.SEO.Hashtags.X...)
	for _, svc := range awsServices(pkg) {
		tags = append(tags, "#"+hashify(svc))
	}
	for _, l := range programmingLanguages(pkg) {
		tags = append(tags, "#"+hashify(l))
	}
	tags = append(tags, "#SoftwareEngineering")
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
