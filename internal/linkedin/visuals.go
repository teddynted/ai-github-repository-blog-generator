package linkedin

// planVisualRefs references previously generated visual assets and architecture
// diagrams — it never regenerates images. Post types prefer the most relevant
// asset (architecture posts → the architecture diagram/illustration; others →
// the LinkedIn banner / release graphic).
func planVisualRefs(pkg ReleasePackage, c postCandidate) []VisualRef {
	var refs []VisualRef

	switch c.Type {
	case "Architecture Deep Dive", "AWS Best Practice", "Performance Improvement":
		if d := firstArchDiagram(pkg); d != "" {
			refs = append(refs, VisualRef{Type: "Architecture Diagram", Reference: d, Source: "Architecture (M11)"})
		}
		if a := visualAssetByType(pkg, "Architecture Illustration"); a != "" {
			refs = append(refs, VisualRef{Type: "Architecture Illustration", Reference: a, Source: "Visual Assets (M9)"})
		}
	case "AI Engineering Highlight", "Behind-the-Build", "Release Announcement":
		if a := visualAssetByType(pkg, "LinkedIn Banner"); a != "" {
			refs = append(refs, VisualRef{Type: "LinkedIn Banner", Reference: a, Source: "Visual Assets (M9)"})
		}
		if a := visualAssetByType(pkg, "Release Card"); a != "" {
			refs = append(refs, VisualRef{Type: "Release Card", Reference: a, Source: "Visual Assets (M9)"})
		}
	default:
		if a := visualAssetByType(pkg, "LinkedIn Banner"); a != "" {
			refs = append(refs, VisualRef{Type: "LinkedIn Banner", Reference: a, Source: "Visual Assets (M9)"})
		}
	}

	// Always offer the repository hero / YouTube thumbnail as a fallback visual.
	if len(refs) == 0 {
		if a := visualAssetByType(pkg, "YouTube Thumbnail"); a != "" {
			refs = append(refs, VisualRef{Type: "YouTube Thumbnail", Reference: a, Source: "Visual Assets (M9)"})
		} else if a := visualAssetByType(pkg, "Repository Hero Image"); a != "" {
			refs = append(refs, VisualRef{Type: "Repository Hero Image", Reference: a, Source: "Visual Assets (M9)"})
		}
	}
	return refs
}

// visualAssetByType returns the recommended filename of a visual asset of the
// given type, or "".
func visualAssetByType(pkg ReleasePackage, typ string) string {
	for _, a := range pkg.VisualAssets.Assets {
		if a.Type == typ {
			return firstNonEmpty(a.Metadata.RecommendedFilename, a.Title)
		}
	}
	return ""
}

// firstArchDiagram returns the title of the first architecture diagram, or "".
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
