package seo

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Markdown renders the SEO metadata as a production-ready document, one section
// per channel plus keywords, hashtags, Open Graph, and structured data.
func (m SEOMetadata) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# SEO Metadata: %s\n\n", firstNonEmpty(m.Metadata.SourceBlogTitle, m.Metadata.Repository))
	fmt.Fprintf(&b, "_%s · release %s · SEO confidence %d/100_\n\n",
		m.Metadata.Repository, m.Metadata.Release, m.ContentIntelligence.SEOConfidenceScore)

	// m.Warnings are internal QA diagnostics (title too long, thin content). They
	// stay on the struct for JSON/provenance and are logged at generation, but
	// must not surface in the viewer-facing artifact — so they are not rendered.

	writeBlog(&b, m.Blog)
	writeYouTube(&b, m.YouTube)
	writeShorts(&b, m.Shorts)
	writeSocial(&b, m.Social)
	writeKeywords(&b, m.Keywords)
	writeHashtags(&b, m.Hashtags)
	writeOpenGraph(&b, m.OpenGraph)
	writeStructured(&b, m.StructuredData)
	writeIntelligence(&b, m.ContentIntelligence)
	return b.String()
}

func writeBlog(b *strings.Builder, s BlogSEO) {
	b.WriteString("---\n\n## Blog SEO\n\n")
	fmt.Fprintf(b, "- **Title:** %s (%d chars)\n", s.Title, len(s.Title))
	fmt.Fprintf(b, "- **Meta description:** %s (%d chars)\n", s.MetaDescription, len(s.MetaDescription))
	fmt.Fprintf(b, "- **Slug:** `%s`\n", s.Slug)
	if len(s.AlternativeSlugs) > 0 {
		fmt.Fprintf(b, "- **Alternative slugs:** %s\n", codeJoin(s.AlternativeSlugs))
	}
	fmt.Fprintf(b, "- **Category:** %s · **Reading time:** %s\n", s.Category, s.ReadingTime)
	if len(s.Tags) > 0 {
		fmt.Fprintf(b, "- **Tags:** %s\n", strings.Join(s.Tags, ", "))
	}
	if len(s.TopicClusters) > 0 {
		fmt.Fprintf(b, "- **Topic clusters:** %s\n", strings.Join(s.TopicClusters, ", "))
	}
	fmt.Fprintf(b, "- **Canonical:** %s (`%s`)\n", firstNonEmpty(s.Canonical.URL, "—"), s.Canonical.Robots)
	fmt.Fprintf(b, "\n**Excerpt (50w):** %s\n\n", s.Excerpts.Short)
	fmt.Fprintf(b, "**Excerpt (100w):** %s\n\n", s.Excerpts.Medium)
}

func writeYouTube(b *strings.Builder, s YouTubeSEO) {
	b.WriteString("---\n\n## YouTube SEO\n\n")
	fmt.Fprintf(b, "- **Title:** %s (%d chars)\n", s.Title, len(s.Title))
	if len(s.AlternativeTitles) > 0 {
		fmt.Fprintf(b, "- **Alternative titles:** %s\n", strings.Join(s.AlternativeTitles, "; "))
	}
	fmt.Fprintf(b, "- **Description:** %s\n", truncateChars(s.Description, 200))
	if len(s.Tags) > 0 {
		fmt.Fprintf(b, "- **Tags:** %s\n", strings.Join(s.Tags, ", "))
	}
	if len(s.Hashtags) > 0 {
		fmt.Fprintf(b, "- **Hashtags:** %s\n", strings.Join(s.Hashtags, " "))
	}
	if len(s.ChapterTitles) > 0 {
		b.WriteString("- **Chapters:**\n")
		for _, c := range s.ChapterTitles {
			fmt.Fprintf(b, "  - `%s` %s\n", c.Timestamp, c.Title)
		}
	}
	if len(s.Playlists) > 0 {
		fmt.Fprintf(b, "- **Playlists:** %s\n", strings.Join(s.Playlists, ", "))
	}
	if len(s.ThumbnailText) > 0 {
		fmt.Fprintf(b, "- **Thumbnail text:** %s\n", strings.Join(s.ThumbnailText, " / "))
	}
	if s.PinnedComment != "" {
		fmt.Fprintf(b, "- **Pinned comment:** %s\n", s.PinnedComment)
	}
	b.WriteString("\n")
}

func writeShorts(b *strings.Builder, s ShortsSEO) {
	if len(s.Items) == 0 {
		return
	}
	b.WriteString("---\n\n## Short-form SEO\n\n")
	for _, it := range s.Items {
		fmt.Fprintf(b, "### %s — %s\n\n", it.Platform, it.Title)
		fmt.Fprintf(b, "- **Description:** %s\n", it.Description)
		fmt.Fprintf(b, "- **Caption:** %s\n", it.Caption)
		if len(it.Hashtags) > 0 {
			fmt.Fprintf(b, "- **Hashtags:** %s\n", strings.Join(it.Hashtags, " "))
		}
		if it.EngagementPrompt != "" {
			fmt.Fprintf(b, "- **Engagement:** %s\n", it.EngagementPrompt)
		}
		if it.CTA != "" {
			fmt.Fprintf(b, "- **CTA:** %s\n", it.CTA)
		}
		b.WriteString("\n")
	}
}

func writeSocial(b *strings.Builder, s SocialSEO) {
	if len(s.Items) == 0 {
		return
	}
	b.WriteString("---\n\n## Social SEO\n\n")
	for _, it := range s.Items {
		fmt.Fprintf(b, "### %s\n\n", it.Platform)
		fmt.Fprintf(b, "- **Title:** %s\n", it.Title)
		fmt.Fprintf(b, "- **Summary:** %s\n", it.Summary)
		if len(it.Hashtags) > 0 {
			fmt.Fprintf(b, "- **Hashtags:** %s\n", strings.Join(it.Hashtags, " "))
		}
		if it.EngagementPrompt != "" {
			fmt.Fprintf(b, "- **Engagement:** %s\n", it.EngagementPrompt)
		}
		b.WriteString("\n")
	}
}

func writeKeywords(b *strings.Builder, k Keywords) {
	b.WriteString("---\n\n## Keywords\n\n")
	writeKw(b, "Primary", k.Primary)
	writeKw(b, "Secondary", k.Secondary)
	writeKw(b, "Long-tail", k.LongTail)
	writeKw(b, "AWS", k.AWS)
	writeKw(b, "Technology", k.Technology)
	writeKw(b, "Developer", k.Developer)
	b.WriteString("\n")
}

func writeKw(b *strings.Builder, label string, kw []string) {
	if len(kw) > 0 {
		fmt.Fprintf(b, "- **%s:** %s\n", label, strings.Join(kw, ", "))
	}
}

func writeHashtags(b *strings.Builder, t PlatformTags) {
	b.WriteString("---\n\n## Hashtags\n\n")
	writeKw(b, "YouTube", t.YouTube)
	writeKw(b, "TikTok", t.TikTok)
	writeKw(b, "LinkedIn", t.LinkedIn)
	writeKw(b, "X", t.X)
	b.WriteString("\n")
}

func writeOpenGraph(b *strings.Builder, og OpenGraph) {
	b.WriteString("---\n\n## Open Graph & Twitter Card\n\n")
	fmt.Fprintf(b, "- `og:title` %s\n", og.Title)
	fmt.Fprintf(b, "- `og:description` %s\n", og.Description)
	fmt.Fprintf(b, "- `og:type` %s · `og:site_name` %s · `og:locale` %s\n", og.Type, og.SiteName, og.Locale)
	fmt.Fprintf(b, "- `og:image` %s\n", og.ImageGuidance)
	fmt.Fprintf(b, "- `twitter:card` %s · `twitter:title` %s\n", og.Twitter.Card, og.Twitter.Title)
	b.WriteString("\n")
}

func writeStructured(b *strings.Builder, sd StructuredData) {
	b.WriteString("---\n\n## Structured Data\n\n")
	fmt.Fprintf(b, "- **Schema type:** %s\n", sd.SchemaType)
	fmt.Fprintf(b, "- **RSS:** %s — %s\n", sd.RSS.Title, sd.RSS.Description)
	fmt.Fprintf(b, "- **Sitemap:** changefreq %s, priority %s\n\n", sd.Sitemap.ChangeFreq, sd.Sitemap.Priority)
	b.WriteString("**JSON-LD:**\n\n```json\n")
	writeJSONLD(b, sd.JSONLD)
	b.WriteString("\n```\n\n")
}

// writeJSONLD renders the JSON-LD object as valid, indented JSON. The previous
// hand-rolled renderer used %v, which emitted unquoted strings and Go's
// map[...] / [slice] syntax for nested values — invalid JSON that failed the
// Google Rich Results Test. encoding/json quotes correctly and sorts map keys,
// so the output stays valid AND deterministic.
func writeJSONLD(b *strings.Builder, ld map[string]interface{}) {
	out, err := json.MarshalIndent(ld, "", "  ")
	if err != nil {
		b.WriteString("{}")
		return
	}
	b.Write(out)
}

func writeIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Content Intelligence\n\n")
	fmt.Fprintf(b, "- **Audience:** %s · **Difficulty:** %s\n", ci.Audience, ci.Difficulty)
	fmt.Fprintf(b, "- **Topic:** %s · **Category:** %s\n", ci.Topic, ci.Category)
	fmt.Fprintf(b, "- **Content type:** %s\n", ci.ContentType)
	if len(ci.CloudServices) > 0 {
		fmt.Fprintf(b, "- **Cloud services:** %s\n", strings.Join(ci.CloudServices, ", "))
	}
	if len(ci.ProgrammingLanguages) > 0 {
		fmt.Fprintf(b, "- **Languages:** %s\n", strings.Join(ci.ProgrammingLanguages, ", "))
	}
	fmt.Fprintf(b, "- **Reading time:** %s · **Watch time:** %s\n", ci.EstimatedReadingTime, firstNonEmpty(ci.EstimatedWatchTime, "—"))
	fmt.Fprintf(b, "- **Search intent:** %s\n", ci.SearchIntent)
	fmt.Fprintf(b, "- **SEO confidence:** %d/100\n\n", ci.SEOConfidenceScore)
}

func codeJoin(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = "`" + s + "`"
	}
	return strings.Join(q, ", ")
}
