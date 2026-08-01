package seo

import "strings"

func wordCount(s string) int { return len(strings.Fields(s)) }

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func topStrings(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}

// truncateChars caps s at max characters on a word boundary, adding an ellipsis.
func truncateChars(s string, max int) string {
	s = collapse(s)
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}

// truncateWords caps s at max words, adding an ellipsis when it truncates.
func truncateWords(s string, max int) string {
	f := strings.Fields(s)
	if len(f) <= max {
		return strings.Join(f, " ")
	}
	return strings.Join(f[:max], " ") + "…"
}

// firstSentences returns up to n sentences of s (boundary requires trailing
// space/EOS so version tags like v0.2.0 are not split).
func firstSentences(s string, n int) string {
	s = collapse(s)
	if s == "" {
		return ""
	}
	var out []string
	start, count := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] != '.' && s[i] != '!' && s[i] != '?' {
			continue
		}
		if i+1 < len(s) && s[i+1] != ' ' {
			continue
		}
		out = append(out, strings.TrimSpace(s[start:i+1]))
		start = i + 1
		count++
		if count >= n {
			break
		}
	}
	if count == 0 {
		return s
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

// slugify produces a lowercase, hyphen-separated, readable slug.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// hashify turns a phrase into a CamelCase hashtag token (no leading #).
// canonicalHashtagBody maps grounded AWS tokens to their canonical, hyphen-free
// hashtag body with correct casing (a hashtag can't contain a hyphen, and
// "#Aws-iam" is wrong for both display and search).
var canonicalHashtagBody = map[string]string{
	"aws-iam":                      "AWSIAM",
	"aws-lambda":                   "AWSLambda",
	"aws-cloudformation":           "AWSCloudFormation",
	"amazon-ec2":                   "AmazonEC2",
	"amazon-s3":                    "AmazonS3",
	"amazon-cloudwatch":            "AmazonCloudWatch",
	"amazon-eventbridge":           "AmazonEventBridge",
	"amazon-eventbridge-scheduler": "AmazonEventBridgeScheduler",
	"amazon-sqs":                   "AmazonSQS",
	"amazon-sns":                   "AmazonSNS",
	"github-actions":               "GitHubActions",
}

// hashtagAcronyms are word parts that should be fully upper-cased in a hashtag.
var hashtagAcronyms = map[string]string{
	"aws": "AWS", "iam": "IAM", "ec2": "EC2", "s3": "S3", "api": "API",
	"sdk": "SDK", "cli": "CLI", "sqs": "SQS", "sns": "SNS", "ai": "AI",
	"ci": "CI", "cd": "CD",
}

// hashify turns a grounded token into a hyphen-free hashtag body with canonical
// AWS casing: "aws-iam" -> "AWSIAM", "amazon-ec2" -> "AmazonEC2",
// "event-driven" -> "EventDriven".
func hashify(s string) string {
	key := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(s, "-", " ")), "-"))
	if body, ok := canonicalHashtagBody[key]; ok {
		return body
	}
	var b strings.Builder
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) {
		if a, ok := hashtagAcronyms[strings.ToLower(part)]; ok {
			b.WriteString(a)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}

// prependHash turns tokens into #hashtags, skipping blanks and deduping.
func prependHash(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range dedupe(tokens) {
		if h := hashify(t); h != "" {
			out = append(out, "#"+h)
		}
	}
	return out
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] += 'a' - 'A'
	}
	return string(r)
}
