package tiktok

import (
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/shorts"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// ReleasePackage bundles every upstream artifact the TikTok Generator adapts.
// The generator never regenerates these — it adapts them into TikTok-native
// videos. Context is required (the ground truth); the rest enrich discovery and
// grounding, and the YouTube Shorts drive adaptation when present.
type ReleasePackage struct {
	Context    *rc.ReleaseContext        // Milestone 2 — the ground truth
	Blog       releasegen.BlogPost       // Milestone 3
	Storyboard storyboard.Storyboard     // Milestone 4
	VoiceOver  voiceover.VoiceOverScript // Milestone 5
	YouTube    youtube.YouTubeScript     // Milestone 6
	Shorts     shorts.ShortsCollection   // Milestone 7 — adapted 1:1 into TikToks when present
}
