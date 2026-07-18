package shorts

import (
	"fmt"
	"strings"
)

// planSEO builds per-Short publishing metadata (thumbnail text, description,
// alternative titles, suggested publish slot). Publish slots are staggered so a
// batch doesn't all drop at once.
func planSEO(c candidate, pkg ReleasePackage, index int) SEO {
	tag := releaseTag(pkg)
	return SEO{
		AlternativeTitles:    alternativeTitles(c, pkg),
		ThumbnailText:        thumbnailText(c, tag),
		Description:          shortDescription(c, pkg),
		SuggestedPublishTime: publishSlot(index),
	}
}

func alternativeTitles(c candidate, pkg ReleasePackage) []string {
	tag := releaseTag(pkg)
	switch c.Angle {
	case "Architecture Reveal":
		return []string{"The architecture explained fast", tag + " architecture in 60s"}
	case "Common Mistake":
		return []string{"Stop doing this", "The mistake to avoid"}
	case "Interesting Statistic":
		return []string{"The numbers don't lie", "By the numbers: " + tag}
	default:
		return []string{c.Title, "Quick " + strings.ToLower(c.Angle)}
	}
}

func thumbnailText(c candidate, tag string) string {
	switch c.Angle {
	case "Architecture Reveal":
		return "THE ARCHITECTURE"
	case "Common Mistake":
		return "YOU'RE DOING IT WRONG"
	case "Optimization":
		return "MADE IT FASTER"
	case "AWS Best Practice":
		return "AWS TIP"
	case "CloudFormation Tip":
		return "IaC TIP"
	case "Interesting Statistic":
		return "BY THE NUMBERS"
	default:
		return strings.ToUpper(firstNonEmpty(tag, "DEV TIP"))
	}
}

func shortDescription(c candidate, pkg ReleasePackage) string {
	var b strings.Builder
	b.WriteString(firstSentences(c.Seed, 1))
	if url := repoURL(pkg); url != "" {
		fmt.Fprintf(&b, " Full video + repo in the description. %s", url)
	}
	return collapse(b.String())
}

// publishSlot staggers a batch across peak developer-viewing windows.
func publishSlot(index int) string {
	slots := []string{
		"Tue 9:00am (local)", "Wed 12:30pm (local)", "Thu 5:30pm (local)",
		"Sat 10:00am (local)", "Mon 8:00am (local)", "Fri 1:00pm (local)",
	}
	return slots[index%len(slots)]
}

// planIntelligence aggregates collection-level production metadata.
func (g *Generator) planIntelligence(pkg ReleasePackage, shorts []Short) Intelligence {
	total := 0
	var topics, keywords []string
	for _, s := range shorts {
		total += s.DurationSec
		topics = append(topics, s.Angle)
	}
	if c := pkg.Context; c != nil {
		keywords = append(keywords, c.ContentIntelligence.SEOKeywords...)
		keywords = append(keywords, c.Architecture.AWSServices...)
	}
	keywords = append(keywords, pkg.Blog.Tags...)

	avg := 0
	if len(shorts) > 0 {
		avg = total / len(shorts)
	}
	return Intelligence{
		ShortCount:         len(shorts),
		TotalDurationSec:   total,
		AverageDurationSec: avg,
		Topics:             dedupe(topics),
		Audience:           audience(pkg),
		Difficulty:         difficulty(pkg),
		SEOKeywords:        topStrings(dedupe(keywords), 15),
		PublishCadence:     "Publish 1 Short per weekday, staggered to peak developer-viewing windows.",
		ProductionNotes: []string{
			"Vertical 9:16, burned-in captions, fast cuts.",
			"Each Short ends on the repository so viewers can find the full project.",
		},
	}
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
