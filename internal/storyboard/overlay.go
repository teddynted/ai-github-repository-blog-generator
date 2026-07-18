package storyboard

import (
	"fmt"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// planOverlays builds on-screen overlays for a scene, grounded in the Release
// Context (AWS services, features, statistics, takeaways) — never invented.
func planOverlays(typ, title string, rctx *rc.ReleaseContext) []Overlay {
	out := []Overlay{{Kind: "Title", Text: title, Position: "lower-third", Timing: "scene start"}}

	switch typ {
	case "introduction":
		sub := rctx.Repository.FullName
		if rctx.Release.Tag != "" {
			sub += " " + rctx.Release.Tag
		}
		out = append(out, Overlay{Kind: "Subtitle", Text: sub, Position: "center", Timing: "scene start"})
	case "architecture", "diagram":
		for _, svc := range topStrings(rctx.Architecture.AWSServices, 5) {
			out = append(out, Overlay{Kind: "AWS Service Label", Text: svc, Position: "beside node", Timing: "on node highlight"})
		}
	case "cloudformation":
		if rctx.CloudFormation.Counts.Resources > 0 {
			out = append(out, Overlay{
				Kind:     "Callout",
				Text:     fmt.Sprintf("%d resources · %d serverless · %d IAM", rctx.CloudFormation.Counts.Resources, rctx.CloudFormation.Counts.Serverless, rctx.CloudFormation.Counts.IAM),
				Position: "corner", Timing: "mid-scene",
			})
		}
	case "repository":
		out = append(out, Overlay{
			Kind:     "Repository Statistic",
			Text:     fmt.Sprintf("%d commits · %d files changed", rctx.CommitStats.Analyzed, rctx.FileStats.Total),
			Position: "lower-third", Timing: "mid-scene",
		})
	case "implementation":
		for _, f := range topStrings(featureLines(rctx), 3) {
			out = append(out, Overlay{Kind: "Feature Highlight", Text: f, Position: "right", Timing: "sequential"})
		}
	case "results", "conclusion":
		for _, t := range topStrings(rctx.ContentIntelligence.TechnicalHighlights, 3) {
			out = append(out, Overlay{Kind: "Key Takeaway", Text: t, Position: "center", Timing: "sequential"})
		}
	case "lessons":
		out = append(out, Overlay{Kind: "Best Practice", Text: "Ground content in the release; keep it accurate.", Position: "center", Timing: "mid-scene"})
	}
	return out
}

// featureLines prefers CHANGELOG features, falling back to implementation notes.
func featureLines(rctx *rc.ReleaseContext) []string {
	if len(rctx.Changelog.Features) > 0 {
		return rctx.Changelog.Features
	}
	return rctx.Implementation.WhatChanged
}
