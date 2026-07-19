package visualassets

import "strings"

func repoName(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.FullName != "" {
		return pkg.Context.Repository.FullName
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Repository, pkg.YouTube.Metadata.Repository, "this project")
}

func repoShortName(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.Name != "" {
		return pkg.Context.Repository.Name
	}
	full := repoName(pkg)
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[i+1:]
	}
	return full
}

func releaseTag(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.Tag != "" {
		return pkg.Context.Release.Tag
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Release, pkg.YouTube.Metadata.Release)
}

func repoURL(pkg ReleasePackage) string {
	if pkg.Context != nil {
		if pkg.Context.Repository.URL != "" {
			return pkg.Context.Repository.URL
		}
		if pkg.Context.Repository.FullName != "" {
			return "https://github.com/" + pkg.Context.Repository.FullName
		}
	}
	return ""
}

func awsServices(pkg ReleasePackage, n int) []string {
	if pkg.Context == nil {
		return nil
	}
	return topStrings(pkg.Context.Architecture.AWSServices, n)
}

func diagramSource(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		for _, d := range c.Mermaid {
			if d.Source != "" {
				return d.Source
			}
		}
	}
	for _, sc := range pkg.Storyboard.Scenes {
		for _, d := range sc.Diagrams {
			if d.Source != "" {
				return d.Source
			}
		}
	}
	return ""
}

// featureName is a short, grounded name for what shipped.
func featureName(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if len(c.Changelog.Features) > 0 {
			return firstSentences(c.Changelog.Features[0], 1)
		}
		if len(c.Implementation.WhatChanged) > 0 {
			return firstSentences(c.Implementation.WhatChanged[0], 1)
		}
	}
	return "this release"
}

// subjectSeed returns a grounded one-line subject the imagery should depict.
func subjectSeed(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if s := firstSentences(c.Architecture.Overview, 1); s != "" {
			return s
		}
		if s := firstSentences(c.ContentIntelligence.Summary, 1); s != "" {
			return s
		}
	}
	return firstNonEmpty(pkg.Blog.Title, repoShortName(pkg)+" "+releaseTag(pkg))
}

// technicalTopics gathers grounded topics for SEO and technical focus.
func technicalTopics(pkg ReleasePackage) []string {
	var out []string
	if c := pkg.Context; c != nil {
		out = append(out, c.Architecture.AWSServices...)
		for _, t := range c.Technologies {
			out = append(out, t.Name)
		}
		out = append(out, c.ContentIntelligence.SEOKeywords...)
	}
	out = append(out, pkg.Blog.Tags...)
	return topStrings(dedupe(out), 12)
}

// groundedRefs is the set of reference strings an asset may cite: the repo,
// release, Mermaid sources, AWS services, and technologies. Validation uses it
// to confirm every prompt is grounded.
func groundedRefs(pkg ReleasePackage) map[string]bool {
	refs := map[string]bool{}
	add := func(s string) {
		if s = collapse(s); s != "" {
			refs[s] = true
		}
	}
	add(repoURL(pkg))
	add(repoName(pkg))
	add(repoShortName(pkg))
	add(releaseTag(pkg))
	if c := pkg.Context; c != nil {
		for _, d := range c.Mermaid {
			add(d.Source)
		}
		for _, svc := range c.Architecture.AWSServices {
			add(svc)
		}
		for _, t := range c.Technologies {
			add(t.Name)
		}
	}
	return refs
}

// knownAWSServices is a catalogue used to detect AWS service mentions in a
// prompt, so validation can reject any service NOT present in the Release
// Context (the "no unsupported implementation details" rule).
var knownAWSServices = []string{
	"AWS Lambda", "Lambda", "Amazon SQS", "SQS", "Amazon S3", "S3",
	"Amazon EventBridge", "EventBridge", "Amazon DynamoDB", "DynamoDB",
	"Amazon EC2", "EC2", "API Gateway", "Amazon Bedrock", "Bedrock",
	"CloudFormation", "Amazon SNS", "SNS", "Amazon RDS", "RDS",
	"Amazon ECS", "ECS", "Amazon EKS", "EKS", "AWS Fargate", "Fargate",
	"CloudWatch", "Amazon Kinesis", "Kinesis", "AWS Step Functions",
	"Secrets Manager", "Amazon CloudFront", "CloudFront", "Amazon Cognito",
	"Amazon Redshift", "Redshift", "Amazon SageMaker", "SageMaker",
	"AWS Glue", "Glue", "Amazon Aurora", "Aurora",
}

// contextAWSSet returns the lowercased set of AWS services named in the context.
func contextAWSSet(pkg ReleasePackage) map[string]bool {
	set := map[string]bool{}
	if pkg.Context == nil {
		return set
	}
	for _, svc := range pkg.Context.Architecture.AWSServices {
		set[strings.ToLower(collapse(svc))] = true
	}
	return set
}
