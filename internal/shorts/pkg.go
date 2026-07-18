package shorts

import (
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// ReleasePackage bundles every upstream artifact the Shorts Generator mines. The
// generator never regenerates these — it discovers the best moments across them
// and plans standalone Shorts. Context is required (the ground truth); the rest
// enrich discovery and grounding.
type ReleasePackage struct {
	Context    *rc.ReleaseContext        // Milestone 2 — the ground truth
	Blog       releasegen.BlogPost       // Milestone 3
	Storyboard storyboard.Storyboard     // Milestone 4
	VoiceOver  voiceover.VoiceOverScript // Milestone 5
	YouTube    youtube.YouTubeScript     // Milestone 6 — drives discovery (grounded chapters + callouts)
}
