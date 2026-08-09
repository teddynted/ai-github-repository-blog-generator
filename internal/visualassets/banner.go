package visualassets

// bannerStyle styles promotional banners, release cards, and repository hero
// images: announcement-grade, confident, with a clear headline zone.
func bannerStyle(c candidate, b Branding) Style {
	comp := "wide banner composition with a central-to-right focal graphic and a generous headline zone, announcement-grade framing"
	if c.AspectRatio == "1:1" {
		comp = "centered square composition with a strong focal graphic, headline zone reserved along the lower third"
	}
	return Style{
		Composition:    comp,
		Perspective:    "hero isometric or gentle three-quarter view",
		Lighting:       b.Lighting,
		Mood:           "celebratory but professional, confident release energy",
		ColorPalette:   palette(b),
		Style:          "bold flat vector with layered depth, " + b.IllustrationStyle,
		TechnicalFocus: firstNonEmpty(c.Focus, "the system"),
	}
}
