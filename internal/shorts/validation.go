package shorts

import "fmt"

// Validate checks the collection's structural invariants and returns a list of
// problems (empty when valid). It enforces the Milestone 7 rules: every Short
// has a hook, a CTA, captions, and non-empty scenes; durations stay in the
// 30–60s window; visual references are grounded in the source artifacts; and no
// two Shorts are duplicates.
//
// pkg is the source package, used to confirm visual grounding.
func (col ShortsCollection) Validate(pkg ReleasePackage) []string {
	var problems []string

	if col.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(col.Shorts) == 0 {
		problems = append(problems, "collection has no shorts")
	}
	if col.Metadata.ShortCount != len(col.Shorts) {
		problems = append(problems, fmt.Sprintf("metadata.shortCount %d != shorts %d", col.Metadata.ShortCount, len(col.Shorts)))
	}

	grounded := groundedRefs(pkg)
	seen := map[string]bool{}

	for i, s := range col.Shorts {
		where := fmt.Sprintf("short %d", i+1)

		if collapse(s.Hook) == "" {
			problems = append(problems, where+": missing hook")
		}
		if collapse(s.CTA) == "" {
			problems = append(problems, where+": missing CTA")
		}
		if len(s.Captions) == 0 {
			problems = append(problems, where+": no captions")
		}
		if s.DurationSec < shortMinSec || s.DurationSec > shortMaxSec {
			problems = append(problems, fmt.Sprintf("%s: duration %ds outside [%d,%d]", where, s.DurationSec, shortMinSec, shortMaxSec))
		}

		// Non-empty scenes: at least one scene, each with narration and a visual.
		if len(s.Scenes) == 0 {
			problems = append(problems, where+": no scenes")
		}
		for j, sc := range s.Scenes {
			if collapse(sc.Narration) == "" {
				problems = append(problems, fmt.Sprintf("%s scene %d: empty narration", where, j+1))
			}
			if collapse(sc.Visual) == "" {
				problems = append(problems, fmt.Sprintf("%s scene %d: empty visual", where, j+1))
			}
		}

		// Visual references must be grounded in the source artifacts.
		if len(s.Visuals) == 0 {
			problems = append(problems, where+": no visuals")
		}
		var anyGrounded bool
		for _, v := range s.Visuals {
			if v.Reference != "" && grounded[collapse(v.Reference)] {
				anyGrounded = true
				break
			}
		}
		if len(s.Visuals) > 0 && !anyGrounded {
			problems = append(problems, where+": no visual reference is grounded in the release artifacts")
		}

		// Duplicate detection by normalized title.
		key := slug(s.Title)
		if seen[key] {
			problems = append(problems, fmt.Sprintf("%s: duplicate Short title %q", where, s.Title))
		}
		seen[key] = true
	}

	return problems
}
