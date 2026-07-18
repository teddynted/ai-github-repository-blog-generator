package youtube

import (
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
)

// ReleasePackage bundles every upstream artifact the YouTube Script Generator
// consumes. The generator never regenerates these — it composes them into a
// long-form script. Context is required (it is the ground truth); Blog,
// Storyboard, and VoiceOver enrich the result.
type ReleasePackage struct {
	Context    *rc.ReleaseContext        // Milestone 2 — the ground truth
	Blog       releasegen.BlogPost       // Milestone 3
	Storyboard storyboard.Storyboard     // Milestone 4 — drives chapter structure & timing
	VoiceOver  voiceover.VoiceOverScript // Milestone 5 — drives chapter narration
}
