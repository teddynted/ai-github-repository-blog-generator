package visualassets

import (
	"fmt"
	"strings"
)

// Markdown renders the collection as a production-ready document: a brand
// guideline block, then one section per asset with purpose, prompt, negative
// prompt, style, branding, and composition notes.
func (col VisualAssetCollection) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Visual Assets: %s\n\n", firstNonEmpty(col.Metadata.SourceBlogTitle, col.Metadata.Repository))
	fmt.Fprintf(&b, "_%s · release %s · %d assets_\n\n",
		col.Metadata.Repository, col.Metadata.Release, col.Metadata.AssetCount)

	if len(col.Warnings) > 0 {
		fmt.Fprintf(&b, "> **Notes:** %s\n\n", strings.Join(col.Warnings, "; "))
	}

	writeBranding(&b, col.Branding)
	writeSharedConstraints(&b)

	for _, a := range col.Assets {
		writeAsset(&b, a, col.Metadata.Release)
	}

	writeIntelligence(&b, col.ContentIntelligence)
	writeValidationMatrix(&b, col.Assets)
	return b.String()
}

func writeBranding(b *strings.Builder, br Branding) {
	b.WriteString("## Brand Guidelines\n\n")
	fmt.Fprintf(b, "- **Primary colors:** %s\n", strings.Join(br.PrimaryColors, ", "))
	fmt.Fprintf(b, "- **Accent colors:** %s\n", strings.Join(br.AccentColors, ", "))
	fmt.Fprintf(b, "- **Illustration style:** %s\n", br.IllustrationStyle)
	fmt.Fprintf(b, "- **Icon style:** %s\n", br.IconStyle)
	fmt.Fprintf(b, "- **Background:** %s\n", br.BackgroundStyle)
	fmt.Fprintf(b, "- **Lighting:** %s · **Depth:** %s\n", br.Lighting, br.Depth)
	fmt.Fprintf(b, "- **Typography placement:** %s\n", br.TypographyPlacement)
	fmt.Fprintf(b, "- **Spacing:** %s\n", br.Spacing)
	fmt.Fprintf(b, "- **Visual tone:** %s\n\n", br.VisualTone)
}

func writeAsset(b *strings.Builder, a Asset, release string) {
	fmt.Fprintf(b, "---\n\n## %s\n\n", a.Type)
	fmt.Fprintf(b, "- **Platform:** %s · **Aspect ratio:** %s", a.Platform, a.AspectRatio)
	if a.Dimensions != "" {
		fmt.Fprintf(b, " (%s)", a.Dimensions)
	}
	b.WriteString("\n")
	fmt.Fprintf(b, "- **Purpose:** %s\n", a.Purpose)
	fmt.Fprintf(b, "- **Recommended filename:** `%s`\n", a.Metadata.RecommendedFilename)

	fmt.Fprintf(b, "\n### Prompt\n\n```text\n%s\n```\n\n", a.Prompt)

	if a.NegativePrompt != "" {
		fmt.Fprintf(b, "### Negative Prompt\n\n```text\n%s\n```\n\n", a.NegativePrompt)
	}

	b.WriteString("### Composition Notes\n\n")
	fmt.Fprintf(b, "- **Composition:** %s\n", a.Style.Composition)
	fmt.Fprintf(b, "- **Perspective:** %s · **Lighting:** %s\n", a.Style.Perspective, a.Style.Lighting)
	fmt.Fprintf(b, "- **Mood:** %s · **Technical focus:** %s\n", a.Style.Mood, a.Style.TechnicalFocus)
	fmt.Fprintf(b, "- **Palette:** %s\n", strings.Join(a.Style.ColorPalette, ", "))
	if len(a.TextPlaceholders) > 0 {
		parts := make([]string, len(a.TextPlaceholders))
		for i, p := range a.TextPlaceholders {
			parts[i] = p.Area + " → " + p.Purpose
		}
		fmt.Fprintf(b, "- **Text placeholders (render no text):** %s\n", strings.Join(parts, "; "))
	}
	if len(a.References) > 0 {
		fmt.Fprintf(b, "- **Grounded in:** %s\n", strings.Join(a.References, ", "))
	}

	b.WriteString("\n### Quality Checklist\n\n")
	for _, c := range qualityChecklist() {
		fmt.Fprintf(b, "- %s\n", c)
	}

	complexity, reliability, models := renderGuidance(a.Type)
	b.WriteString("\n### Render Guidance\n\n")
	fmt.Fprintf(b, "- **Complexity:** %s · **Reliability:** %d/5 · **Best suited for:** %s\n",
		complexity, reliability, strings.Join(models, ", "))

	if notes := platformNotes(a.Type); len(notes) > 0 {
		b.WriteString("\n### Platform Optimization\n\n")
		for _, n := range notes {
			fmt.Fprintf(b, "- %s\n", n)
		}
	}

	if v := diagrammaticVariant(a.Type); v != "" {
		fmt.Fprintf(b, "\n### Diagrammatic Variant\n\n```text\n%s\n```\n", v)
	}

	if v := compactVariant(a.Type); v != "" {
		fmt.Fprintf(b, "\n### Compact Prompt Variant\n\n```text\n%s\n```\n", v)
	}

	fmt.Fprintf(b, "\n### Automation Metadata\n\n```yaml\n%s\n```\n", automationMeta(a.Type, release))

	if m := motionHandoff(a.Type); m != "" {
		fmt.Fprintf(b, "\n### Motion Handoff\n\n```yaml\n%s\n```\n", m)
	}

	b.WriteString("\n")
}

// writeValidationMatrix appends the end-of-document readiness matrix (#6),
// computed from each asset's existing content.
func writeValidationMatrix(b *strings.Builder, assets []Asset) {
	b.WriteString("---\n\n# Final Validation Matrix\n\n")
	b.WriteString("| Asset | Text-safe zones | Mobile-safe | Docs-safe | Animation-ready |\n")
	b.WriteString("| --- | :---: | :---: | :---: | :---: |\n")
	for _, a := range assets {
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n",
			a.Type, yn(len(a.TextPlaceholders) > 0), yn(mobileSafe(a.Type)), yn(docsSafe(a.Type)), yn(motionHandoff(a.Type) != ""))
	}
	b.WriteString("\n")
}

func yn(v bool) string {
	if v {
		return "✅"
	}
	return "—"
}

func mobileSafe(assetType string) bool {
	c, _, _ := renderGuidance(assetType)
	return len(platformNotes(assetType)) > 0 || c != "High"
}

func docsSafe(assetType string) bool {
	for _, k := range []string{"Architecture", "Workflow", "Blog", "Dev.to", "Medium", "Hero", "GitHub"} {
		if strings.Contains(assetType, k) {
			return true
		}
	}
	return false
}

// writeSharedConstraints renders the reusable render rules once, so each asset
// prompt can reference them implicitly instead of repeating the boilerplate.
func writeSharedConstraints(b *strings.Builder) {
	b.WriteString("---\n\n## Shared Render Constraints\n\n")
	b.WriteString("_Applied to every asset below — referenced by each prompt, not repeated verbatim._\n\n")
	for _, c := range sharedRenderConstraints() {
		fmt.Fprintf(b, "- %s\n", c)
	}
	b.WriteString("\n")
}

func writeIntelligence(b *strings.Builder, ci Intelligence) {
	b.WriteString("---\n\n## Collection Intelligence\n\n")
	fmt.Fprintf(b, "- **Assets:** %d · **Visual complexity:** %s · **Est. cost:** %s\n", ci.AssetCount, ci.VisualComplexity, ci.EstimatedCost)
	fmt.Fprintf(b, "- **Audience:** %s · **Difficulty:** %s\n", ci.Audience, ci.Difficulty)
	if len(ci.Platforms) > 0 {
		fmt.Fprintf(b, "- **Platforms:** %s\n", strings.Join(ci.Platforms, ", "))
	}
	if len(ci.SEOKeywords) > 0 {
		fmt.Fprintf(b, "- **SEO keywords:** %s\n", strings.Join(ci.SEOKeywords, ", "))
	}
	if len(ci.ProductionNotes) > 0 {
		b.WriteString("- **Production notes:**\n")
		for _, n := range ci.ProductionNotes {
			fmt.Fprintf(b, "  - %s\n", n)
		}
	}
	b.WriteString("\n")
}
