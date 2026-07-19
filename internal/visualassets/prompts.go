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
	fmt.Fprintf(&sb, " Composition: %s.", style.Composition)
	fmt.Fprintf(&sb, " Perspective: %s.", style.Perspective)
	fmt.Fprintf(&sb, " Lighting: %s.", style.Lighting)
	fmt.Fprintf(&sb, " Mood: %s.", style.Mood)
	fmt.Fprintf(&sb, " Style: %s.", style.Style)
	if len(style.ColorPalette) > 0 {
		fmt.Fprintf(&sb, " Color palette: %s.", strings.Join(style.ColorPalette, ", "))
	}
	fmt.Fprintf(&sb, " Visual tone: %s. Depth: %s.", b.VisualTone, b.Depth)

	// Text placeholders — the image renders NO literal text.
	if len(placeholders) > 0 {
		zones := make([]string, 0, len(placeholders))
		for _, p := range placeholders {
			zones = append(zones, fmt.Sprintf("%s for the %s", p.Area, p.Purpose))
		}
		fmt.Fprintf(&sb, " Reserve empty space: %s.", joinAnd(zones))
	}
	sb.WriteString(" Render NO text, letters, numbers, logos, or watermarks — leave the reserved areas clean for a compositor to add copy.")

	fmt.Fprintf(&sb, " Target format: %s aspect ratio", c.AspectRatio)
	if c.Dimensions != "" {
		fmt.Fprintf(&sb, " (%s)", c.Dimensions)
	}
	sb.WriteString(".")
	return collapse(sb.String())
}

func promptRewrite(c candidate, draft string) string {
	return fmt.Sprintf(
		"You are an art director writing a single, vivid AI image-generation prompt for a %s (%s, %s).\n\n"+
			"Rewrite the DRAFT into one fluent, richly descriptive prompt that works across image models "+
			"(GPT Image, DALL·E, Stable Diffusion, Midjourney, Nova Canvas, Flux). Keep every concrete detail "+
			"(composition, palette, aspect ratio, reserved text zones). Add NO new architecture, services, or "+
			"facts, and instruct that NO literal text is rendered. Output only the prompt.\n\nDRAFT:\n%s",
		c.Type, c.Platform, c.AspectRatio, draft)
}
