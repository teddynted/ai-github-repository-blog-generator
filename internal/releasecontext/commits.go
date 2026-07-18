package releasecontext

import (
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
)

// categoryForType maps a Conventional Commit type to a human release category.
var categoryForType = map[string]string{
	"feat":     "Features",
	"fix":      "Bug Fixes",
	"docs":     "Documentation",
	"refactor": "Refactoring",
	"perf":     "Performance",
	"test":     "Testing",
	"build":    "Build",
	"ci":       "CI/CD",
	"chore":    "Chores",
	"revert":   "Reverts",
}

// infraScopes flags scopes that mean the change is infrastructure work,
// promoting it to the Infrastructure category regardless of type.
var infraScopes = map[string]bool{
	"infra": true, "infrastructure": true, "cfn": true, "cloudformation": true,
	"deploy": true, "iam": true, "network": true, "scheduler": true, "compute": true,
}

// analyzeCommits categorizes the commit range, dropping merge, version-bump and
// formatting-only commits, and returns per-category statistics.
func analyzeCommits(raw []RawCommit) ([]Commit, CommitStats) {
	stats := CommitStats{Total: len(raw), ByCategory: map[string]int{}}
	out := make([]Commit, 0, len(raw))
	seen := map[string]bool{}
	var contributors []string

	for _, rc := range raw {
		if rc.Parents > 1 || isBump(rc.Subject) || isFormatOnly(rc.Subject) {
			stats.Ignored++
			continue
		}
		c := Commit{SHA: rc.SHA, Subject: rc.Subject, Author: rc.Author, Date: rc.Date}
		if parsed, err := conventional.Parse(rc.Subject, conventional.Config{}); err == nil {
			c.Conventional = true
			c.Type = parsed.Type
			c.Scope = parsed.Scope
			c.Breaking = parsed.Breaking
			c.Category = categorize(parsed.Type, parsed.Scope)
		} else {
			c.Type = "other"
			c.Category = heuristicCategory(rc.Subject)
		}
		out = append(out, c)
		stats.Analyzed++
		if c.Conventional {
			stats.Conventional++
		}
		if c.Breaking {
			stats.Breaking++
		}
		stats.ByCategory[c.Category]++
		if rc.Author != "" && !seen[rc.Author] {
			seen[rc.Author] = true
			contributors = append(contributors, rc.Author)
		}
	}
	sort.Strings(contributors)
	stats.Contributors = contributors
	return out, stats
}

// categorize resolves the release category, letting an infrastructure scope win.
func categorize(typ, scope string) string {
	if infraScopes[strings.ToLower(scope)] {
		return "Infrastructure"
	}
	if cat, ok := categoryForType[typ]; ok {
		return cat
	}
	return "Other"
}

// heuristicCategory classifies a non-conventional subject by keyword.
func heuristicCategory(subject string) string {
	s := strings.ToLower(subject)
	switch {
	case containsAnyWord(s, "fix", "bug", "patch", "hotfix"):
		return "Bug Fixes"
	case containsAnyWord(s, "add", "feature", "implement", "introduce", "support"):
		return "Features"
	case containsAnyWord(s, "doc", "docs", "readme", "documentation"):
		return "Documentation"
	case containsAnyWord(s, "refactor", "cleanup", "simplify"):
		return "Refactoring"
	case containsAnyWord(s, "test", "tests", "coverage"):
		return "Testing"
	case containsAnyWord(s, "ci", "pipeline", "workflow", "actions"):
		return "CI/CD"
	case containsAnyWord(s, "cloudformation", "infra", "terraform", "deploy", "iam"):
		return "Infrastructure"
	default:
		return "Other"
	}
}

// isBump reports whether a subject is a version-bump / release commit.
func isBump(subject string) bool {
	s := strings.ToLower(strings.TrimSpace(subject))
	return strings.HasPrefix(s, "chore(release)") ||
		strings.HasPrefix(s, "release ") ||
		strings.HasPrefix(s, "bump version") ||
		strings.HasPrefix(s, "v") && looksLikeVersionOnly(s)
}

// isFormatOnly reports whether a subject is a formatting-only change.
func isFormatOnly(subject string) bool {
	s := strings.ToLower(strings.TrimSpace(subject))
	return strings.HasPrefix(s, "style:") || strings.HasPrefix(s, "style(") ||
		strings.Contains(s, "gofmt") || strings.Contains(s, "formatting only") ||
		strings.Contains(s, "run prettier")
}

// looksLikeVersionOnly matches subjects that are just a version like "v1.2.3".
func looksLikeVersionOnly(s string) bool {
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

func containsAnyWord(haystack string, words ...string) bool {
	fields := strings.FieldsFunc(haystack, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	set := map[string]bool{}
	for _, f := range fields {
		set[f] = true
	}
	for _, w := range words {
		if set[w] {
			return true
		}
	}
	return false
}
