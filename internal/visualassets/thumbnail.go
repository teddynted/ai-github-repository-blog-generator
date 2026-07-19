package visualassets

// thumbnailStyle styles 16:9 YouTube thumbnails: one strong focal point, bold
// hierarchy, minimal clutter, and a clear headline zone. Grounded technical
// focus comes from the candidate.
func thumbnailStyle(c candidate, b Branding) Style {
	return Style{
		Composition:    "single bold focal subject offset to the right, large empty headline zone on the left third, strong visual hierarchy, minimal clutter",
		Perspective:    "slight isometric hero angle for depth",
		Lighting:       b.Lighting,
		Mood:           "high-energy, credible, click-worthy without being clickbait",
		ColorPalette:   palette(b),
		Style:          "bold flat vector with subtle depth, high contrast, " + b.IllustrationStyle,
		TechnicalFocus: firstNonEmpty(c.Focus, "the release at a glance"),
	}
}
