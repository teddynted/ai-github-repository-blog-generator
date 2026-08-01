package linkedin

import (
	"fmt"
	"strings"
)

// Markdown renders the collection as a production-ready document, one section
// per post with body, highlights, engagement prompt, CTA, hashtags, and visual
// references.
func (col LinkedInCollection) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# LinkedIn Content: %s %s\n\n", col.Metadata.Repository, col.Metadata.Release)
	fmt.Fprintf(&b, "_%d posts · avg engagement %d/100 · avg confidence %d/100_\n\n",
		col.Metadata.PostCount, col.ContentIntelligence.AverageEngagementScore, col.ContentIntelligence.AverageProfessionalConfidence)

	// Warnings (e.g. "post 3 has no technical highlights") are internal QA signals
	// kept on the struct for CI/manifest — they must NOT render into the published
	// artifact, where they read as broken copy.

	for _, p := range col.Posts {
		writePost(&b, p)
	}

	writeIntelligence(&b, col.ContentIntelligence)
	return b.String()
}

func writePost(b *strings.Builder, p Post) {
	fmt.Fprintf(b, "---\n\n## %s\n\n", p.Type)
	fmt.Fprintf(b, "**Variation:** %s · **Audience:** %s · **Engagement:** %d/100\n\n", p.Variation, p.Audience, p.Metadata.EngagementScore)
	fmt.Fprintf(b, "**Title:** %s\n\n", p.Title)

	fmt.Fprintf(b, "### Post\n\n%s\n\n", p.Body)

	if len(p.TechnicalHighlights) > 0 {
		b.WriteString("**Technical highlights:**\n")
		for _, h := range p.TechnicalHighlights {
			fmt.Fprintf(b, "- %s\n", collapse(h))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "**Engagement prompt:** %s\n\n", p.EngagementPrompt)
	fmt.Fprintf(b, "**CTA:** %s\n\n", p.CTA)
	if len(p.Hashtags) > 0 {
		fmt.Fprintf(b, "**Hashtags:** %s\n\n", strings.Join(p.Hashtags, " "))
	}
	if len(p.VisualReferences) > 0 {
		b.WriteString("**Visual references:**\n")
		for _, v := range p.VisualReferences {
			fmt.Fprintf(b, "- [%s] %s _(%s)_\n", v.Type, v.Reference, v.Source)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "_Suggested publish: %s · %s_\n\n", p.Metadata.SuggestedPublishTime, p.Metadata.EstimatedReadingTime)
}

func writeIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Content Intelligence\n\n")
	fmt.Fprintf(b, "- **Posts:** %d · **Tone:** %s\n", ci.PostCount, ci.Tone)
	if len(ci.TargetAudiences) > 0 {
		fmt.Fprintf(b, "- **Audiences:** %s\n", strings.Join(ci.TargetAudiences, "; "))
	}
	if len(ci.AWSServices) > 0 {
		fmt.Fprintf(b, "- **AWS services:** %s\n", strings.Join(ci.AWSServices, ", "))
	}
	if len(ci.TechnologyStack) > 0 {
		fmt.Fprintf(b, "- **Technology stack:** %s\n", strings.Join(ci.TechnologyStack, ", "))
	}
	if len(ci.SEOKeywords) > 0 {
		fmt.Fprintf(b, "- **SEO keywords:** %s\n", strings.Join(ci.SEOKeywords, ", "))
	}
	fmt.Fprintf(b, "- **Avg engagement:** %d/100 · **Avg confidence:** %d/100\n", ci.AverageEngagementScore, ci.AverageProfessionalConfidence)
	fmt.Fprintf(b, "- **Publish cadence:** %s\n", ci.PublishCadence)
	if len(ci.ProductionNotes) > 0 {
		b.WriteString("- **Production notes:**\n")
		for _, n := range ci.ProductionNotes {
			fmt.Fprintf(b, "  - %s\n", n)
		}
	}
	b.WriteString("\n")
}
