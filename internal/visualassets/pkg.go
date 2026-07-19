package visualassets

import (
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/shorts"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/tiktok"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// ReleasePackage bundles every upstream artifact the Visual Asset Generator
// consumes. The generator never regenerates these — it converts them into image
// prompts. Context is required (the ground truth); the rest inform which assets
// are needed and enrich grounding (e.g. Shorts/TikTok presence adds their covers).
type ReleasePackage struct {
	Context    *rc.ReleaseContext        // Milestone 2 — the ground truth
	Blog       releasegen.BlogPost       // Milestone 3
	Storyboard storyboard.Storyboard     // Milestone 4
	VoiceOver  voiceover.VoiceOverScript // Milestone 5
	YouTube    youtube.YouTubeScript     // Milestone 6
	Shorts     shorts.ShortsCollection   // Milestone 7 — adds a Shorts cover when present
	TikTok     tiktok.TikTokCollection   // Milestone 8 — adds a TikTok cover when present
}
