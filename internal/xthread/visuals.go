package xthread

// planVisualRefs references previously generated visual assets and architecture
// diagrams — it never regenerates images.
func planVisualRefs(pkg ReleasePackage, c threadCandidate) []VisualRef {
	var refs []VisualRef
	switch c.Type {
	case "Architecture Walkthrough", "AWS Best Practices", "Performance Improvements":
		if d := firstArchDiagram(pkg); d != "" {
			refs = append(refs, VisualRef{Type: "Architecture Diagram", Reference: d, Source: "Architecture (M11)"})
		}
		if a := visualAssetByType(pkg, "Architecture Illustration"); a != "" {
			refs = append(refs, VisualRef{Type: "Architecture Illustration", Reference: a, Source: "Visual Assets (M9)"})
		}
	default:
		if a := visualAssetByType(pkg, "X Image"); a != "" {
			refs = append(refs, VisualRef{Type: "X Image", Reference: a, Source: "Visual Assets (M9)"})
		}
		if a := visualAssetByType(pkg, "Release Card"); a != "" {
			refs = append(refs, VisualRef{Type: "Release Card", Reference: a, Source: "Visual Assets (M9)"})
		}
	}
	if len(refs) == 0 {
		if a := visualAssetByType(pkg, "YouTube Thumbnail"); a != "" {
			refs = append(refs, VisualRef{Type: "YouTube Thumbnail", Reference: a, Source: "Visual Assets (M9)"})
		}
	}
	return refs
}

func visualAssetByType(pkg ReleasePackage, typ string) string {
	for _, a := range pkg.VisualAssets.Assets {
		if a.Type == typ {
			return firstNonEmpty(a.Metadata.RecommendedFilename, a.Title)
		}
	}
	return ""
}

func firstArchDiagram(pkg ReleasePackage) string {
	for _, d := range pkg.Architecture.Diagrams {
		if d.Type == "High-Level Architecture" {
			return d.Title
		}
	}
	if len(pkg.Architecture.Diagrams) > 0 {
		return pkg.Architecture.Diagrams[0].Title
	}
	return ""
}
