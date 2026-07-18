package youtube

import "math"

// Long-form target band (seconds): 10–20 minutes.
const (
	targetMinRuntimeSec = 10 * 60
	targetMaxRuntimeSec = 20 * 60
)

// defaultWordsPerMinute is the long-form educational narration rate; slightly
// slower than a short so demonstrations and diagrams have room to breathe.
const defaultWordsPerMinute = 150

// Hook and framing bounds (seconds).
const (
	hookMinSec       = 15
	hookMaxSec       = 30
	introMinSec      = 30
	introMaxSec      = 60
	conclusionMinSec = 20
	conclusionMaxSec = 45
)

// speakingSeconds estimates spoken time for n words at wpm.
func speakingSeconds(words, wpm int) int {
	if wpm <= 0 {
		wpm = defaultWordsPerMinute
	}
	if words <= 0 {
		return 0
	}
	return int(math.Round(float64(words) / (float64(wpm) / 60.0)))
}

// chapterDuration derives a chapter's timing window from the storyboard's
// allocated seconds, expanded modestly for the fuller teaching narration a
// long-form video carries.
func chapterDuration(storyboardSec int) Duration {
	target := storyboardSec
	if target <= 0 {
		target = 20
	}
	return Duration{
		MinSec:    clampInt(target-5, 5, target),
		MaxSec:    target + 15,
		TargetSec: target,
	}
}
