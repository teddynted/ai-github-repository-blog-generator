package visualassets

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
)

// SDXLModelTarget is the Replicate model these assets are tuned for.
const SDXLModelTarget = "stability-ai/sdxl"

// SharedSDXLStylePrompt is the reusable style anchor every asset prompt builds
// on, so releases stay on-brand without repeating style boilerplate. Prepend it
// to an asset's focal description; keep the per-asset part concise (SDXL performs
// best at a clear subject + composition, not adjective stacking).
const SharedSDXLStylePrompt = "Polished flat-vector technical illustration, subtle isometric depth, clean geometric cloud-infrastructure shapes, layered flat planes with soft drop shadows, dark navy gradient background with faint grid texture, high contrast using #0B1F33, #12263A, #FF9900, and #4F9DFF, modern AWS-native engineering aesthetic, generous negative space, crisp edges, minimal clutter, no photorealistic people, no text, no logos, no watermarks."

// SharedSDXLQualitySuffix is the mandatory cinematic quality modifier appended
// to every SDXL asset prompt so the output reads as art-directed, enterprise
// production work rather than a generic render.
const SharedSDXLQualitySuffix = "cinematic lighting, volumetric depth, soft global illumination, crisp clean vector edges, ultra-detailed, professional enterprise technical illustration, high production value, art-directed color grading, sharp focus, 4k, award-winning design"

// SharedSDXLNegativePrompt is the reusable negative prompt applied to every
// asset — focused, so SDXL avoids the specific failure modes that hurt these
// illustrations.
const SharedSDXLNegativePrompt = "text, words, letters, numbers, labels, captions, writing, typography, lettering, gibberish text, UI text, screen text, dashboards, terminal text, code, source code, logos, watermarks, signatures, clutter, cluttered, low detail, excessive visual noise, distorted perspective, low resolution, blurry, blurry details, photorealistic humans, unrelated cloud services, busy background, tangled connectors, unreadable shapes"

// SDXLParams are the Replicate stability-ai/sdxl generation parameters for one
// asset. The JSON tags mirror the Replicate input names so the block can be fed
// to the API (or a Step Functions / Lambda injector) directly.
type SDXLParams struct {
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	Scheduler         string  `json:"scheduler"`
	NumInferenceSteps int     `json:"num_inference_steps"`
	GuidanceScale     float64 `json:"guidance_scale"`
	HighNoiseFrac     float64 `json:"high_noise_frac"`
	Refine            string  `json:"refine"`
}

// sdxlParamsFor returns tuned SDXL parameters for an asset, keyed by its
// category (the spec's four tuning groups) at the asset's own dimensions.
func sdxlParamsFor(assetType string, w, h int) SDXLParams {
	p := SDXLParams{Width: w, Height: h, Refine: "expert_ensemble_refiner"}
	switch {
	case isArchitectureAsset(assetType):
		p.Scheduler, p.NumInferenceSteps, p.GuidanceScale, p.HighNoiseFrac = "K_DPM_2_ANCESTRAL", 40, 8.0, 0.75
	case isVerticalAsset(assetType):
		p.Scheduler, p.NumInferenceSteps, p.GuidanceScale, p.HighNoiseFrac = "K_EULER_ANCESTRAL", 32, 7.8, 0.82
	case isSocialCardAsset(assetType):
		p.Scheduler, p.NumInferenceSteps, p.GuidanceScale, p.HighNoiseFrac = "K_EULER", 30, 7.0, 0.8
	default: // YouTube / GitHub / Blog / X and the rest
		p.Scheduler, p.NumInferenceSteps, p.GuidanceScale, p.HighNoiseFrac = "K_EULER", 35, 7.5, 0.8
	}
	return p
}

func isArchitectureAsset(t string) bool {
	return strings.Contains(t, "Architecture") || strings.Contains(t, "Workflow") || strings.Contains(t, "Diagram")
}

func isVerticalAsset(t string) bool {
	return strings.Contains(t, "Shorts") || strings.Contains(t, "TikTok")
}

func isSocialCardAsset(t string) bool {
	return strings.Contains(t, "Social Card") || strings.Contains(t, "LinkedIn")
}

// compositionArchetypes are the layout patterns releases rotate through, so
// consecutive releases do not repeat the same focal arrangement.
var compositionArchetypes = []string{
	"right-weighted reveal",
	"centered orchestration hub",
	"diagonal event cascade",
	"layered infrastructure stack",
	"radial event burst",
}

// archetypeGuidance is the one-line composition instruction per archetype.
var archetypeGuidance = map[string]string{
	"right-weighted reveal":        "place the focal cluster on the right; keep the left third empty for a headline overlay.",
	"centered orchestration hub":   "put a central orchestration node with supporting planes radiating outward; reserve the lower band for overlay.",
	"diagonal event cascade":       "flow events top-left to bottom-right along a diagonal; keep the upper-left corner clear for a title.",
	"layered infrastructure stack": "stack the control plane above the compute plane in parallel horizontal layers; reserve the top for a headline.",
	"radial event burst":           "emit event paths radially from a single trigger; keep the outer margins clear for overlay text.",
}

// chooseArchetype rotates the composition archetype deterministically from the
// release and the asset index, so releases vary and consecutive assets differ,
// while a regenerate of the same release is stable.
func chooseArchetype(release string, index int) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(release))
	base := int(h.Sum32())
	return compositionArchetypes[(base+index)%len(compositionArchetypes)]
}

// parseDimensions reads an asset's "WxH" dimensions string, falling back to a
// size derived from the aspect ratio when dimensions are absent.
func parseDimensions(dimensions, aspect string) (int, int) {
	if w, h, ok := splitWH(dimensions); ok {
		return w, h
	}
	switch aspect {
	case "16:9":
		return 1280, 720
	case "9:16":
		return 768, 1344
	case "1:1":
		return 1024, 1024
	case "4:5":
		return 1080, 1350
	case "2:1":
		return 1280, 640
	}
	return 1024, 1024
}

func splitWH(s string) (int, int, bool) {
	i := strings.IndexAny(s, "x×X")
	if i <= 0 {
		return 0, 0, false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(s[:i]))
	h, err2 := strconv.Atoi(strings.TrimSpace(s[i+1:]))
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// yamlSDXLParams renders SDXL params as the Replicate input YAML block.
func yamlSDXLParams(p SDXLParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "width: %d\n", p.Width)
	fmt.Fprintf(&b, "height: %d\n", p.Height)
	fmt.Fprintf(&b, "scheduler: %s\n", p.Scheduler)
	fmt.Fprintf(&b, "num_inference_steps: %d\n", p.NumInferenceSteps)
	fmt.Fprintf(&b, "guidance_scale: %s\n", strconv.FormatFloat(p.GuidanceScale, 'f', -1, 64))
	fmt.Fprintf(&b, "high_noise_frac: %s\n", strconv.FormatFloat(p.HighNoiseFrac, 'f', -1, 64))
	fmt.Fprintf(&b, "refine: %s", p.Refine)
	return b.String()
}
