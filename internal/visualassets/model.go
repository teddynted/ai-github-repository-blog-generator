// Package visualassets is the Visual Design engine (Milestone 9). It converts
// the previously generated artifacts — Release Context (M2), Technical Blog
// (M3), Storyboard (M4), Voice-over (M5), YouTube Script (M6), YouTube Shorts
// (M7), and TikTok (M8) — into structured, provider-neutral AI IMAGE PROMPTS for
// thumbnails, social graphics, blog headers, promotional banners, and technical
// illustrations.
//
// It produces PROMPTS, not images: the output is designed to drive any image
// model (GPT Image, DALL·E, Stable Diffusion, Midjourney, Amazon Nova Canvas,
// Flux, …) without vendor lock-in. It CONSUMES the upstream artifacts and never
// regenerates repository knowledge, and it never invents architecture or
// implementation details: every prompt is grounded in the Release Context, and
// no AWS service is named unless the context names it. It is deterministic where
// it matters — asset discovery, platform/aspect selection, branding, negative
// prompts, metadata, and validation are rule-based and independently testable.
// The shared releasegen.Model port is used only to polish the creative prompt
// prose, with a deterministic fallback so nothing is fabricated.
//
// Prompts never embed literal text: headlines and labels are specified as
// reserved text placeholders so a downstream compositor adds real copy.
package visualassets

// SchemaVersion is the visual-asset document version (SemVer, additive-only) so
// future image-generation/publishing milestones extend it without breaking.
const SchemaVersion = "1.0.0"

// VisualAssetCollection is the full set of image prompts for one release.
type VisualAssetCollection struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Metadata            Metadata     `json:"metadata"`
	Branding            Branding     `json:"branding"`
	Assets              []Asset      `json:"assets"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source artifacts the prompts were derived from.
type Metadata struct {
	Repository      string            `json:"repository"`
	Release         string            `json:"release"`
	SourceBlogTitle string            `json:"sourceBlogTitle"`
	GeneratedAt     string            `json:"generatedAt"`
	AssetCount      int               `json:"assetCount"`
	SourceSchemas   map[string]string `json:"sourceSchemas,omitempty"`
}

// Branding is the release-wide visual identity applied to every asset for a
// consistent look across YouTube, social, blogs, and docs.
type Branding struct {
	PrimaryColors       []string `json:"primaryColors"` // hex
	AccentColors        []string `json:"accentColors"`
	IllustrationStyle   string   `json:"illustrationStyle"`
	IconStyle           string   `json:"iconStyle"`
	BackgroundStyle     string   `json:"backgroundStyle"`
	Lighting            string   `json:"lighting"`
	Depth               string   `json:"depth"`
	TypographyPlacement string   `json:"typographyPlacement"`
	Spacing             string   `json:"spacing"`
	VisualTone          string   `json:"visualTone"`
	FontStyle           string   `json:"fontStyle"` // guidance only — no text is rendered
}

// Asset is one image prompt for a specific platform and format.
type Asset struct {
	ID               int               `json:"id"`
	Type             string            `json:"type"` // YouTube Thumbnail | YouTube Shorts Cover | TikTok Cover | Blog Header | Medium Cover | Dev.to Cover | GitHub Social Card | LinkedIn Banner | X Image | Architecture Illustration | AWS Workflow Diagram | Promotional Graphic | Release Card | Repository Hero Image
	Platform         string            `json:"platform"`
	AspectRatio      string            `json:"aspectRatio"`
	Dimensions       string            `json:"dimensions,omitempty"`
	Title            string            `json:"title"`
	Purpose          string            `json:"purpose"`
	Prompt           string            `json:"prompt"`
	NegativePrompt   string            `json:"negativePrompt,omitempty"`
	Style            Style             `json:"style"`
	Branding         Branding          `json:"branding"`
	TextPlaceholders []TextPlaceholder `json:"textPlaceholders,omitempty"`
	References       []string          `json:"references"` // grounded artifact references
	Metadata         AssetMeta         `json:"metadata"`
}

// Style is the structured creative direction for an asset.
type Style struct {
	Composition    string   `json:"composition"`
	Perspective    string   `json:"perspective"`
	Lighting       string   `json:"lighting"`
	Mood           string   `json:"mood"`
	ColorPalette   []string `json:"colorPalette"`
	Style          string   `json:"style"` // e.g. "flat vector, isometric, technical, modern"
	TechnicalFocus string   `json:"technicalFocus"`
}

// TextPlaceholder is a reserved area for a downstream compositor to add real
// copy — the image itself renders no text.
type TextPlaceholder struct {
	Area    string `json:"area"`    // e.g. "upper-left", "lower-third", "center"
	Purpose string `json:"purpose"` // e.g. "headline", "release tag", "logo"
}

// AssetMeta is per-asset production/SEO metadata.
type AssetMeta struct {
	TargetAudience      string   `json:"targetAudience"`
	Difficulty          string   `json:"difficulty"`
	Topic               string   `json:"topic"`
	VisualComplexity    string   `json:"visualComplexity"` // low | medium | high
	EstimatedCost       string   `json:"estimatedCost"`
	SEOKeywords         []string `json:"seoKeywords,omitempty"`
	RecommendedFilename string   `json:"recommendedFilename"`
}

// Intelligence is collection-level production metadata.
type Intelligence struct {
	AssetCount       int      `json:"assetCount"`
	Platforms        []string `json:"platforms,omitempty"`
	Audience         string   `json:"audience"`
	Difficulty       string   `json:"difficulty"`
	VisualComplexity string   `json:"visualComplexity"`
	SEOKeywords      []string `json:"seoKeywords,omitempty"`
	EstimatedCost    string   `json:"estimatedCost"`
	ProductionNotes  []string `json:"productionNotes,omitempty"`
}
