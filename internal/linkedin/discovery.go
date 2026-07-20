package linkedin

// maxPosts bounds how many LinkedIn posts one release yields by default.
const maxPosts = 6

// postCandidate is a discovered post targeting a specific professional audience.
type postCandidate struct {
	Type      string
	Variation string
	Audience  string
	Seed      string // grounded fact the post is built on
}

// discover selects the LinkedIn posts a release warrants, each targeting a
// different professional audience. Types are gated on grounded data so nothing
// is invented, then a diverse set is chosen (≤ max).
func discover(pkg ReleasePackage, max int) []postCandidate {
	if max <= 0 {
		max = maxPosts
	}
	var cands []postCandidate
	add := func(c postCandidate) { cands = append(cands, c) }

	// Release Announcement — always.
	add(postCandidate{Type: "Release Announcement", Variation: "Long-form Post",
		Audience: "Software engineers and engineering managers", Seed: summary(pkg)})

	// Feature Spotlight — when there is a headline feature.
	if fn := featureName(pkg); fn != "" && fn != "this release" {
		add(postCandidate{Type: "Feature Spotlight", Variation: "Medium Post",
			Audience: "Developers evaluating the project", Seed: fn})
	}

	// Architecture Deep Dive — when there is architecture / AWS.
	if len(awsServices(pkg)) > 0 || archOverview(pkg) != "" {
		add(postCandidate{Type: "Architecture Deep Dive", Variation: "Long-form Post",
			Audience: "Cloud and solutions architects", Seed: firstNonEmpty(archOverview(pkg), summary(pkg))})
	}

	// AWS Best Practice — when AWS services are used.
	if len(awsServices(pkg)) > 0 {
		add(postCandidate{Type: "AWS Best Practice", Variation: "Medium Post",
			Audience: "AWS and cloud engineers", Seed: awsPracticeSeed(pkg)})
	}

	// AI Engineering Highlight — when the stack has AI.
	if hasAI(pkg) {
		add(postCandidate{Type: "AI Engineering Highlight", Variation: "Medium Post",
			Audience: "AI and ML engineers", Seed: aiSeed(pkg)})
	}

	// Engineering Lesson — when there's a "why it matters" / lesson.
	if l := lessonSeed(pkg); l != "" {
		add(postCandidate{Type: "Engineering Lesson", Variation: "Medium Post",
			Audience: "Engineers and technical leads", Seed: l})
	}

	// Developer Productivity Tip — short, always useful.
	add(postCandidate{Type: "Developer Productivity Tip", Variation: "Short Update",
		Audience: "Working developers", Seed: featureName(pkg)})

	// Behind-the-Build — the story of how it was built.
	add(postCandidate{Type: "Behind-the-Build", Variation: "Long-form Post",
		Audience: "Developer advocates and the open-source community", Seed: summary(pkg)})

	// Performance Improvement — only when there is an infra/perf signal.
	if p := perfSeed(pkg); p != "" {
		add(postCandidate{Type: "Performance Improvement", Variation: "Medium Post",
			Audience: "Platform and SRE engineers", Seed: p})
	}

	return selectDiverse(cands, max)
}

func selectDiverse(cands []postCandidate, max int) []postCandidate {
	var out []postCandidate
	seen := map[string]bool{}
	for _, c := range cands {
		if len(out) >= max {
			break
		}
		if seen[c.Type] {
			continue
		}
		seen[c.Type] = true
		out = append(out, c)
	}
	return out
}

func archOverview(pkg ReleasePackage) string {
	if pkg.Context != nil {
		return firstSentences(pkg.Context.Architecture.Overview, 1)
	}
	return ""
}

func awsPracticeSeed(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if len(c.ContentIntelligence.ArchitectureHighlights) > 0 {
			return c.ContentIntelligence.ArchitectureHighlights[0]
		}
		if len(c.Architecture.AWSServices) > 0 {
			return "Wiring up " + joinAnd(topStrings(awsServices(pkg), 3)) + " cleanly."
		}
	}
	return ""
}

func aiSeed(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		for _, h := range c.ContentIntelligence.TechnicalHighlights {
			return h
		}
	}
	return "AI-assisted content generation grounded in real repository analysis."
}

func lessonSeed(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if c.Implementation.WhyItMatters != "" {
			return firstSentences(c.Implementation.WhyItMatters, 1)
		}
		if c.ContentIntelligence.DeveloperValue != "" {
			return firstSentences(c.ContentIntelligence.DeveloperValue, 1)
		}
	}
	return ""
}

func perfSeed(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if len(c.ContentIntelligence.InfrastructureHighlights) > 0 {
			return c.ContentIntelligence.InfrastructureHighlights[0]
		}
		for _, imp := range c.Implementation.InfrastructureImprovements {
			return imp
		}
	}
	return ""
}
