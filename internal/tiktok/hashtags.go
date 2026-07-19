package tiktok

import "strings"

// maxHashtags keeps the tag set practical for TikTok.
const maxHashtags = 8

// baseHashtags always apply to this project's TikToks.
var baseHashtags = []string{"TechTok", "SoftwareEngineering", "AIEngineering", "Programming"}

// topicHashtags are the tags most relevant to each TikTok topic.
var topicHashtags = map[string][]string{
	"AWS Tip":                 {"AWS", "CloudComputing", "DevOps"},
	"CloudFormation Trick":    {"AWS", "CloudFormation", "IaC"},
	"AI Workflow":             {"AI", "MachineLearning", "Automation"},
	"GitHub Automation":       {"GitHub", "DevOps", "Automation"},
	"Architecture Insight":    {"SystemDesign", "Architecture", "CloudArchitecture"},
	"Performance Improvement": {"Performance", "Optimization"},
	"Code Optimization":       {"Coding", "GoLang", "CleanCode"},
	"Common Mistake":          {"CodingTips", "DevMistakes"},
	"Developer Productivity":  {"DevTips", "Productivity", "GoLang"},
	"Deployment Strategy":     {"DevOps", "CICD", "Deployment"},
	"Interesting Statistic":   {"OpenSource", "DevLife"},
	"Best Practice":           {"BestPractices", "CleanArchitecture"},
}

// planHashtags returns a focused, deduplicated hashtag set for a TikTok: topic
// tags first, then grounded topics from the release, then the base set.
func planHashtags(t topic, pkg ReleasePackage) []string {
	var tags []string
	tags = append(tags, topicHashtags[t.Topic]...)

	if c := pkg.Context; c != nil {
		for _, svc := range c.Architecture.AWSServices {
			tags = append(tags, hashify(svc))
		}
		for _, tech := range c.Technologies {
			tags = append(tags, hashify(tech.Name))
		}
	}
	tags = append(tags, baseHashtags...)

	tags = dedupe(tags)
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != "" {
			out = append(out, "#"+tag)
		}
	}
	return topStrings(out, maxHashtags)
}

// hashify turns a service/tech name into a CamelCase hashtag token.
func hashify(s string) string {
	fields := strings.Fields(s)
	var b strings.Builder
	for _, f := range fields {
		f = strings.TrimFunc(f, isPunct)
		if f == "" {
			continue
		}
		b.WriteString(strings.ToUpper(f[:1]))
		b.WriteString(f[1:])
	}
	return b.String()
}
