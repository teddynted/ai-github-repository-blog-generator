package visualassets

import (
	"fmt"
	"strings"
)

// snakeID turns an asset type into a stable snake_case id for automation
// ("YouTube Thumbnail" -> "youtube_thumbnail").
func snakeID(s string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// primaryUse classifies an asset's main channel for automation routing (#3).
func primaryUse(assetType string) string {
	switch {
	case strings.Contains(assetType, "YouTube"), strings.Contains(assetType, "TikTok"), strings.Contains(assetType, "Shorts"):
		return "video"
	case strings.Contains(assetType, "Architecture"), strings.Contains(assetType, "Workflow"):
		return "docs"
	case strings.Contains(assetType, "Repository"), strings.Contains(assetType, "GitHub"):
		return "repository"
	default:
		return "social"
	}
}

// renderPriority maps an asset to a coarse render priority (#3).
func renderPriority(assetType string) string {
	switch {
	case strings.Contains(assetType, "Thumbnail"), strings.Contains(assetType, "Hero"), strings.Contains(assetType, "Architecture"):
		return "high"
	case strings.Contains(assetType, "Cover"), strings.Contains(assetType, "Social"),
		strings.Contains(assetType, "Banner"), strings.Contains(assetType, "Promotional"), strings.Contains(assetType, "Workflow"):
		return "medium"
	default:
		return "low"
	}
}

// automationMeta renders the machine-readable per-asset metadata block (#3).
func automationMeta(assetType, release string) string {
	if release == "" {
		release = "v0.0.0"
	}
	return fmt.Sprintf(
		"asset_id: %s\nversion: %s\ntheme: event_driven_architecture\nrender_priority: %s\nprimary_use: %s\nsupports_motion: %t",
		snakeID(assetType), release, renderPriority(assetType), primaryUse(assetType), motionHandoff(assetType) != "")
}

// motionHandoff returns optional animation-handoff metadata for the assets that
// are animated by downstream tooling (#4). Empty for the rest.
func motionHandoff(assetType string) string {
	switch assetType {
	case "YouTube Thumbnail", "GitHub Social Card", "X Image", "YouTube Shorts Cover", "TikTok Cover":
		return "motion_handoff:\n  parallax_layers: 4\n  animate_connectors: true\n  animate_pulse_dots: true\n  safe_crop_center: true\n  preferred_zoom_anchor: orchestration_hub"
	}
	return ""
}

// isHeroAsset reports whether an asset is a hero-style visual that warrants a
// compact diffusion-model prompt variant (#5).
func isHeroAsset(assetType string) bool {
	return depictsArchitecture(assetType) || strings.Contains(assetType, "Promotional")
}

// compactVariant is a <40-word prompt tuned for Flux / SDXL / Stable Diffusion,
// offered for hero-style assets (#5).
func compactVariant(assetType string) string {
	if !isHeroAsset(assetType) {
		return ""
	}
	return "event-driven AWS-native architecture, four conceptual modules, orchestration hub focal point, " +
		"dark navy background, amber and blue accents, flat vector, subtle isometric depth, clean connectors, " +
		"strong silhouette, empty headline space, high contrast, professional cloud infrastructure illustration"
}

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

// platformNotes returns per-platform composition optimizations for an asset (#3),
// tuned to where the image is actually consumed. Empty for platforms with no
// distinctive constraint.
func platformNotes(assetType string) []string {
	switch {
	case strings.Contains(assetType, "YouTube Thumbnail"):
		return []string{
			"One dominant focal object; extreme silhouette readability at 120px",
			"Strong warm/cool contrast with clear depth separation",
			"Avoid fine connector details that disappear on mobile",
		}
	case strings.Contains(assetType, "GitHub Social Card"):
		return []string{
			"Reads cleanly in GitHub dark-mode preview",
			"Legible when embedded in Slack, Discord, and X link previews",
			"Strong center-right focal cluster",
		}
	case strings.Contains(assetType, "LinkedIn Banner"):
		return []string{
			"Survives professional-feed compression on desktop and mobile",
			"Keep important detail out of the top-left profile-photo overlap area",
			"Respect desktop and mobile banner safe zones",
		}
	case strings.Contains(assetType, "TikTok") || strings.Contains(assetType, "Shorts"):
		return []string{
			"Keep the center 40% vertical band clear of platform UI overlays; avoid the right-edge interaction rail",
			"Large simple shapes; high-contrast focal cluster centered and readable at small preview sizes",
			"Reduced architectural complexity vs. desktop assets for small-screen viewing",
		}
	default:
		return nil
	}
}

// diagrammaticVariant returns a documentation-first, diagrammatic alternative
// prompt for architecture/workflow assets (#7) — offered alongside the artistic
// version. Empty for non-diagram assets.
func diagrammaticVariant(assetType string) string {
	if !strings.Contains(assetType, "Architecture") && !strings.Contains(assetType, "Workflow") {
		return ""
	}
	return "A documentation-first diagrammatic version of the same architecture: strict left-to-right flow, " +
		"evenly spaced nodes with a clear directional arrow hierarchy, and minimal decorative elements. " +
		"Leave each component block's interior empty and label-safe (render no text — interiors stay clean for " +
		"labels added in compositing). Prioritise legibility over style: flat vector, brand palette, generous " +
		"spacing, high contrast between nodes and background."
}
