// Package changelog renders and maintains a Keep-a-Changelog-style CHANGELOG.md
// from Conventional Commits, grouping entries by category under each release
// version. Rendering is deterministic and idempotent: re-adding a version that
// already exists is a no-op.
package changelog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
)

// Header is the file preamble kept above the version sections.
const Header = "# Changelog\n\nAll notable changes to this project are documented here.\nThis project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)\nand [Conventional Commits](https://www.conventionalcommits.org/).\n"

// Category maps commit types to a changelog heading, in display order.
type Category struct {
	Title string   `json:"title"`
	Types []string `json:"types"`
}

// DefaultCategories is the default grouping. Breaking changes are handled
// separately (any breaking commit is listed under "Breaking Changes" too).
var DefaultCategories = []Category{
	{"Features", []string{"feat"}},
	{"Bug Fixes", []string{"fix"}},
	{"Performance", []string{"perf"}},
	{"Refactoring", []string{"refactor"}},
	{"Documentation", []string{"docs"}},
}

// Render returns the markdown section for a single release version. date is
// ISO-8601 (YYYY-MM-DD). It emits only non-empty categories, plus a leading
// "Breaking Changes" section when any commit is breaking.
func Render(version, date string, commits []conventional.Commit, cats []Category) string {
	if len(cats) == 0 {
		cats = DefaultCategories
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## [%s] - %s\n", version, date)

	if breaking := breakingLines(commits); len(breaking) > 0 {
		b.WriteString("\n### Breaking Changes\n\n")
		for _, l := range breaking {
			b.WriteString(l)
		}
	}

	for _, cat := range cats {
		lines := linesFor(commits, cat.Types)
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n\n", cat.Title)
		for _, l := range lines {
			b.WriteString(l)
		}
	}
	b.WriteString("\n")
	return b.String()
}

// Update inserts a rendered section for version into existing CHANGELOG content
// (creating the file body if empty), immediately below the header and above the
// most recent version. If version is already present, existing is returned
// unchanged (idempotent, no duplicate entries).
func Update(existing, version, section string) string {
	if existing == "" {
		return Header + "\n" + section
	}
	if strings.Contains(existing, fmt.Sprintf("## [%s]", version)) {
		return existing // already documented — do not duplicate
	}

	// Insert the new section just before the first existing "## [" version
	// heading, or append after the header if there are none yet.
	marker := "\n## ["
	if i := strings.Index(existing, marker); i >= 0 {
		return existing[:i+1] + section + "\n" + existing[i+1:]
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + section
}

// Contains reports whether the changelog already documents version.
func Contains(existing, version string) bool {
	return strings.Contains(existing, fmt.Sprintf("## [%s]", version))
}

func linesFor(commits []conventional.Commit, types []string) []string {
	var out []string
	for _, c := range commits {
		if contains(types, c.Type) {
			out = append(out, "- "+entry(c)+"\n")
		}
	}
	sort.Strings(out)
	return out
}

func breakingLines(commits []conventional.Commit) []string {
	var out []string
	for _, c := range commits {
		if c.Breaking {
			out = append(out, "- "+entry(c)+"\n")
		}
	}
	sort.Strings(out)
	return out
}

// entry formats a single commit line: "**scope:** description" (scope optional).
func entry(c conventional.Commit) string {
	if c.Scope != "" {
		return fmt.Sprintf("**%s:** %s", c.Scope, c.Description)
	}
	return c.Description
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
