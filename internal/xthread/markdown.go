package xthread

import (
	"fmt"
	"strings"
)

// Markdown renders the collection as a production-ready document, one section
// per thread with each numbered post, key takeaways, CTA, hashtags, and visual
// references.
func (col XThreadCollection) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# X Threads: %s %s\n\n", col.Metadata.Repository, col.Metadata.Release)
	fmt.Fprintf(&b, "_%d threads · %d posts each · avg engagement %d/100_\n\n",
		col.Metadata.ThreadCount, col.Metadata.PostsPerThread, col.ContentIntelligence.AverageEngagementScore)

	// Warnings are internal QA signals kept on the struct for CI — they must not
	// render into the published artifact.

	for _, th := range col.Threads {
		writeThread(&b, th)
	}

	writeIntelligence(&b, col.ContentIntelligence)
	return b.String()
}

func writeThread(b *strings.Builder, th Thread) {
	fmt.Fprintf(b, "---\n\n## %s\n\n", th.Type)
	fmt.Fprintf(b, "**Audience:** %s · **Length:** %d posts · **Engagement:** %d/100\n\n", th.Audience, th.Length, th.Metadata.EngagementScore)

	for _, p := range th.Posts {
		fmt.Fprintf(b, "**Post %d/%d** _(%d chars)_\n\n", p.Index, th.Length, p.CharacterCount)
		fmt.Fprintf(b, "> %s\n\n", strings.ReplaceAll(p.Content, "\n", "\n> "))
		if p.CodeSnippet != "" {
			fmt.Fprintf(b, "```\n%s\n```\n\n", p.CodeSnippet)
		}
		if p.VisualReference != "" {
			fmt.Fprintf(b, "_Visual: %s_\n\n", p.VisualReference)
		}
	}

	if len(th.KeyTakeaways) > 0 {
		b.WriteString("**Key takeaways:**\n")
		for _, k := range th.KeyTakeaways {
			fmt.Fprintf(b, "- %s\n", collapse(k))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "**Engagement prompt:** %s\n\n", th.EngagementPrompt)
	fmt.Fprintf(b, "**CTA:** %s\n\n", th.CTA)
	if len(th.Hashtags) > 0 {
		fmt.Fprintf(b, "**Hashtags:** %s\n\n", strings.Join(th.Hashtags, " "))
	}
	if len(th.VisualReferences) > 0 {
		b.WriteString("**Visual references:**\n")
		for _, v := range th.VisualReferences {
			fmt.Fprintf(b, "- [%s] %s _(%s)_\n", v.Type, v.Reference, v.Source)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "_Suggested posting: %s · %s_\n\n", th.Metadata.SuggestedPostingTime, th.Metadata.EstimatedReadingTime)
}

func writeIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Content Intelligence\n\n")
	fmt.Fprintf(b, "- **Threads:** %d · **Tone:** %s\n", ci.ThreadCount, ci.Tone)
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
	fmt.Fprintf(b, "- **Avg engagement:** %d/100 · **Avg technical confidence:** %d/100\n", ci.AverageEngagementScore, ci.AverageTechnicalConfidence)
	fmt.Fprintf(b, "- **Posting cadence:** %s\n", ci.PostingCadence)
	if len(ci.ProductionNotes) > 0 {
		b.WriteString("- **Production notes:**\n")
		for _, n := range ci.ProductionNotes {
			fmt.Fprintf(b, "  - %s\n", n)
		}
	}
	b.WriteString("\n")
}
