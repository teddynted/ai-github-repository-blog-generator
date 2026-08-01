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

// minChapterSec floors a chapter's slot so a very short chapter still reads.
const minChapterSec = 8

// chapterDurationFromScript sizes a chapter's slot to the time it takes to speak
// its (already length-capped) narration. Deriving the slot from the actual words
// — rather than the storyboard's much shorter voice-over slot — keeps the video
// timeline and the "speaking time"/"runtime" metadata consistent (they were
// 4:54 vs 12:40 when the slot came from the short storyboard timing but the
// chapter narration had been expanded for long-form).
func chapterDurationFromScript(script string, wpm int) Duration {
	target := speakingSeconds(wordCount(script), wpm)
	if target < minChapterSec {
		target = minChapterSec
	}
	return Duration{
		MinSec:    clampInt(target-5, 5, target),
		MaxSec:    target + 15,
		TargetSec: target,
	}
}
