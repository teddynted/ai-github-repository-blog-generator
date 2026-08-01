package youtube

import (
	"fmt"
	"strings"
)

// planCTA builds the closing call to action: concrete, relevant, and concise.
// Links are grounded in the Release Context (repo URL, homepage) — never
// fabricated.
func (g *Generator) planCTA(pkg ReleasePackage) CallToAction {
	repo := repoName(pkg)
	var items []CTAItem

	if url := repoURL(pkg); url != "" {
		items = append(items, CTAItem{Kind: "GitHub Repository", Text: "Star and explore " + repo, URL: url})
	} else {
		items = append(items, CTAItem{Kind: "GitHub Repository", Text: "Star and explore " + repo})
	}
	if docs := docsURL(pkg); docs != "" {
		items = append(items, CTAItem{Kind: "Documentation", Text: "Read the docs to go deeper", URL: docs})
	}
	items = append(items,
		CTAItem{Kind: "Subscribe", Text: "Subscribe for the next deep dive"},
		CTAItem{Kind: "Like", Text: "Like the video if the walkthrough helped"},
		CTAItem{Kind: "Comment", Text: "Comment with how you'd build this differently"},
		CTAItem{Kind: "Future Releases", Text: "Follow along as the pipeline grows release by release"},
		CTAItem{Kind: "Contribute", Text: "Open-source contributions are welcome — issues and PRs both"},
	)

	script := ctaScript(repo)
	return CallToAction{Script: script, Items: items, PinnedComment: pinnedComment(pkg)}
}

func ctaScript(repo string) string {
	return collapse(fmt.Sprintf(
		"If you got something out of this, do three quick things: star %s so you can find it again, "+
			"subscribe so you catch the next deep dive, and drop a comment with how you'd approach it "+
			"differently — I read them. Links to the repo and the docs are in the description.", repo))
}

// pinnedComment is a short, useful pinned comment grounded in the release.
func pinnedComment(pkg ReleasePackage) string {
	var b strings.Builder
	b.WriteString("📌 Everything in this video is generated from the project's own release analysis.")
	if url := repoURL(pkg); url != "" {
		fmt.Fprintf(&b, " Repo: %s", url)
	}
	b.WriteString(" What would you like the next deep dive to cover?")
	return collapse(b.String())
}

func repoURL(pkg ReleasePackage) string {
	if pkg.Context != nil {
		if pkg.Context.Repository.URL != "" {
			return pkg.Context.Repository.URL
		}
		if pkg.Context.Repository.FullName != "" {
			return "https://github.com/" + pkg.Context.Repository.FullName
		}
	}
	return ""
}

func docsURL(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.Homepage != "" {
		return pkg.Context.Repository.Homepage
	}
	return ""
}
