package xthread

// planThreadMeta builds per-thread metadata, including engagement and technical-
// confidence scores.
func planThreadMeta(pkg ReleasePackage, c threadCandidate, thread Thread) ThreadMeta {
	total := 0
	for _, p := range thread.Posts {
		total += p.CharacterCount
	}
	m := ThreadMeta{
		Topic:                firstSentences(c.Seed, 1),
		Difficulty:           difficulty(pkg),
		Tone:                 "technical, concise, educational",
		TechnologyStack:      technologies(pkg),
		AWSServices:          awsServices(pkg),
		ProgrammingLanguages: programmingLanguages(pkg),
		EstimatedReadingTime: readingTime(len(thread.Posts)),
		SEOKeywords:          topStrings(pkg.SEO.Keywords.Primary, 6),
		SuggestedPostingTime: postingSlot(thread.ID),
		TotalCharacters:      total,
	}
	m.EngagementScore = engagementScore(c, thread)
	m.TechnicalConfidence = technicalConfidence(pkg, thread)
	return m
}

func readingTime(posts int) string {
	// ~12 seconds per post to skim.
	sec := posts * 12
	if sec < 30 {
		return "under 30s"
	}
	return itoa(sec/60) + "m " + itoa(sec%60) + "s"
}

func engagementScore(c threadCandidate, thread Thread) int {
	score := 55
	switch c.Type {
	case "Architecture Walkthrough", "Engineering Lessons Learned", "AI Engineering Insights":
		score += 20
	case "AWS Best Practices", "Implementation Deep Dive":
		score += 12
	default:
		score += 6
	}
	if thread.EngagementPrompt != "" {
		score += 8
	}
	if len(thread.KeyTakeaways) >= 3 {
		score += 8
	}
	if len(thread.Posts) >= 5 {
		score += 5
	}
	return clamp(score, 0, 100)
}

func technicalConfidence(pkg ReleasePackage, thread Thread) int {
	score := 50
	if thread.CTA != "" {
		score += 10
	}
	if len(thread.Hashtags) >= 2 {
		score += 8
	}
	if len(thread.KeyTakeaways) >= 2 {
		score += 12
	}
	if len(awsServices(pkg)) > 0 || len(technologies(pkg)) > 0 {
		score += 10
	}
	if hasCode(pkg) {
		score += 10
	}
	return clamp(score, 0, 100)
}

func postingSlot(id int) string {
	slots := []string{
		"Tue 9:00am (local)", "Wed 12:30pm (local)", "Thu 8:00am (local)",
		"Tue 4:00pm (local)", "Wed 9:00am (local)", "Fri 11:00am (local)",
	}
	return slots[(id-1+len(slots))%len(slots)]
}

// planIntelligence aggregates collection-level metadata.
func (g *Generator) planIntelligence(pkg ReleasePackage, threads []Thread) Intelligence {
	var audiences, topics []string
	engSum, confSum := 0, 0
	for _, th := range threads {
		audiences = append(audiences, th.Audience)
		topics = append(topics, th.Type)
		engSum += th.Metadata.EngagementScore
		confSum += th.Metadata.TechnicalConfidence
	}
	eng, conf := 0, 0
	if len(threads) > 0 {
		eng = engSum / len(threads)
		conf = confSum / len(threads)
	}
	return Intelligence{
		ThreadCount:                len(threads),
		TargetAudiences:            dedupe(audiences),
		Topics:                     dedupe(topics),
		TechnologyStack:            technologies(pkg),
		AWSServices:                awsServices(pkg),
		ProgrammingLanguages:       programmingLanguages(pkg),
		AverageEngagementScore:     eng,
		AverageTechnicalConfidence: conf,
		SEOKeywords:                topStrings(pkg.SEO.Keywords.Primary, 8),
		Tone:                       "technical, concise, educational",
		PostingCadence:             "Post 1 thread per weekday, spaced across morning/lunch windows; lead with the deep dives.",
		ProductionNotes: []string{
			"Every post respects the 280-character limit; code snippets are grounded in the blog.",
			"Threads reference existing visual assets and architecture diagrams — do not regenerate images.",
			"Keep claims grounded in the release; no invented metrics or performance numbers.",
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
