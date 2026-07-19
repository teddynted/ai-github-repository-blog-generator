package visualassets

// illustrationStyle styles technical illustrations (architecture, AWS workflow)
// and blog/cover art. Clear, diagrammatic, and educational — it complements
// documentation rather than decorating it.
func illustrationStyle(c candidate, b Branding) Style {
	switch c.Category {
	case catIllustration:
		return Style{
			Composition:    "clear left-to-right technical diagram flow, labelled component blocks connected by directional arrows, balanced spacing, no text baked in",
			Perspective:    "clean isometric or flat top-down schematic",
			Lighting:       "even, diagrammatic lighting with soft shadows for layering",
			Mood:           "clear, educational, precise",
			ColorPalette:   palette(b),
			Style:          "isometric technical illustration, " + b.IllustrationStyle,
			TechnicalFocus: firstNonEmpty(c.Focus, "component relationships"),
		}
	default: // catBlog
		return Style{
			Composition:    "editorial header composition, a conceptual technical scene with a clear focal point and calm negative space for a title",
			Perspective:    "gentle isometric or layered flat scene",
			Lighting:       b.Lighting,
			Mood:           "thoughtful, technical, inviting",
			ColorPalette:   palette(b),
			Style:          "conceptual flat vector illustration, " + b.IllustrationStyle,
			TechnicalFocus: firstNonEmpty(c.Focus, "the technical story"),
		}
	}
}
