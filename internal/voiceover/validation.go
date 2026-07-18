package voiceover

import "fmt"

// Validate checks the voice-over script's structural invariants and returns a
// list of problems (empty when valid). It enforces the Milestone 5 rules: every
// scene has narration, narration fits its allocated time, timestamps are
// sequential and gap-free, pronunciation entries are unique within a scene,
// transition cues exist between scenes, and timing information is present.
func (s VoiceOverScript) Validate() []string {
	var problems []string

	if s.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(s.Scenes) == 0 {
		problems = append(problems, "script has no scenes")
	}
	if s.Metadata.SceneCount != len(s.Scenes) {
		problems = append(problems, fmt.Sprintf("metadata.sceneCount %d != scenes %d", s.Metadata.SceneCount, len(s.Scenes)))
	}

	prevEnd := 0
	for i, sc := range s.Scenes {
		where := fmt.Sprintf("scene %d", i+1)

		if sc.SceneNumber != i+1 {
			problems = append(problems, fmt.Sprintf("%s: sceneNumber %d out of sequence", where, sc.SceneNumber))
		}
		if sc.Title == "" {
			problems = append(problems, where+": empty title")
		}

		// Empty narration is rejected.
		if collapse(sc.Narration) == "" {
			problems = append(problems, where+": empty narration")
		}

		// Missing timing information is rejected.
		if sc.Duration.AllocatedSec <= 0 {
			problems = append(problems, where+": missing timing (allocatedSec <= 0)")
		}
		if sc.Timestamp.EndSec <= sc.Timestamp.StartSec {
			problems = append(problems, fmt.Sprintf("%s: timestamp end %d not after start %d", where, sc.Timestamp.EndSec, sc.Timestamp.StartSec))
		}

		// Timestamps are sequential and gap-free.
		if sc.Timestamp.StartSec != prevEnd {
			problems = append(problems, fmt.Sprintf("%s: timestamp start %d is not contiguous with previous end %d", where, sc.Timestamp.StartSec, prevEnd))
		}
		prevEnd = sc.Timestamp.EndSec

		// Narration fits the allocated scene time.
		if !sc.Duration.Fits {
			problems = append(problems, fmt.Sprintf("%s: narration ~%ds exceeds allocated %ds", where, sc.Duration.EstimatedSpeechSec, sc.Duration.AllocatedSec))
		}

		// Pronunciation entries are unique within the scene.
		seen := map[string]bool{}
		for _, p := range sc.Pronunciation {
			key := lower(p.Term)
			if seen[key] {
				problems = append(problems, fmt.Sprintf("%s: duplicate pronunciation entry %q", where, p.Term))
			}
			seen[key] = true
		}

		// Voice direction must be present.
		if sc.VoiceDirection == "" {
			problems = append(problems, where+": missing voice direction")
		}

		// Transition cues exist between scenes (every scene, incl. the closing
		// line on the last one).
		if collapse(sc.Transition) == "" {
			problems = append(problems, where+": missing transition cue")
		}
	}

	return problems
}
