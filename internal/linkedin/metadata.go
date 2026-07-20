package linkedin

// planPostMeta builds per-post metadata, including deterministic engagement and
// professional-confidence scores.
func planPostMeta(pkg ReleasePackage, c postCandidate, post Post) PostMeta {
	m := PostMeta{
		Topic:                firstSentences(c.Seed, 1),
		Difficulty:           difficulty(pkg),
		Tone:                 "professional, educational, authentic",
		TechnologyStack:      technologies(pkg),
		AWSServices:          awsServices(pkg),
		ProgrammingLanguages: programmingLanguages(pkg),
		EstimatedReadingTime: readingTime(post.Body),
		SEOKeywords:          topStrings(pkg.SEO.Keywords.Primary, 6),
		SuggestedPublishTime: publishSlot(post.ID),
		CharacterCount:       len(post.Body),
	}
	m.EngagementScore = engagementScore(c, post)
	m.ProfessionalConfidence = professionalConfidence(pkg, post)
	return m
}

func readingTime(body string) string {
	w := wordCount(body)
	if w < 120 {
		return "under 1 min"
	}
	return itoa((w+199)/200) + " min"
}

// engagementScore is a 0–100 heuristic favouring discussion-oriented types and
// complete posts.
func engagementScore(c postCandidate, post Post) int {
	score := 55
	switch c.Type {
	case "Architecture Deep Dive", "Engineering Lesson", "AI Engineering Highlight":
		score += 20 // these spark discussion
	case "AWS Best Practice", "Behind-the-Build":
		score += 12
	default:
		score += 6
	}
	if post.EngagementPrompt != "" {
		score += 8
	}
	if len(post.TechnicalHighlights) >= 3 {
		score += 8
	}
	if len(post.VisualReferences) > 0 {
		score += 5
	}
	return clamp(score, 0, 100)
}

// professionalConfidence rewards a grounded, complete, well-formed post.
func professionalConfidence(pkg ReleasePackage, post Post) int {
	score := 50
	if post.CTA != "" {
		score += 12
	}
	if len(post.Hashtags) >= 3 {
		score += 10
	}
	if len(post.TechnicalHighlights) >= 2 {
		score += 12
	}
	if len(awsServices(pkg)) > 0 || len(technologies(pkg)) > 0 {
		score += 8
	}
	if wordCount(post.Body) >= 60 {
		score += 8
	}
	return clamp(score, 0, 100)
}

func publishSlot(id int) string {
	slots := []string{
		"Tue 8:30am (local)", "Wed 12:00pm (local)", "Thu 9:00am (local)",
		"Tue 5:00pm (local)", "Wed 8:00am (local)", "Thu 1:00pm (local)",
	}
	return slots[(id-1+len(slots))%len(slots)]
}

// planIntelligence aggregates collection-level metadata.
func (g *Generator) planIntelligence(pkg ReleasePackage, posts []Post) Intelligence {
	var audiences, topics []string
	engSum, confSum := 0, 0
	for _, p := range posts {
		audiences = append(audiences, p.Audience)
		topics = append(topics, p.Type)
		engSum += p.Metadata.EngagementScore
		confSum += p.Metadata.ProfessionalConfidence
	}
	eng, conf := 0, 0
	if len(posts) > 0 {
		eng = engSum / len(posts)
		conf = confSum / len(posts)
	}
	return Intelligence{
		PostCount:                     len(posts),
		TargetAudiences:               dedupe(audiences),
		Topics:                        dedupe(topics),
		TechnologyStack:               technologies(pkg),
		AWSServices:                   awsServices(pkg),
		ProgrammingLanguages:          programmingLanguages(pkg),
		AverageEngagementScore:        eng,
		AverageProfessionalConfidence: conf,
		SEOKeywords:                   topStrings(pkg.SEO.Keywords.Primary, 8),
		Tone:                          "professional, educational, authentic",
		PublishCadence:                "Publish 1–2 posts per week, spaced across Tue–Thu peak windows; lead with the deep dives.",
		ProductionNotes: []string{
			"Posts reference existing visual assets and architecture diagrams — do not regenerate images.",
			"Keep the tone authentic and technical; avoid hype and performance claims not in the release.",
		},
	}
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

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
