package xthread

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// defaultPostsPerThread is the default configurable thread length.
const defaultPostsPerThread = 5

// Generator produces X threads from a ReleasePackage. It reuses the shared
// releasegen.Model port; when Model is nil, generation is fully deterministic
// (posts are assembled from grounded facts). It never regenerates repository
// knowledge and never invents features or metrics.
type Generator struct {
	Model releasegen.Model
	// MaxThreads caps how many threads to produce; <= 0 uses the default (5).
	MaxThreads int
	// PostsPerThread sets the thread length; <= 0 uses the default (5), clamped 3–10.
	PostsPerThread int
	Now            func() time.Time
	Logger         *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

func (g *Generator) postsPerThread() int {
	if g.PostsPerThread > 0 {
		return clamp(g.PostsPerThread, minPosts, maxPostsPerThread)
	}
	return defaultPostsPerThread
}

// XThread converts a ReleasePackage into a collection of technical X threads,
// each targeting a distinct audience. Discovery, post composition, character-
// limit enforcement, code extraction, takeaways, engagement, CTAs, hashtags, and
// metadata are deterministic; the Model only sharpens the opening hook. Every
// thread is grounded in the Release Context.
func (g *Generator) XThread(ctx context.Context, pkg ReleasePackage) (XThreadCollection, error) {
	if pkg.Context == nil {
		return XThreadCollection{}, fmt.Errorf("xthread: release context is required")
	}

	cands := discover(pkg, g.MaxThreads)
	if len(cands) == 0 {
		return XThreadCollection{}, fmt.Errorf("xthread: no groundable threads for the release")
	}

	length := g.postsPerThread()
	collection := XThreadCollection{
		SchemaVersion: SchemaVersion,
		Metadata: Metadata{
			Repository:      repoName(pkg),
			Release:         tag(pkg),
			SourceBlogTitle: pkg.Blog.Title,
			GeneratedAt:     g.now().Format(time.RFC3339),
			PostsPerThread:  length,
			SourceSchemas:   sourceSchemas(pkg),
		},
	}

	threads := make([]Thread, 0, len(cands))
	for i, c := range cands {
		th := g.buildThread(ctx, pkg, c, length, i+1)
		th.Metadata = planThreadMeta(pkg, c, th)
		threads = append(threads, th)
	}

	collection.Threads = threads
	collection.Metadata.ThreadCount = len(threads)
	collection.ContentIntelligence = g.planIntelligence(pkg, threads)
	collection.Warnings = collectWarnings(threads)

	if g.Logger != nil {
		g.Logger.Info("x threads generated",
			slog.String("repository", collection.Metadata.Repository),
			slog.String("release", collection.Metadata.Release),
			slog.Int("threads", len(threads)),
			slog.Int("posts_per_thread", length),
			slog.Int("warnings", len(collection.Warnings)),
		)
	}
	return collection, nil
}

// hook optionally sharpens the opening post via the Model, grounded in the draft
// and re-trimmed to the character limit. Falls back to the draft.
func (g *Generator) hook(ctx context.Context, c threadCandidate, draft string) string {
	if g.Model == nil || strings.TrimSpace(draft) == "" {
		return fitChars(draft, MaxPostChars)
	}
	out, err := g.Model.Generate(ctx, hookPrompt(c, draft))
	if err != nil {
		return fitChars(draft, MaxPostChars)
	}
	if r := collapse(strings.TrimSpace(out)); r != "" {
		return fitChars(r, MaxPostChars)
	}
	return fitChars(draft, MaxPostChars)
}

func sourceSchemas(pkg ReleasePackage) map[string]string {
	m := map[string]string{"releaseContext": pkg.Context.SchemaVersion}
	set := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	set("visualAssets", pkg.VisualAssets.SchemaVersion)
	set("seo", pkg.SEO.SchemaVersion)
	set("architecture", pkg.Architecture.SchemaVersion)
	set("linkedIn", pkg.LinkedIn.SchemaVersion)
	return m
}

func collectWarnings(threads []Thread) []string {
	var w []string
	for _, th := range threads {
		if len(th.KeyTakeaways) == 0 {
			w = append(w, fmt.Sprintf("thread %d (%s) has no key takeaways; the release context may be thin", th.ID, th.Type))
		}
	}
	return w
}
