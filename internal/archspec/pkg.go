package archspec

import (
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// ReleasePackage bundles what the specification generator consumes. Context is
// the ground truth (required); Blog anchors the specification to the engineering
// topic the article covers so the diagram stays focused rather than generic.
type ReleasePackage struct {
	Context *rc.ReleaseContext  // Milestone 2 — the ground truth
	Blog    releasegen.BlogPost // Milestone 3 — the article topic
}
