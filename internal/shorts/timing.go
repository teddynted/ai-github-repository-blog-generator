package shorts

import "math"

// Short duration bounds (seconds).
const (
	shortMinSec = 30
	shortMaxSec = 60
)

// fastWordsPerSecond is the energetic Shorts narration pace (~168 wpm).
const fastWordsPerSecond = 2.8

// durationFor derives a Short's duration from its script length at Shorts pace,
// clamped to the 30–60s window.
func durationFor(script string) int {
	sec := int(math.Round(float64(wordCount(script)) / fastWordsPerSecond))
	return clampInt(sec, shortMinSec, shortMaxSec)
}

// splitDuration allocates total seconds across n scenes, giving the remainder to
// the earlier scenes so the sum is exact.
func splitDuration(total, n int) []int {
	if n <= 0 {
		return nil
	}
	base := total / n
	rem := total - base*n
	out := make([]int, n)
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
	}
	return out
}
