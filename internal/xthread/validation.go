package xthread

import (
	"fmt"
	"strings"
)

// Validate checks the collection's invariants and returns a list of problems
// (empty when valid). It enforces the Milestone 13 rules: every thread is
// grounded, has a CTA, hashtags, an engagement prompt, and at least one key
// takeaway; posts are non-empty and within the character limit; technical claims
// are grounded; and no two threads are duplicates.
//
// pkg is the source package, used to confirm grounding.
func (col XThreadCollection) Validate(pkg ReleasePackage) []string {
	var problems []string

	if col.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(col.Threads) == 0 {
		problems = append(problems, "collection has no threads")
	}
	if col.Metadata.ThreadCount != len(col.Threads) {
		problems = append(problems, fmt.Sprintf("metadata.threadCount %d != threads %d", col.Metadata.ThreadCount, len(col.Threads)))
	}

	grounded := groundedTerms(pkg)
	seenType := map[string]bool{}
	seenBody := map[string]bool{}

	for i, th := range col.Threads {
		where := fmt.Sprintf("thread %d (%s)", i+1, th.Type)

		if len(th.Posts) == 0 {
			problems = append(problems, where+": no posts")
		}
		if collapse(th.CTA) == "" {
			problems = append(problems, where+": missing CTA")
		}
		if len(th.Hashtags) == 0 {
			problems = append(problems, where+": no hashtags")
		}
		if collapse(th.EngagementPrompt) == "" {
			problems = append(problems, where+": missing engagement prompt")
		}
		if len(th.KeyTakeaways) == 0 {
			problems = append(problems, where+": no key takeaways")
		}

		// Posts: non-empty and within the character limit.
		for _, p := range th.Posts {
			if collapse(p.Content) == "" {
				problems = append(problems, fmt.Sprintf("%s post %d: empty content", where, p.Index))
			}
			if runeLen(p.Content) > MaxPostChars {
				problems = append(problems, fmt.Sprintf("%s post %d: %d chars exceeds the %d limit", where, p.Index, runeLen(p.Content), MaxPostChars))
			}
		}

		// Grounding: the thread must reference at least one grounded term.
		if !threadGrounded(th, grounded) {
			problems = append(problems, where+": not grounded in the release context")
		}

		// Neutrality: thread PROSE must be evergreen — no repository name or release
		// version (a CTA link is attribution, not body copy, so it's excluded).
		if prose := proseOnly(threadBody(th)); namesReleaseIdentity(prose, pkg) {
			lc := strings.ToLower(prose)
			if r := repoShort(pkg); r != "" && strings.Contains(lc, strings.ToLower(r)) {
				problems = append(problems, where+": thread names the repository (must be evergreen)")
			}
			if t := tag(pkg); t != "" && strings.Contains(prose, t) {
				problems = append(problems, where+": thread names the release version (must be evergreen)")
			}
			if strings.Contains(lc, "this release") || strings.Contains(lc, "the release") {
				problems = append(problems, where+": thread uses 'this/the release' framing (must be evergreen)")
			}
		}

		// Technical claims: every key takeaway must be grounded.
		for _, k := range th.KeyTakeaways {
			if !termGrounded(k, grounded) {
				problems = append(problems, fmt.Sprintf("%s: ungrounded technical claim %q", where, truncateForMsg(k)))
			}
		}

		// Duplicate detection by type and by concatenated post text.
		if seenType[th.Type] {
			problems = append(problems, where+": duplicate thread type")
		}
		seenType[th.Type] = true
		bkey := slug(threadBody(th))
		if bkey != "" && seenBody[bkey] {
			problems = append(problems, where+": duplicate thread body")
		}
		seenBody[bkey] = true
	}

	return problems
}

func threadBody(th Thread) string {
	var parts []string
	for _, p := range th.Posts {
		parts = append(parts, p.Content)
	}
	return strings.Join(parts, " ")
}

func threadGrounded(th Thread, grounded map[string]bool) bool {
	hay := strings.ToLower(th.Title + " " + threadBody(th) + " " + strings.Join(th.KeyTakeaways, " "))
	for term := range grounded {
		if term != "" && strings.Contains(hay, term) {
			return true
		}
	}
	return false
}

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

func truncateForMsg(s string) string {
	f := strings.Fields(s)
	if len(f) <= 8 {
		return strings.Join(f, " ")
	}
	return strings.Join(f[:8], " ") + "…"
}
