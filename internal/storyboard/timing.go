package storyboard

import "math"

// defaultWordsPerSecond is a natural narration pace (145 wpm). It MUST match the
// voice-over estimator (voiceover.defaultWordsPerMinute) so a scene's allocated
// duration and its estimated spoken time are computed on the same clock — that
// equality is what guarantees narration never exceeds its allocation.
const defaultWordsPerSecond = 145.0 / 60.0

// sceneMinSec / sceneMaxSec bound a single scene's recommended duration.
const (
	sceneMinSec = 5
	sceneMaxSec = 25
)

// planDuration derives a scene's timing from its narration length at the given
// speaking rate, clamped to the per-scene window.
func planDuration(narration string, wordsPerSecond float64) Duration {
	if wordsPerSecond <= 0 {
		wordsPerSecond = defaultWordsPerSecond
	}
	rec := int(math.Round(float64(wordCount(narration)) / wordsPerSecond))
	rec = clampInt(rec, sceneMinSec, sceneMaxSec)
	return Duration{
		MinSec:         clampInt(rec-3, 4, rec),
		MaxSec:         clampInt(rec+5, rec, 30),
		RecommendedSec: rec,
		Pacing:         pacingFor(rec),
	}
}

func pacingFor(rec int) string {
	switch {
	case rec <= 8:
		return "fast"
	case rec <= 16:
		return "medium"
	default:
		return "slow"
	}
}
