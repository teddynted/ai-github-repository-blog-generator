package seo

import (
	"context"
	"fmt"
	"strings"
)

// blogTitleMax is the SEO-preferred blog title length (search snippets truncate
// beyond this).
const blogTitleMax = 60

// planBlogTitle returns the blog SEO title. It prefers the already-generated
// blog title, optionally polished by the Model for search, and bounded to the
// SEO length. Grounded — the fallback is the existing title.
func (g *Generator) planBlogTitle(ctx context.Context, pkg ReleasePackage) string {
	base := firstNonEmpty(pkg.Blog.Title, pkg.YouTube.ContentIntelligence.SuggestedTitle,
		repoShortName(pkg)+" "+releaseTag(pkg))
	title := base
	if g.Model != nil {
		if out, err := g.Model.Generate(ctx, blogTitlePrompt(base, releaseTag(pkg))); err == nil {
			title = sanitizeModelText(out, title)
		}
	}
	return boundTitle(title, blogTitleMax, base)
}

func blogTitlePrompt(base, tag string) string {
	return fmt.Sprintf(
		"Rewrite this technical blog title to be SEO-optimized and click-worthy (NOT clickbait), under 60 characters, "+
			"keeping it accurate to the release %s. Use ONLY the facts in the title — invent nothing. Output only the title.\n\nTITLE: %s",
		tag, base)
}

// boundTitle caps a title to max characters, falling back to a bounded base when
// the candidate is empty or too long to salvage.
func boundTitle(title string, max int, base string) string {
	title = collapse(title)
	if title == "" {
		title = collapse(base)
	}
	if len(title) <= max {
		return title
	}
	return truncateChars(title, max)
}

// blogCategory classifies the release into a content category.
func blogCategory(pkg ReleasePackage) string {
	if len(awsServices(pkg)) > 0 {
		return "Cloud & DevOps"
	}
	if c := pkg.Context; c != nil && len(c.Architecture.Components) > 0 {
		return "Software Architecture"
	}
	return "Software Engineering"
}

// topicClusters groups the release into SEO topic clusters that represent the
// content DOMAIN, derived from grounded signals in the release (feature, summary,
// architecture, services, highlights) — not a fixed list of this project's own
// topics. A cluster is emitted only when the release actually exhibits it.
func topicClusters(pkg ReleasePackage) []string {
	hay := clusterSignals(pkg)
	var out []string
	if len(awsServices(pkg)) > 0 {
		out = append(out, "AWS & Cloud Infrastructure")
	}
	for _, r := range clusterRules {
		if containsAnyTerm(hay, r.terms) {
			out = append(out, r.cluster)
		}
	}
	if len(out) == 0 {
		out = append(out, "Software Engineering")
	}
	return topStrings(dedupe(out), 5)
}

// clusterRules map grounded content signals to a domain cluster.
var clusterRules = []struct {
	cluster string
	terms   []string
}{
	{"Event-Driven Architecture", []string{"event-driven", "event driven", "eventbridge", "sqs", "sns", "kafka", "event bus", "pub/sub", "pubsub", "message queue"}},
	{"Compute & Startup Optimization", []string{"ami", "machine image", "spot instance", "startup", "cold start", "provision", "boot time", "instance launch", "userdata", "user data", "pre-bake", "prebake"}},
	{"AI Agent Platforms", []string{"agent", "inference", "bedrock", "llm", "model routing", "prompt", "claude", "genai", "rag "}},
	{"Infrastructure Automation", []string{"pipeline", "workflow", "automation", "ci/cd", "cicd", "orchestrat", "provisioning", "deploy"}},
	{"Software Architecture", []string{"clean architecture", "hexagonal", "domain-driven", "ddd", "microservice", "modular", "ports and adapters"}},
}

// clusterSignals is the lower-cased haystack of grounded text used to classify
// the release's domain.
func clusterSignals(pkg ReleasePackage) string {
	parts := []string{featureName(pkg), summary(pkg), pkg.Blog.Title}
	if c := pkg.Context; c != nil {
		parts = append(parts, c.Architecture.Overview)
		parts = append(parts, c.ContentIntelligence.TechnicalHighlights...)
		parts = append(parts, c.ContentIntelligence.SEOKeywords...)
		parts = append(parts, c.Architecture.AWSServices...)
	}
	parts = append(parts, pkg.Blog.Tags...)
	return strings.ToLower(strings.Join(parts, " "))
}

// topicSignals is the lower-cased haystack of grounded TOPIC PROSE — feature,
// summary, title, architecture overview, highlights. It deliberately excludes
// both the raw AWS service inventory AND keyword/tag dumps (SEO keywords, blog
// tags), which can list a service verbatim: a service counts as "central" only
// when the article's prose actually discusses it, not because someone dumped its
// name into a keyword list.
func topicSignals(pkg ReleasePackage) string {
	parts := []string{featureName(pkg), summary(pkg), pkg.Blog.Title}
	if c := pkg.Context; c != nil {
		parts = append(parts, c.Architecture.Overview)
		parts = append(parts, c.ContentIntelligence.TechnicalHighlights...)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func containsAnyTerm(hay string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}
