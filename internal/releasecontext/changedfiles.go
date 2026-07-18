package releasecontext

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Change categories, in priority order (first match wins).
const (
	catInfrastructure = "Infrastructure"
	catCICD           = "CI/CD"
	catDiagrams       = "Diagrams"
	catDocumentation  = "Documentation"
	catTests          = "Tests"
	catConfiguration  = "Configuration"
	catAppCode        = "Application Code"
	catOther          = "Other"
)

// categorizeFile classifies a repository-relative path into a change category.
// Order matters: more specific rules are checked first.
func categorizeFile(p string) string {
	lp := strings.ToLower(p)
	base := strings.ToLower(path.Base(p))
	ext := strings.ToLower(path.Ext(p))

	switch {
	case strings.HasPrefix(lp, ".github/workflows/") || strings.Contains(lp, "/hooks/") || strings.HasPrefix(lp, "scripts/hooks/"):
		return catCICD
	case ext == ".mmd" || strings.Contains(lp, "/diagrams/") || strings.HasPrefix(lp, "diagrams/"):
		return catDiagrams
	case strings.HasPrefix(lp, "infrastructure/") || strings.HasPrefix(lp, "packer/") ||
		strings.HasPrefix(lp, "instance/") || ext == ".tf" || isCFNTemplatePath(lp):
		return catInfrastructure
	case strings.HasSuffix(lp, "_test.go") || strings.HasPrefix(lp, "test/") ||
		strings.Contains(lp, "/testdata/") || strings.Contains(lp, "/__tests__/"):
		return catTests
	case ext == ".md" || strings.HasPrefix(lp, "docs/") || base == "readme" || base == "changelog.md":
		return catDocumentation
	case ext == ".go" || ext == ".py" || ext == ".js" || ext == ".ts" ||
		strings.HasPrefix(lp, "cmd/") || strings.HasPrefix(lp, "internal/") ||
		strings.HasPrefix(lp, "pkg/") || strings.HasPrefix(lp, "lambdas/") || strings.HasPrefix(lp, "src/"):
		return catAppCode
	case isConfigFile(base, ext):
		return catConfiguration
	default:
		return catOther
	}
}

func isConfigFile(base, ext string) bool {
	switch base {
	case "makefile", "dockerfile", "docker-compose.yml", "docker-compose.yaml",
		"go.mod", "go.sum", ".release.json", ".gitleaks.toml", ".gitignore",
		"package.json", "tsconfig.json", ".env.example":
		return true
	}
	switch ext {
	case ".json", ".toml", ".ini", ".cfg", ".env":
		return true
	case ".yaml", ".yml":
		return true // non-infra YAML (config); infra YAML matched earlier
	}
	return false
}

// analyzeChangedFiles categorizes the changed files and rolls up statistics.
func analyzeChangedFiles(raw []RawChangedFile) ([]ChangedFile, FileStats) {
	out := make([]ChangedFile, 0, len(raw))
	stats := FileStats{Total: len(raw), ByCategory: map[string]int{}}
	for _, rc := range raw {
		cat := categorizeFile(rc.Path)
		out = append(out, ChangedFile{
			Path: rc.Path, Category: cat, Status: rc.Status,
			Additions: rc.Additions, Deletions: rc.Deletions,
		})
		stats.ByCategory[cat]++
		stats.Additions += rc.Additions
		stats.Deletions += rc.Deletions
	}
	stats.Summary = summarizeFileStats(stats)
	return out, stats
}

func summarizeFileStats(s FileStats) string {
	if s.Total == 0 {
		return "No files changed for this release."
	}
	// Order categories by count desc, then name, for a stable summary.
	type kv struct {
		k string
		v int
	}
	var pairs []kv
	for k, v := range s.ByCategory {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, fmt.Sprintf("%d %s", p.v, p.k))
	}
	return fmt.Sprintf("%d files changed (+%d/-%d): %s.",
		s.Total, s.Additions, s.Deletions, strings.Join(parts, ", "))
}
