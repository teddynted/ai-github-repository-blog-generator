package linkedin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// Generator produces a LinkedIn content package from a ReleasePackage. It reuses
// the shared releasegen.Model port; when Model is nil, generation is fully
// deterministic (post bodies are assembled from grounded facts). It never
// regenerates repository knowledge and never invents features.
type Generator struct {
	Model    releasegen.Model
	MaxPosts int // <= 0 uses the default (6)
	Now      func() time.Time
	Logger   *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// LinkedIn converts a ReleasePackage into a collection of professional LinkedIn
// posts, each targeting a distinct engineering audience. Discovery, highlights,
// engagement, CTAs, hashtags, visual references, and metadata are deterministic;
// the Model only writes the post bodies. Every post is grounded in the Release
// Context.
func (g *Generator) LinkedIn(ctx context.Context, pkg ReleasePackage) (LinkedInCollection, error) {
	if pkg.Context == nil {
		return LinkedInCollection{}, fmt.Errorf("linkedin: release context is required")
	}

	cands := discover(pkg, g.MaxPosts)
	if len(cands) == 0 {
		return LinkedInCollection{}, fmt.Errorf("linkedin: no groundable posts for the release")
	}

	collection := LinkedInCollection{
		SchemaVersion: SchemaVersion,
		Metadata: Metadata{
			Repository:      repoName(pkg),
			Release:         tag(pkg),
			SourceBlogTitle: pkg.Blog.Title,
			GeneratedAt:     g.now().Format(time.RFC3339),
			SourceSchemas:   sourceSchemas(pkg),
		},
	}

	// Rotate highlights across posts so the same grounded highlight (e.g. the
	// event-driven "EventBridge → Lambda → EC2" line) doesn't lead every post.
	used := map[string]bool{}
	posts := make([]Post, 0, len(cands))
	for i, c := range cands {
		highlights := distinctHighlights(planHighlights(pkg, c.Type), used)
		posts = append(posts, g.buildPost(ctx, pkg, c, i, highlights))
	}

	collection.Posts = posts
	collection.Metadata.PostCount = len(posts)
	collection.ContentIntelligence = g.planIntelligence(pkg, posts)
	collection.Warnings = collectWarnings(posts)

	if g.Logger != nil {
		g.Logger.Info("linkedin content generated",
			slog.String("repository", collection.Metadata.Repository),
			slog.String("release", collection.Metadata.Release),
			slog.Int("posts", len(posts)),
			slog.Int("warnings", len(collection.Warnings)),
		)
	}
	return collection, nil
}

// distinctHighlights reduces cross-post repetition: it prefers highlights not yet
// used by an earlier post, but always keeps at least two so a post is never
// highlight-less, then records what it used. Capped at four (the body renders up
// to four).
func distinctHighlights(hs []string, used map[string]bool) []string {
	var fresh, seen []string
	for _, h := range hs {
		if used[strings.ToLower(collapse(h))] {
			seen = append(seen, h)
		} else {
			fresh = append(fresh, h)
		}
	}
	out := fresh
	for _, h := range seen {
		if len(out) >= 2 {
			break
		}
		out = append(out, h)
	}
	out = topStrings(out, 4)
	for _, h := range out {
		used[strings.ToLower(collapse(h))] = true
	}
	return out
}

// buildPost assembles one LinkedIn post from a discovered candidate, using the
// (already cross-post-deduped) highlights.
func (g *Generator) buildPost(ctx context.Context, pkg ReleasePackage, c postCandidate, index int, highlights []string) Post {
	engagement := engagementPrompt(c)
	cta := planCTA(pkg, c)

	post := Post{
		ID:                  index + 1,
		Type:                c.Type,
		Variation:           c.Variation,
		Audience:            c.Audience,
		Title:               title(pkg, c),
		Summary:             planSummary(pkg, c),
		TechnicalHighlights: highlights,
		EngagementPrompt:    engagement,
		CTA:                 cta,
		Hashtags:            planHashtags(pkg, c),
		VisualReferences:    planVisualRefs(pkg, c),
	}
	post.Body = g.body(ctx, pkg, c, highlights, engagement, cta)
	post.Metadata = planPostMeta(pkg, c, post)
	return post
}

func sourceSchemas(pkg ReleasePackage) map[string]string {
	m := map[string]string{"releaseContext": pkg.Context.SchemaVersion}
	set := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	set("youTube", pkg.YouTube.SchemaVersion)
	set("visualAssets", pkg.VisualAssets.SchemaVersion)
	set("seo", pkg.SEO.SchemaVersion)
	set("architecture", pkg.Architecture.SchemaVersion)
	return m
}

func collectWarnings(posts []Post) []string {
	var w []string
	for _, p := range posts {
		if len(p.TechnicalHighlights) == 0 {
			w = append(w, fmt.Sprintf("post %d (%s) has no technical highlights; the release context may be thin", p.ID, p.Type))
		}
	}
	return w
}
