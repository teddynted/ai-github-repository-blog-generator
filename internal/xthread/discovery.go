package xthread

// maxThreads bounds how many threads one release yields by default.
const maxThreads = 5

// threadCandidate is a discovered thread targeting a specific audience.
type threadCandidate struct {
	Type     string
	Audience string
	Seed     string // grounded fact
}

// discover selects the threads a release warrants, each targeting a different
// audience/objective, gated on grounded data. A diverse set is chosen (≤ max).
func discover(pkg ReleasePackage, max int) []threadCandidate {
	if max <= 0 {
		max = maxThreads
	}
	var cands []threadCandidate
	add := func(c threadCandidate) { cands = append(cands, c) }

	add(threadCandidate{Type: "Release Announcement", Audience: "Developers and technical communities", Seed: summary(pkg)})

	if fn := featureName(pkg); fn != "" && fn != "this release" {
		add(threadCandidate{Type: "Feature Breakdown", Audience: "Developers evaluating the project", Seed: fn})
	}
	if len(awsServices(pkg)) > 0 || archOverview(pkg) != "" {
		add(threadCandidate{Type: "Architecture Walkthrough", Audience: "Cloud engineers and software architects", Seed: firstNonEmpty(archOverview(pkg), summary(pkg))})
	}
	if len(awsServices(pkg)) > 0 {
		add(threadCandidate{Type: "AWS Best Practices", Audience: "AWS and DevOps engineers", Seed: awsSeed(pkg)})
	}
	if hasAI(pkg) {
		add(threadCandidate{Type: "AI Engineering Insights", Audience: "AI and ML engineers", Seed: aiSeed(pkg)})
	}
	if l := lessonSeed(pkg); l != "" {
		add(threadCandidate{Type: "Engineering Lessons Learned", Audience: "Engineers and technical leads", Seed: l})
	}
	if hasCode(pkg) {
		add(threadCandidate{Type: "Implementation Deep Dive", Audience: "Developers who want the details", Seed: featureName(pkg)})
	}
	if p := perfSeed(pkg); p != "" {
		add(threadCandidate{Type: "Performance Improvements", Audience: "Platform and SRE engineers", Seed: p})
	}
	add(threadCandidate{Type: "Developer Tips", Audience: "Working developers", Seed: featureName(pkg)})
	add(threadCandidate{Type: "Open Source Update", Audience: "The open-source community", Seed: summary(pkg)})

	return selectDiverse(cands, max)
}

func selectDiverse(cands []threadCandidate, max int) []threadCandidate {
	var out []threadCandidate
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

func awsSeed(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil && len(c.ContentIntelligence.ArchitectureHighlights) > 0 {
		return c.ContentIntelligence.ArchitectureHighlights[0]
	}
	if len(awsServices(pkg)) > 0 {
		return "Wiring up " + joinAnd(topStrings(awsServices(pkg), 3)) + " cleanly."
	}
	return ""
}

func aiSeed(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil && len(c.ContentIntelligence.TechnicalHighlights) > 0 {
		return c.ContentIntelligence.TechnicalHighlights[0]
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
