package release

import (
	"fmt"
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
)

// Notes renders GitHub-Release-ready markdown notes for a version: a summary,
// grouped features / fixes / breaking changes, a link to the CHANGELOG section,
// and contributors (when available).
func Notes(version, date, changelogAnchor string, commits []conventional.Commit, contributors []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s (%s)\n\n", version, date)

	feat := descriptions(commits, func(c conventional.Commit) bool { return c.Type == "feat" && !c.Breaking })
	fixes := descriptions(commits, func(c conventional.Commit) bool { return c.Type == "fix" && !c.Breaking })
	breaking := descriptions(commits, func(c conventional.Commit) bool { return c.Breaking })

	fmt.Fprintf(&b, "%s\n\n", summary(len(feat), len(fixes), len(breaking)))

	writeSection(&b, "Breaking Changes", breaking)
	writeSection(&b, "Features", feat)
	writeSection(&b, "Bug Fixes", fixes)

	if len(contributors) > 0 {
		uniq := dedupeSorted(contributors)
		b.WriteString("### Contributors\n\n")
		for _, c := range uniq {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteString("\n")
	}

	if changelogAnchor != "" {
		fmt.Fprintf(&b, "See the [CHANGELOG](%s) for the full list of changes.\n", changelogAnchor)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func summary(nFeat, nFix, nBreak int) string {
	if nFeat == 0 && nFix == 0 && nBreak == 0 {
		return "Maintenance release."
	}
	var parts []string
	if nBreak > 0 {
		parts = append(parts, plural(nBreak, "breaking change", "breaking changes"))
	}
	if nFeat > 0 {
		parts = append(parts, plural(nFeat, "new feature", "new features"))
	}
	if nFix > 0 {
		parts = append(parts, plural(nFix, "bug fix", "bug fixes"))
	}
	return "This release includes " + joinList(parts) + "."
}

func writeSection(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "### %s\n\n", title)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
	b.WriteString("\n")
}

func descriptions(commits []conventional.Commit, keep func(conventional.Commit) bool) []string {
	var out []string
	for _, c := range commits {
		if !keep(c) {
			continue
		}
		if c.Scope != "" {
			out = append(out, fmt.Sprintf("**%s:** %s", c.Scope, c.Description))
		} else {
			out = append(out, c.Description)
		}
	}
	sort.Strings(out)
	return out
}

func dedupeSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func joinList(parts []string) string {
	switch len(parts) {
	case 0:
		return "changes"
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
	}
}
