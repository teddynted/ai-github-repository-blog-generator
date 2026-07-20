package architecture

import (
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
)

// ReleasePackage bundles the artifacts the Architecture Generator consumes. The
// generator never regenerates repository knowledge — it visualizes what the
// Release Context already captured. Context is required (the ground truth); Blog
// and Storyboard enrich titles and descriptions.
type ReleasePackage struct {
	Context    *rc.ReleaseContext    // Milestone 2 — the ground truth
	Blog       releasegen.BlogPost   // Milestone 3
	Storyboard storyboard.Storyboard // Milestone 4
}
