package visualassets

import (
	"context"
	"fmt"
	"strings"
)

// prompt builds the image prompt for a candidate: a deterministic, grounded
// assembly and (when a Model is configured) an LLM polish into vivid, fluent
// prompt prose — never adding architecture or embedding literal text. Falls back
// to the deterministic assembly.
func (g *Generator) prompt(ctx context.Context, c candidate, style Style, b Branding, placeholders []TextPlaceholder) string {
	draft := buildPrompt(c, style, b, placeholders)
	if g.Model == nil || draft == "" {
		return draft
	}
	if out, err := g.Model.Generate(ctx, promptRewrite(c, draft)); err == nil {
		if r := collapse(strings.TrimSpace(out)); r != "" {
			return r
		}
	}
	return draft
}

// buildPrompt assembles a grounded, provider-neutral image prompt.
func buildPrompt(c candidate, style Style, b Branding, placeholders []TextPlaceholder) string {
	var sb strings.Builder

	subject := strings.TrimRight(collapse(firstNonEmpty(c.Subject, c.Focus, "a modern software project")), ". ")
	fmt.Fprintf(&sb, "%s illustration depicting %s.", c.Type, lowerFirst(subject))
	fmt.Fprintf(&sb, " Technical focus: %s.", lowerFirst(style.TechnicalFocus))
	// Ground the fixed conceptual roles when the asset depicts the architecture,
	// so every such visual shows the same four modules (#4).
	if depictsArchitecture(c.Type) {
		fmt.Fprintf(&sb, " Depict the platform's four conceptual roles as connected modules: %s.", joinAnd(conceptualRoles))
	}
	fmt.Fprintf(&sb, " Composition: %s.", style.Composition)
	fmt.Fprintf(&sb, " Perspective: %s.", style.Perspective)
	fmt.Fprintf(&sb, " Mood: %s.", style.Mood)

	// Text placeholders — the image renders NO literal text.
	if len(placeholders) > 0 {
		zones := make([]string, 0, len(placeholders))
		for _, p := range placeholders {
			zones = append(zones, fmt.Sprintf("%s for the %s", p.Area, p.Purpose))
		}
		fmt.Fprintf(&sb, " Reserve empty space: %s.", joinAnd(zones))
	}
	// Reference the shared render constraints once, instead of repeating the
	// lighting / flat-vector / palette / no-text boilerplate in every prompt (#2).
	sb.WriteString(" Apply the shared render constraints: flat vector with subtle isometric depth, soft directional lighting, brand palette, and a purely WORDLESS image — render no text, words, letters, numbers, labels, captions, UI, dashboards, code, logos, or watermarks of any kind.")
	sb.WriteString(" " + SharedSDXLQualitySuffix + ".")

	fmt.Fprintf(&sb, " Target format: %s aspect ratio", c.AspectRatio)
	if c.Dimensions != "" {
		fmt.Fprintf(&sb, " (%s)", c.Dimensions)
	}
	sb.WriteString(".")
	return collapse(sb.String())
}

func promptRewrite(c candidate, draft string) string {
	return fmt.Sprintf(
		"You are an art director writing a single AI image-generation prompt for a %s (%s, %s).\n\n"+
			"Rewrite the DRAFT into one clear prompt that renders reliably across image models (GPT Image, "+
			"DALL·E, Stable Diffusion, Midjourney, Nova Canvas, Flux). Keep every concrete detail about THIS "+
			"asset (subject, focal hierarchy, composition, perspective, aspect ratio, reserved text zones). "+
			"A shared render-constraints block already covers lighting, flat-vector style, isometric depth, "+
			"layered planes, the brand palette, and the no-text rule — reference these implicitly; do NOT "+
			"restate them.\n"+
			"Use concrete, literal visual instructions, not metaphors — avoid words like \"living\", "+
			"\"reactive\", \"constellation\", \"orbits\", \"radiates\", \"choreography\", \"heroic\", "+
			"\"celebratory energy\". Do not use provider-specific syntax (no \"--ar\", weights, or camera "+
			"jargon). Keep it under about 170 words, with no redundant adjectives. Add NO new architecture, "+
			"services, or facts, and never embed literal text. Output only the prompt.\n\nDRAFT:\n%s",
		c.Type, c.Platform, c.AspectRatio, draft)
}
