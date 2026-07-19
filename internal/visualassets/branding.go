package visualassets

// planBranding derives a consistent visual identity for the whole release. It is
// deterministic: an AWS-heavy release earns an AWS-orange accent, otherwise a
// clean developer palette. The same Branding is applied to every asset so the
// repository reads as one brand across YouTube, social, blogs, and docs.
func planBranding(pkg ReleasePackage) Branding {
	primary := []string{"#0B1F33", "#12263A"} // deep slate/navy base
	accent := []string{"#3DDC97", "#4F9DFF"}  // teal + blue developer accents

	if len(awsServices(pkg, 1)) > 0 {
		// AWS-forward releases lean on the familiar AWS orange as an accent.
		accent = []string{"#FF9900", "#4F9DFF"}
	}

	return Branding{
		PrimaryColors:       primary,
		AccentColors:        accent,
		IllustrationStyle:   "flat vector with subtle isometric depth, clean geometric shapes, technical but friendly",
		IconStyle:           "minimal line + solid hybrid icons, consistent stroke weight, rounded corners",
		BackgroundStyle:     "dark gradient with a faint grid or circuit texture, generous negative space",
		Lighting:            "soft directional key light, gentle rim light on focal shapes, no harsh shadows",
		Depth:               "layered flat planes with soft drop shadows for hierarchy",
		TypographyPlacement: "headline zone reserved but left empty; strong left or lower-third alignment",
		Spacing:             "generous margins, clear focal point, uncluttered composition",
		VisualTone:          "modern, confident, engineering-credible, approachable",
		FontStyle:           "geometric sans-serif feel (guidance only — render no text)",
	}
}
