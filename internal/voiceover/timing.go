package voiceover

import "math"

// defaultWordsPerMinute is a natural spoken-narration rate for technical
// content (~2.6 words/second), matching the storyboard's pacing model so the
// voice-over and storyboard timelines stay consistent.
const defaultWordsPerMinute = 156

// fitToleranceSec is how far the estimated speech time may exceed the allocated
// scene time before the narration is considered not to fit. A small tolerance
// absorbs rounding between the storyboard's and the voice-over's rate.
const fitToleranceSec = 2

// wordsPerSecond converts the configured words-per-minute to words/second.
func wordsPerSecond(wpm int) float64 {
	if wpm <= 0 {
		wpm = defaultWordsPerMinute
	}
	return float64(wpm) / 60.0
}

// speechSeconds estimates how long it takes to speak n words at wpm.
func speechSeconds(words, wpm int) int {
	if words <= 0 {
		return 0
	}
	return int(math.Round(float64(words) / wordsPerSecond(wpm)))
}

// planDuration reconciles the storyboard's allocated time for a scene with the
// estimated time to speak the (possibly re-polished) narration.
func planDuration(narration string, allocatedSec, wpm int) Duration {
	est := speechSeconds(wordCount(narration), wpm)
	return Duration{
		AllocatedSec:       allocatedSec,
		EstimatedSpeechSec: est,
		Fits:               est <= allocatedSec+fitToleranceSec,
	}
}

// timeline assigns each scene a sequential, gap-free timestamp from the running
// total of allocated durations. Deterministic by construction: scene i starts
// exactly where scene i-1 ended.
func timeline(allocations []int) []Timestamp {
	stamps := make([]Timestamp, len(allocations))
	start := 0
	for i, dur := range allocations {
		if dur < 0 {
			dur = 0
		}
		end := start + dur
		stamps[i] = Timestamp{
			Start:    clock(start),
			End:      clock(end),
			StartSec: start,
			EndSec:   end,
			Label:    clock(start) + "–" + clock(end),
		}
		start = end
	}
	return stamps
}
