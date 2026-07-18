package storyboard

import "fmt"

// Validate checks the storyboard's structural invariants and returns a list of
// problems (empty when valid). It is used by tests and can gate downstream
// video generation.
func (s Storyboard) Validate() []string {
	var problems []string
	if s.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(s.Scenes) == 0 {
		problems = append(problems, "storyboard has no scenes")
	}
	if s.Video.SceneCount != len(s.Scenes) {
		problems = append(problems, fmt.Sprintf("video.sceneCount %d != scenes %d", s.Video.SceneCount, len(s.Scenes)))
	}

	for i, sc := range s.Scenes {
		where := fmt.Sprintf("scene %d", i+1)
		if sc.SceneNumber != i+1 {
			problems = append(problems, fmt.Sprintf("%s: sceneNumber %d out of sequence", where, sc.SceneNumber))
		}
		if sc.Title == "" {
			problems = append(problems, where+": empty title")
		}
		if sc.Narration == "" {
			problems = append(problems, where+": empty narration")
		}
		d := sc.Duration
		if d.RecommendedSec < sceneMinSec || d.RecommendedSec > sceneMaxSec {
			problems = append(problems, fmt.Sprintf("%s: recommended %ds outside [%d,%d]", where, d.RecommendedSec, sceneMinSec, sceneMaxSec))
		}
		if d.MinSec > d.RecommendedSec || d.MaxSec < d.RecommendedSec {
			problems = append(problems, fmt.Sprintf("%s: duration window [%d,%d] does not contain recommended %d", where, d.MinSec, d.MaxSec, d.RecommendedSec))
		}
		if sc.Camera.Direction == "" {
			problems = append(problems, where+": missing camera direction")
		}
		if sc.Transition.Type == "" {
			problems = append(problems, where+": missing transition")
		}
	}
	return problems
}
