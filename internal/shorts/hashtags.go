package shorts

import "strings"

// maxHashtags keeps the tag set focused (excessive hashtags hurt reach).
const maxHashtags = 8

// baseHashtags always apply to this project's Shorts.
var baseHashtags = []string{"SoftwareEngineering", "DevOps", "GitHub"}

// angleHashtags are the tags most relevant to each Short angle.
var angleHashtags = map[string][]string{
	"Architecture Reveal":   {"SystemDesign", "Architecture", "CloudArchitecture"},
	"AWS Best Practice":     {"AWS", "CloudComputing", "BestPractices"},
	"CloudFormation Tip":    {"AWS", "CloudFormation", "IaC"},
	"Code Walkthrough":      {"Coding", "Golang", "CleanCode"},
	"Demo Highlight":        {"Demo", "DevTools", "Automation"},
	"Interesting Statistic": {"OpenSource", "DevLife"},
	"Lesson Learned":        {"LessonsLearned", "CleanArchitecture"},
	"Optimization":          {"Performance", "Optimization"},
	"Common Mistake":        {"CodingTips", "DevMistakes"},
	"Developer Tip":         {"DevTips", "Productivity"},
}

// planHashtags returns a focused, deduplicated hashtag set for a Short: angle
// tags first, then grounded topics from the release, then the base set.
func planHashtags(c candidate, pkg ReleasePackage) []string {
	var tags []string
	tags = append(tags, angleHashtags[c.Angle]...)

	// Grounded topics from AWS services and technologies become tags.
	if ctx := pkg.Context; ctx != nil {
		for _, svc := range ctx.Architecture.AWSServices {
			tags = append(tags, hashify(svc))
		}
		for _, t := range ctx.Technologies {
			tags = append(tags, hashify(t.Name))
		}
	}
	tags = append(tags, baseHashtags...)

	tags = dedupe(tags)
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != "" {
			out = append(out, "#"+t)
		}
	}
	return topStrings(out, maxHashtags)
}

// hashify turns a service/tech name into a CamelCase hashtag token.
func hashify(s string) string {
	fields := strings.Fields(s)
	var b strings.Builder
	for _, f := range fields {
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
