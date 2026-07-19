package visualassets

import "strings"

// planAssetMeta builds per-asset production/SEO metadata. Visual complexity and
// the (provider-neutral) cost estimate are derived from the asset category.
func planAssetMeta(c candidate, pkg ReleasePackage) AssetMeta {
	complexity := complexityFor(c.Category)
	return AssetMeta{
		TargetAudience:      audience(pkg),
		Difficulty:          difficulty(pkg),
		Topic:               firstNonEmpty(c.Focus, c.Type),
		VisualComplexity:    complexity,
		EstimatedCost:       costFor(complexity),
		SEOKeywords:         topStrings(technicalTopics(pkg), 8),
		RecommendedFilename: recommendedFilename(pkg, c),
	}
}

func complexityFor(cat category) string {
	switch cat {
	case catIllustration:
		return "high" // labelled diagrams / workflows are detailed
	case catThumbnail, catBanner, catCover:
		return "medium"
	default:
		return "low"
	}
}

// costFor gives a provider-neutral generation cost band (not a vendor price).
func costFor(complexity string) string {
	switch complexity {
	case "high":
		return "~2 credits (detailed illustration; may need a higher-quality tier)"
	case "medium":
		return "~1 credit (standard quality)"
	default:
		return "~1 credit (standard quality)"
	}
}

func recommendedFilename(pkg ReleasePackage, c candidate) string {
	parts := []string{repoShortName(pkg), releaseTag(pkg), c.Type}
	name := filenameSlug(strings.Join(parts, " "))
	if name == "" {
		name = "visual-asset"
	}
	return name + ".png"
}

// planIntelligence aggregates collection-level metadata.
func (g *Generator) planIntelligence(pkg ReleasePackage, assets []Asset) Intelligence {
	var platforms, keywords []string
	high, med := 0, 0
	for _, a := range assets {
		platforms = append(platforms, a.Platform)
		keywords = append(keywords, a.Metadata.SEOKeywords...)
		switch a.Metadata.VisualComplexity {
		case "high":
			high++
		case "medium":
			med++
		}
	}
	overall := "low"
	switch {
	case high >= 2:
		overall = "high"
	case high+med >= 3:
		overall = "medium"
	}
	return Intelligence{
		AssetCount:       len(assets),
		Platforms:        dedupe(platforms),
		Audience:         audience(pkg),
		Difficulty:       difficulty(pkg),
		VisualComplexity: overall,
		SEOKeywords:      topStrings(dedupe(keywords), 12),
		EstimatedCost:    estimatedBatchCost(len(assets), high),
		ProductionNotes: []string{
			"Prompts are provider-neutral: usable with GPT Image, DALL·E, Stable Diffusion, Midjourney, Nova Canvas, or Flux.",
			"Images render no text — add real copy in the reserved placeholder zones during compositing.",
			"Apply the shared Branding across every asset for a consistent visual identity.",
		},
	}
}

func estimatedBatchCost(n, high int) string {
	credits := n + high // high-complexity assets count as ~2
	return "~" + itoa(credits) + " credits total (provider-neutral estimate)"
}

func audience(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.ContentIntelligence.TargetAudience != "" {
		return pkg.Context.ContentIntelligence.TargetAudience
	}
	return "Software engineers and cloud practitioners"
}

func difficulty(pkg ReleasePackage) string {
	if pkg.Context != nil {
		switch pkg.Context.ContentIntelligence.ImplementationComplexity {
		case "high":
			return "advanced"
		case "low":
			return "beginner"
		default:
			return "intermediate"
		}
	}
	return "intermediate"
}

// itoa renders a non-negative int (small helper to avoid strconv at call sites).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
