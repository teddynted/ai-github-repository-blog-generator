package seo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// Generator produces the canonical SEO metadata for a release by aggregating and
// normalizing the content pipeline's artifacts. It reuses the shared
// releasegen.Model port; when Model is nil, generation is fully deterministic
// (metadata is drawn from the artifacts and the Release Context). It never
// regenerates repository knowledge and never invents features.
type Generator struct {
	Model  releasegen.Model
	Now    func() time.Time
	Logger *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// SEO converts a ReleasePackage into structured, platform-specific SEO metadata
// for blogs, YouTube, short-form, social, Open Graph, and structured data.
// Slugs, excerpts, keywords, hashtags, Open Graph, structured data, limits, and
// validation are deterministic; the Model only polishes titles, descriptions,
// and excerpts. Every value is grounded in the Release Context and artifacts.
func (g *Generator) SEO(ctx context.Context, pkg ReleasePackage) (SEOMetadata, error) {
	if pkg.Context == nil {
		return SEOMetadata{}, fmt.Errorf("seo: release context is required")
	}

	generatedAt := g.now().Format(time.RFC3339)
	keywords := planKeywords(pkg)
	tags := planHashtags(pkg, keywords)

	m := SEOMetadata{
		SchemaVersion: SchemaVersion,
		Metadata: Metadata{
			Repository:      repoName(pkg),
			Release:         releaseTag(pkg),
			SourceBlogTitle: pkg.Blog.Title,
			GeneratedAt:     generatedAt,
			SourceSchemas:   sourceSchemas(pkg),
		},
		Keywords: keywords,
		Hashtags: tags,
	}

	m.Blog = g.planBlog(ctx, pkg, keywords)
	m.YouTube = planYouTube(pkg, keywords)
	m.Shorts = planShorts(pkg, keywords)
	m.Social = planSocial(pkg, keywords, tags)
	m.OpenGraph = planOpenGraph(pkg, m.Blog)
	m.StructuredData = planStructuredData(pkg, m.Blog, keywords, generatedAt)
	m.ContentIntelligence = planIntelligence(pkg, m)
	m.Warnings = limitWarnings(m)

	if g.Logger != nil {
		g.Logger.Info("seo metadata generated",
			slog.String("repository", m.Metadata.Repository),
			slog.String("release", m.Metadata.Release),
			slog.Int("primary_keywords", len(m.Keywords.Primary)),
			slog.Int("confidence", m.ContentIntelligence.SEOConfidenceScore),
			slog.Int("warnings", len(m.Warnings)),
		)
	}
	return m, nil
}

func sourceSchemas(pkg ReleasePackage) map[string]string {
	m := map[string]string{"releaseContext": pkg.Context.SchemaVersion}
	set := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	set("storyboard", pkg.Storyboard.SchemaVersion)
	set("voiceOver", pkg.VoiceOver.SchemaVersion)
	set("youTube", pkg.YouTube.SchemaVersion)
	set("shorts", pkg.Shorts.SchemaVersion)
	set("tikTok", pkg.TikTok.SchemaVersion)
	set("visualAssets", pkg.VisualAssets.SchemaVersion)
	return m
}

// limitWarnings surfaces soft limit breaches (recommended, not fatal).
func limitWarnings(m SEOMetadata) []string {
	var w []string
	if l := len(m.Blog.MetaDescription); l > BlogDescMax {
		w = append(w, fmt.Sprintf("blog meta description is %d chars (recommended ≤ %d)", l, BlogDescMax))
	}
	if l := len(m.YouTube.Title); l > YouTubeTitlePref {
		w = append(w, fmt.Sprintf("YouTube title is %d chars; ≤ %d is preferred for full display (hard limit %d)", l, YouTubeTitlePref, YouTubeTitleMax))
	}
	if l := len(m.Blog.Title); l > 65 {
		w = append(w, fmt.Sprintf("blog SEO title is %d chars; ≤ 60 avoids search-snippet truncation", l))
	}
	return w
}
