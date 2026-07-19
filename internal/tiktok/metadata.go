package tiktok

import "strings"

// planSEO builds per-video publishing metadata (caption, alternative titles,
// posting slot). Posting slots stagger a batch across TikTok peak windows.
func planSEO(t topic, index int) SEO {
	return SEO{
		AlternativeTitles: alternativeTitles(t),
		Caption:           videoCaption(t),
		PostingTime:       postingSlot(index),
	}
}

func alternativeTitles(t topic) []string {
	switch t.Topic {
	case "Architecture Insight":
		return []string{"The architecture explained fast", "System design in 60 seconds"}
	case "Common Mistake":
		return []string{"Stop doing this", "The mistake to avoid"}
	case "Interesting Statistic":
		return []string{"By the numbers", "The numbers don't lie"}
	default:
		return []string{t.Title, "Quick " + strings.ToLower(t.Topic)}
	}
}

func videoCaption(t topic) string {
	lead := firstSentences(t.Seed, 1)
	if lead == "" {
		lead = t.Title + "."
	}
	return collapse(lead + " Full video + repo in bio.")
}

// postingSlot staggers a batch across TikTok peak windows for dev audiences.
func postingSlot(index int) string {
	slots := []string{
		"Tue 7:00pm (local)", "Wed 12:00pm (local)", "Thu 9:00pm (local)",
		"Fri 6:00am (local)", "Sat 11:00am (local)", "Mon 8:00pm (local)",
	}
	return slots[index%len(slots)]
}

// retentionScore is a 0–100 heuristic estimating how well a TikTok will hold
// viewers: strong opening topics, the ~25–40s sweet spot, dense captions, and
// visual variety all raise it. Deterministic — a planning signal, not a promise.
func retentionScore(v Video) int {
	score := 55

	switch v.Topic {
	case "Common Mistake", "Architecture Insight", "AWS Tip", "GitHub Automation":
		score += 15 // strong scroll-stoppers
	case "Interesting Statistic", "CloudFormation Trick", "Developer Productivity":
		score += 10
	default:
		score += 5
	}

	switch {
	case v.DurationSec >= 25 && v.DurationSec <= 40:
		score += 15 // the retention sweet spot
	case v.DurationSec <= 50:
		score += 8
	}

	if len(v.Captions) >= 6 {
		score += 8 // dense captions hold attention
	}
	if len(v.Visuals) >= 3 {
		score += 7 // visual variety
	}
	if collapse(v.EngagementPrompt) != "" {
		score += 3
	}
	return clampInt(score, 0, 100)
}

// planIntelligence aggregates collection-level production metadata.
func (g *Generator) planIntelligence(pkg ReleasePackage, videos []Video) Intelligence {
	total, retSum := 0, 0
	var topics, keywords []string
	for _, v := range videos {
		total += v.DurationSec
		retSum += v.RetentionScore
		topics = append(topics, v.Topic)
	}
	if c := pkg.Context; c != nil {
		keywords = append(keywords, c.ContentIntelligence.SEOKeywords...)
		keywords = append(keywords, c.Architecture.AWSServices...)
	}
	keywords = append(keywords, pkg.Blog.Tags...)

	avg, avgRet := 0, 0
	if len(videos) > 0 {
		avg = total / len(videos)
		avgRet = retSum / len(videos)
	}
	return Intelligence{
		VideoCount:         len(videos),
		TotalDurationSec:   total,
		AverageDurationSec: avg,
		AverageRetention:   avgRet,
		Topics:             dedupe(topics),
		Audience:           audience(pkg),
		Difficulty:         difficulty(pkg),
		SEOKeywords:        topStrings(dedupe(keywords), 15),
		PostingCadence:     "Post 1 TikTok per day, staggered to evening/lunch dev-viewing windows.",
		ProductionNotes: []string{
			"Vertical 9:16, burned-in captions, hook in the first 2 seconds.",
			"Keep cuts fast; land the engagement prompt before the CTA.",
			"Every video ends on the repository so viewers can find the project.",
		},
	}
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
