package tiktok

import "math"

// TikTok duration bounds (seconds).
const (
	tiktokMinSec = 20
	tiktokMaxSec = 60
)

// fastWordsPerSecond is TikTok's brisk narration pace (~174 wpm).
const fastWordsPerSecond = 2.9

// durationFor derives a video's duration from its script length at TikTok pace,
// clamped to the 20–60s window.
func durationFor(script string) int {
	sec := int(math.Round(float64(wordCount(script)) / fastWordsPerSecond))
	return clampInt(sec, tiktokMinSec, tiktokMaxSec)
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
