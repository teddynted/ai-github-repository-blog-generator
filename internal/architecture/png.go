package architecture

// planPNG derives PNG export settings from the diagram's shape. Wider graphs
// (LR / many columns) export at 16:9; tall graphs at 4:3.
func planPNG(g graph) PNGExport {
	cols := layoutColumns(g)
	maxRows := 0
	for _, c := range cols {
		if len(c.ids) > maxRows {
			maxRows = len(c.ids)
		}
	}
	wide := g.Direction == "LR" || len(cols) >= 3

	if wide {
		return PNGExport{
			RecommendedResolution: "1920x1080",
			AspectRatio:           "16:9",
			CanvasWidth:           1920,
			CanvasHeight:          1080,
			DPI:                   144,
			Background:            "transparent",
			ExportSettings:        "2x scale, transparent background, 24px padding, embed fonts",
		}
	}
	return PNGExport{
		RecommendedResolution: "1600x1200",
		AspectRatio:           "4:3",
		CanvasWidth:           1600,
		CanvasHeight:          1200,
		DPI:                   144,
		Background:            "transparent",
		ExportSettings:        "2x scale, transparent background, 24px padding, embed fonts",
	}
}
