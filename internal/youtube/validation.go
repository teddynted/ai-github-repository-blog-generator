package youtube

import "fmt"

// Validate checks the YouTube script's structural invariants and returns a list
// of problems (empty when valid). It enforces the Milestone 6 rules: every
// storyboard scene is represented, every chapter has narration, timestamps are
// sequential, runtime is internally consistent, and the required framing
// sections (introduction, conclusion, CTA) are present.
//
// sceneCount is the number of scenes in the source storyboard; pass it so the
// "every scene represented" check is exact.
func (s YouTubeScript) Validate(sceneCount int) []string {
	var problems []string

	if s.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(s.Chapters) == 0 {
		problems = append(problems, "script has no chapters")
	}

	// Introduction and conclusion must be present.
	if collapse(s.Introduction.Script) == "" {
		problems = append(problems, "missing introduction")
	}
	if collapse(s.Conclusion.Script) == "" {
		problems = append(problems, "missing conclusion")
	}

	// A call to action must be present.
	if collapse(s.CallToAction.Script) == "" && len(s.CallToAction.Items) == 0 {
		problems = append(problems, "missing call to action")
	}

	// Every chapter has narration; timestamps are sequential and gap-free.
	represented := map[int]bool{}
	prevEnd := s.Hook.Timestamp.EndSec
	for i, ch := range s.Chapters {
		where := fmt.Sprintf("chapter %d", i+1)
		if ch.Number != i+1 {
			problems = append(problems, fmt.Sprintf("%s: number %d out of sequence", where, ch.Number))
		}
		if ch.Title == "" {
			problems = append(problems, where+": empty title")
		}
		if collapse(ch.Script) == "" {
			problems = append(problems, where+": empty narration")
		}
		if ch.Timestamp.EndSec <= ch.Timestamp.StartSec {
			problems = append(problems, fmt.Sprintf("%s: timestamp end %d not after start %d", where, ch.Timestamp.EndSec, ch.Timestamp.StartSec))
		}
		if ch.Timestamp.StartSec != prevEnd {
			problems = append(problems, fmt.Sprintf("%s: timestamp start %d is not contiguous with previous end %d", where, ch.Timestamp.StartSec, prevEnd))
		}
		prevEnd = ch.Timestamp.EndSec
		for _, n := range ch.StoryboardScenes {
			represented[n] = true
		}
	}

	// Every storyboard scene must be represented by some chapter.
	if sceneCount > 0 {
		var missing []int
		for n := 1; n <= sceneCount; n++ {
			if !represented[n] {
				missing = append(missing, n)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("storyboard scenes not represented in any chapter: %v", missing))
		}
	}

	// Runtime must be internally consistent: hook + all chapter durations.
	computed := s.Hook.DurationSec
	for _, ch := range s.Chapters {
		computed += ch.Duration.TargetSec
	}
	if computed != s.Video.DurationSec {
		problems = append(problems, fmt.Sprintf("runtime mismatch: video.durationSec %d != hook+chapters %d", s.Video.DurationSec, computed))
	}

	return problems
}
