package releasecontext

import "strings"

// changelogHeadingField maps a Keep-a-Changelog "### Section" title to a field
// in ChangelogAnalysis.
func changelogHeadingField(title string) string {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "added", "features", "new features":
		return "features"
	case "changed", "improvements", "improved", "enhancements":
		return "improvements"
	case "fixed", "bug fixes", "fixes":
		return "bugfixes"
	case "breaking changes", "breaking":
		return "breaking"
	case "deprecated", "deprecations":
		return "deprecations"
	case "migration", "migration notes", "upgrade notes":
		return "migration"
	default:
		return ""
	}
}

// analyzeChangelog extracts the CHANGELOG section for version (with or without a
// leading "v" or brackets) and buckets its bullet points.
func analyzeChangelog(content, version string) ChangelogAnalysis {
	var a ChangelogAnalysis
	if content == "" || version == "" {
		return a
	}
	want := normalizeVersion(version)
	lines := strings.Split(content, "\n")

	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "## ") && lineHasVersion(ln, want) {
			start = i
			break
		}
	}
	if start < 0 {
		return a
	}
	a.Found = true
	a.Version = want
	a.Date = extractDate(lines[start])

	field := ""
	for _, ln := range lines[start+1:] {
		if strings.HasPrefix(ln, "## ") { // next release section
			break
		}
		if strings.HasPrefix(ln, "### ") {
			field = changelogHeadingField(strings.TrimPrefix(ln, "### "))
			continue
		}
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, "- ") && !strings.HasPrefix(t, "* ") {
			continue
		}
		item := strings.TrimSpace(t[2:])
		if item == "" {
			continue
		}
		switch field {
		case "features":
			a.Features = append(a.Features, item)
		case "improvements":
			a.Improvements = append(a.Improvements, item)
		case "bugfixes":
			a.BugFixes = append(a.BugFixes, item)
		case "breaking":
			a.BreakingChanges = append(a.BreakingChanges, item)
		case "deprecations":
			a.Deprecations = append(a.Deprecations, item)
		case "migration":
			a.MigrationNotes = append(a.MigrationNotes, item)
		}
	}
	return a
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}

// lineHasVersion reports whether a "## ..." heading refers to version want.
func lineHasVersion(line, want string) bool {
	// Matches "## [1.2.3] - date", "## 1.2.3", "## v1.2.3".
	rest := strings.TrimPrefix(line, "## ")
	rest = strings.ReplaceAll(rest, "[", " ")
	rest = strings.ReplaceAll(rest, "]", " ")
	for _, tok := range strings.Fields(rest) {
		if normalizeVersion(tok) == want {
			return true
		}
	}
	return false
}

func extractDate(heading string) string {
	// "## [1.2.3] - 2026-07-18"
	if i := strings.LastIndex(heading, "- "); i >= 0 {
		d := strings.TrimSpace(heading[i+2:])
		if len(d) == 10 && d[4] == '-' && d[7] == '-' {
			return d
		}
	}
	return ""
}
