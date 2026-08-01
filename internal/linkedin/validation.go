package linkedin

import (
	"fmt"
	"strings"
)

// Validate checks the collection's invariants and returns a list of problems
// (empty when valid). It enforces the Milestone 12 rules: every post is grounded,
// non-empty, and carries a CTA, hashtags, and an engagement prompt; technical
// claims are grounded in the Release Context; and no two posts are duplicates.
//
// pkg is the source package, used to confirm grounding.
func (col LinkedInCollection) Validate(pkg ReleasePackage) []string {
	var problems []string

	if col.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(col.Posts) == 0 {
		problems = append(problems, "collection has no posts")
	}
	if col.Metadata.PostCount != len(col.Posts) {
		problems = append(problems, fmt.Sprintf("metadata.postCount %d != posts %d", col.Metadata.PostCount, len(col.Posts)))
	}

	grounded := groundedTerms(pkg)
	seenType := map[string]bool{}
	seenBody := map[string]bool{}

	for i, p := range col.Posts {
		where := fmt.Sprintf("post %d (%s)", i+1, p.Type)

		if collapse(p.Body) == "" {
			problems = append(problems, where+": empty body")
		}
		if collapse(p.CTA) == "" {
			problems = append(problems, where+": missing CTA")
		}
		if len(p.Hashtags) == 0 {
			problems = append(problems, where+": no hashtags")
		}
		if collapse(p.EngagementPrompt) == "" {
			problems = append(problems, where+": missing engagement prompt")
		}

		// Grounding: the post must reference at least one grounded term.
		if !bodyGrounded(p, grounded) {
			problems = append(problems, where+": not grounded in the release context")
		}

		// Neutrality: body PROSE must be evergreen and focus on what the release DOES
		// — no repository name or release version in the copy. A link URL (the CTA)
		// is attribution, not body copy, so it is excluded from the check.
		if prose := proseOnly(p.Body); namesReleaseIdentity(prose, pkg) {
			if r := repoShort(pkg); r != "" && strings.Contains(strings.ToLower(prose), strings.ToLower(r)) {
				problems = append(problems, where+": body names the repository (posts must be evergreen)")
			}
			if t := tag(pkg); t != "" && strings.Contains(prose, t) {
				problems = append(problems, where+": body names the release version (posts must be evergreen)")
			}
		}

		// Technical claims: every highlight must be grounded.
		for _, h := range p.TechnicalHighlights {
			if !termGrounded(h, grounded) {
				problems = append(problems, fmt.Sprintf("%s: ungrounded technical claim %q", where, truncateWords(h, 8)))
			}
		}

		// Duplicate detection by type and by body text.
		if seenType[p.Type] {
			problems = append(problems, where+": duplicate post type")
		}
		seenType[p.Type] = true
		bkey := slug(p.Body)
		if bkey != "" && seenBody[bkey] {
			problems = append(problems, where+": duplicate post body")
		}
		seenBody[bkey] = true
	}

	return problems
}

// bodyGrounded reports whether a post references any grounded term (in its body,
// title, or highlights).
func bodyGrounded(p Post, grounded map[string]bool) bool {
	hay := strings.ToLower(p.Title + " " + p.Body + " " + strings.Join(p.TechnicalHighlights, " "))
	for term := range grounded {
		if term != "" && strings.Contains(hay, term) {
			return true
		}
	}
	return false
}

// termGrounded reports whether a term (or a word within it) is grounded.
func termGrounded(term string, grounded map[string]bool) bool {
	lc := strings.ToLower(collapse(term))
	if grounded[lc] {
		return true
	}
	for g := range grounded {
		if g != "" && (strings.Contains(lc, g) || strings.Contains(g, lc)) {
			return true
		}
	}
	return false
}
