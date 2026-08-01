package visualassets

import "strings"

// conceptualRoles are the fixed, provider-neutral roles every architecture
// visual should depict, so outputs stay coherent across assets. They replace
// vague "four major components" language (#4).
var conceptualRoles = []string{
	"Identity & Access",
	"Event Routing & Scheduling",
	"Serverless Orchestration",
	"Elastic Compute & Observability",
}

// sharedRenderConstraints are the model-agnostic rules every asset shares.
// Rendered once (the "Shared Render Constraints" section) so individual prompts
// reference them implicitly instead of repeating the boilerplate (#2).
func sharedRenderConstraints() []string {
	return []string{
		"No text, letters, numbers, logos, watermarks, or signatures",
		"Flat vector illustration with subtle isometric depth",
		"Soft directional key light with gentle rim highlights; no harsh shadows",
		"Layered flat planes with soft drop shadows for depth hierarchy",
		"Clean geometric shapes, consistent stroke weight, generous negative space",
		"Brand palette only: deep navy #0B1F33 / #12263A with amber #FF9900 and blue #4F9DFF accents",
	}
}

// qualityChecklist is the per-asset legibility gate (#5).
func qualityChecklist() []string {
	return []string{
		"Single clear focal point",
		"Empty text-safe zone preserved",
		"No overlapping connector lines",
		"No tiny unreadable details",
		"Strong contrast between focal object and background",
		"Composition remains legible when scaled down",
	}
}

// renderGuidance rates an asset's complexity and reliability and recommends the
// image models best suited to it (#6), so the pipeline can route model choice.
func renderGuidance(assetType string) (complexity string, reliability int, models []string) {
	switch {
	case strings.Contains(assetType, "Architecture") || strings.Contains(assetType, "Workflow"):
		return "High", 4, []string{"GPT Image", "Flux"}
	case strings.Contains(assetType, "Thumbnail") || strings.Contains(assetType, "Cover") || strings.Contains(assetType, "Promotional"):
		return "Medium", 5, []string{"GPT Image", "Midjourney", "Flux"}
	default: // banners, social cards, headers, hero, release card
		return "Low", 5, []string{"GPT Image", "Flux", "Stable Diffusion"}
	}
}

// depictsArchitecture reports whether an asset's imagery shows the platform
// architecture, so the named conceptual roles are grounded into its prompt.
func depictsArchitecture(assetType string) bool {
	for _, k := range []string{"Architecture", "Workflow", "Thumbnail", "Hero"} {
		if strings.Contains(assetType, k) {
			return true
		}
	}
	return false
}
