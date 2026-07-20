package seo

// planIntelligence builds cross-channel content intelligence, including a
// deterministic SEO confidence score that reflects how complete and grounded the
// metadata is.
func planIntelligence(pkg ReleasePackage, m SEOMetadata) Intelligence {
	watch := ""
	if pkg.YouTube.Video.Duration != "" {
		watch = pkg.YouTube.Video.Duration
	} else if pkg.YouTube.Video.DurationSec > 0 {
		watch = mmss(pkg.YouTube.Video.DurationSec)
	}

	ci := Intelligence{
		Audience:             audience(pkg),
		Difficulty:           difficulty(pkg),
		Topic:                firstSentences(featureName(pkg), 1),
		Category:             m.Blog.Category,
		ContentType:          "Technical tutorial / release deep dive",
		TechnologyStack:      technologies(pkg),
		CloudServices:        awsServices(pkg),
		ProgrammingLanguages: programmingLanguages(pkg),
		EstimatedReadingTime: m.Blog.ReadingTime,
		EstimatedWatchTime:   watch,
		SearchIntent:         "informational / how-to",
		UserIntent:           "learn how the release works and how to build something similar",
	}
	ci.SEOConfidenceScore = confidenceScore(m)
	return ci
}

// confidenceScore is a 0–100 heuristic: it rewards a complete, within-limits,
// grounded metadata set.
func confidenceScore(m SEOMetadata) int {
	score := 0
	if m.Blog.Title != "" && len(m.Blog.Title) <= 65 {
		score += 15
	}
	if m.Blog.MetaDescription != "" && len(m.Blog.MetaDescription) <= BlogDescMax {
		score += 15
	}
	if m.Blog.Slug != "" {
		score += 10
	}
	if len(m.Keywords.Primary) >= 3 {
		score += 15
	}
	if len(m.Keywords.LongTail) >= 2 {
		score += 10
	}
	if m.YouTube.Title != "" && len(m.YouTube.Title) <= YouTubeTitleMax {
		score += 10
	}
	if len(m.Hashtags.All) >= 4 {
		score += 10
	}
	if m.OpenGraph.Title != "" && m.OpenGraph.Description != "" {
		score += 10
	}
	if m.StructuredData.JSONLD != nil {
		score += 5
	}
	if score > 100 {
		score = 100
	}
	return score
}

func audience(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.ContentIntelligence.TargetAudience != "" {
		return pkg.Context.ContentIntelligence.TargetAudience
	}
	return "Software engineers, cloud, DevOps, and AI developers"
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

// mmss formats seconds as "M:SS".
func mmss(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return itoa(sec/60) + ":" + pad2(sec%60)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
