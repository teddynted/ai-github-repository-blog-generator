package linkedin

import (
	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/seo"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/shorts"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/tiktok"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/visualassets"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// ReleasePackage bundles every upstream artifact the LinkedIn Generator consumes.
// The generator never regenerates these — it composes professional posts from
// them. Context is required (the ground truth); SEO, Visual Assets, and
// Architecture supply reusable metadata and visual references.
type ReleasePackage struct {
	Context      *rc.ReleaseContext                  // Milestone 2 — the ground truth
	Blog         releasegen.BlogPost                 // Milestone 3
	Storyboard   storyboard.Storyboard               // Milestone 4
	VoiceOver    voiceover.VoiceOverScript           // Milestone 5
	YouTube      youtube.YouTubeScript               // Milestone 6
	Shorts       shorts.ShortsCollection             // Milestone 7
	TikTok       tiktok.TikTokCollection             // Milestone 8
	VisualAssets visualassets.VisualAssetCollection  // Milestone 9 — referenced, not regenerated
	SEO          seo.SEOMetadata                     // Milestone 10 — hashtags, keywords, summary
	Architecture architecture.ArchitectureCollection // Milestone 11 — diagram references
}
