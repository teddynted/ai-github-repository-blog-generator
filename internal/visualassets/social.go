package visualassets

// socialStyle styles social graphics (GitHub card, LinkedIn banner, X image).
// Professional, modern, readable, and brand-consistent for sharing technical
// content.
func socialStyle(c candidate, b Branding) Style {
	mood := "professional, modern, trustworthy"
	if c.Platform == "X" {
		mood = "punchy, modern, shareable"
	}
	return Style{
		Composition:    "balanced composition with a clear focal graphic on one side and a reserved text zone on the other, brand-consistent framing",
		Perspective:    "clean front or gentle isometric view",
		Lighting:       b.Lighting,
		Mood:           mood,
		ColorPalette:   palette(b),
		Style:          "polished flat vector, " + b.IllustrationStyle,
		TechnicalFocus: firstNonEmpty(c.Focus, "the system"),
	}
}
