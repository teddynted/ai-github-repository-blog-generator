package tiktok

import "fmt"

// Validate checks the collection's structural invariants and returns a list of
// problems (empty when valid). It enforces the Milestone 8 rules: every TikTok
// has a hook, captions, an engagement prompt, and a CTA; durations stay in the
// 20–60s window; visual references are grounded in the source artifacts; and no
// two videos are duplicates.
//
// pkg is the source package, used to confirm visual grounding.
func (col TikTokCollection) Validate(pkg ReleasePackage) []string {
	var problems []string

	if col.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(col.Videos) == 0 {
		problems = append(problems, "collection has no videos")
	}
	if col.Metadata.VideoCount != len(col.Videos) {
		problems = append(problems, fmt.Sprintf("metadata.videoCount %d != videos %d", col.Metadata.VideoCount, len(col.Videos)))
	}

	grounded := groundedRefs(pkg)
	seen := map[string]bool{}

	for i, v := range col.Videos {
		where := fmt.Sprintf("video %d", i+1)

		if collapse(v.Hook) == "" {
			problems = append(problems, where+": missing hook")
		}
		if len(v.Captions) == 0 {
			problems = append(problems, where+": no captions")
		}
		if collapse(v.EngagementPrompt) == "" {
			problems = append(problems, where+": missing engagement prompt")
		}
		if collapse(v.CTA) == "" {
			problems = append(problems, where+": missing CTA")
		}
		if v.DurationSec < tiktokMinSec || v.DurationSec > tiktokMaxSec {
			problems = append(problems, fmt.Sprintf("%s: duration %ds outside [%d,%d]", where, v.DurationSec, tiktokMinSec, tiktokMaxSec))
		}

		if len(v.Scenes) == 0 {
			problems = append(problems, where+": no scenes")
		}
		for j, sc := range v.Scenes {
			if collapse(sc.Narration) == "" {
				problems = append(problems, fmt.Sprintf("%s scene %d: empty narration", where, j+1))
			}
		}

		// Visual references must be grounded in the source artifacts.
		if len(v.Visuals) == 0 {
			problems = append(problems, where+": no visuals")
		}
		var anyGrounded bool
		for _, vis := range v.Visuals {
			if vis.Reference != "" && grounded[collapse(vis.Reference)] {
				anyGrounded = true
				break
			}
		}
		if len(v.Visuals) > 0 && !anyGrounded {
			problems = append(problems, where+": no visual reference is grounded in the release artifacts")
		}

		// Duplicate detection by normalized title.
		key := slug(v.Title)
		if seen[key] {
			problems = append(problems, fmt.Sprintf("%s: duplicate TikTok title %q", where, v.Title))
		}
		seen[key] = true
	}

	return problems
}
