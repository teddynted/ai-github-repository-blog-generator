package seo

import "strings"

// knownAWSServices is the canonical set of AWS service product names, normalized
// (lower-case, single-spaced). It backs isAWSServiceName so a service is
// recognized even when the Release Context did not list it verbatim. Membership
// is by WHOLE-STRING match: "amazon ec2" is a service, but "ec2 startup
// optimization" — a topic phrase that merely contains "ec2" — is not.
var knownAWSServices = map[string]bool{
	"aws iam": true, "amazon cloudwatch": true, "aws lambda": true,
	"amazon eventbridge": true, "amazon eventbridge scheduler": true,
	"amazon ec2": true, "amazon s3": true, "amazon bedrock": true,
	"amazon sqs": true, "amazon sns": true, "aws cloudformation": true,
	"amazon dynamodb": true, "amazon api gateway": true, "aws step functions": true,
	"amazon ecr": true, "amazon ecs": true, "amazon eks": true, "amazon rds": true,
	"amazon vpc": true, "aws fargate": true, "amazon route 53": true,
	"amazon sagemaker": true, "aws secrets manager": true, "aws systems manager": true,
	"amazon kinesis": true, "aws glue": true, "amazon athena": true,
	"amazon cloudfront": true, "aws appsync": true, "amazon cognito": true,
	"aws step function": true, "amazon simple queue service": true,
	// bare service tokens the context sometimes emits
	"ec2": true, "s3": true, "iam": true, "lambda": true, "cloudwatch": true,
	"eventbridge": true, "sqs": true, "sns": true, "dynamodb": true,
	"bedrock": true, "cloudformation": true, "fargate": true,
}

// lowerSet returns the lower-cased, collapsed set of the given terms.
func lowerSet(in []string) map[string]bool {
	m := make(map[string]bool, len(in))
	for _, s := range in {
		if k := strings.ToLower(collapse(s)); k != "" {
			m[k] = true
		}
	}
	return m
}

// isAWSServiceName reports whether s names an AWS service (as opposed to an
// engineering topic that merely mentions one). It matches the whole normalized
// string against the release's detected services and the canonical service set —
// never a substring, so "EC2 startup optimization" stays a topic, not a service.
func isAWSServiceName(s string, detected map[string]bool) bool {
	// normKey folds hyphens/underscores to spaces so a hyphenated tag form
	// ("aws-iam") is recognized as the service "aws iam".
	key := normKey(s)
	if key == "" {
		return false
	}
	return detected[key] || knownAWSServices[key]
}

// countAWSServiceNames counts how many of the terms are AWS service names.
func countAWSServiceNames(list []string, detected map[string]bool) int {
	n := 0
	for _, s := range list {
		if isAWSServiceName(s, detected) {
			n++
		}
	}
	return n
}

// dropAWSServices removes bare AWS service names, preserving order — used to keep
// service inventory out of the topic/primary keyword set.
func dropAWSServices(in []string, detected map[string]bool) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !isAWSServiceName(s, detected) {
			out = append(out, s)
		}
	}
	return out
}

// containsFold reports whether list already holds s (case/space-insensitive).
func containsFold(list []string, s string) bool {
	k := strings.ToLower(collapse(s))
	for _, x := range list {
		if strings.ToLower(collapse(x)) == k {
			return true
		}
	}
	return false
}

// centralAWS returns the AWS services the release is genuinely ABOUT: those named
// in the topic signal text (feature, SEO keywords, highlights, summary, title),
// as opposed to incidental infrastructure. Matching is on the full service name
// or its distinctive short token ("ec2", "bedrock") — tokens under 3 chars ("s3")
// require the full name to avoid false hits.
func centralAWS(aws []string, signals string) []string {
	s := strings.ToLower(signals)
	var out []string
	for _, svc := range aws {
		name := strings.ToLower(collapse(svc))
		tok := name
		if i := strings.LastIndex(name, " "); i >= 0 {
			tok = name[i+1:]
		}
		if strings.Contains(s, name) || (len(tok) >= 3 && strings.Contains(s, tok)) {
			out = append(out, svc)
		}
	}
	return out
}

// buildPrimary assembles up to max primary keywords that represent search intent:
// topic terms first, then only the AWS services the release is central to
// (relevantAWS), and never letting service names exceed half of the set. If the
// release yielded no topic terms at all (a featureless service release), the
// services themselves become the primary set.
func buildPrimary(topic, relevantAWS, allAWS []string, detected map[string]bool, max int) []string {
	out := make([]string, 0, max)
	for _, t := range topic {
		if len(out) >= max {
			break
		}
		if !containsFold(out, t) {
			out = append(out, t)
		}
	}
	for _, s := range relevantAWS {
		if len(out) >= max {
			break
		}
		if containsFold(out, s) {
			continue
		}
		// Adding one more service must keep AWS names at ≤ 50% of the result.
		if (countAWSServiceNames(out, detected)+1)*2 > len(out)+1 {
			break
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		out = topStrings(allAWS, 3)
	}
	return out
}
