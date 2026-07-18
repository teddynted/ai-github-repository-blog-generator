package shorts

import "strings"

// captionMaxWords bounds a single burned-in caption chunk for readability.
const captionMaxWords = 6

// planCaptions splits the full script into short, timed, burned-in subtitle
// chunks spread across the Short's duration. Timing is proportional to each
// chunk's word count, so captions stay in sync with the narration. The first
// chunk (the hook) is styled for emphasis.
func planCaptions(script string, totalSec int) []Caption {
	words := strings.Fields(script)
	if len(words) == 0 || totalSec <= 0 {
		return nil
	}

	// Group words into <= captionMaxWords chunks.
	var chunks [][]string
	for i := 0; i < len(words); i += captionMaxWords {
		end := i + captionMaxWords
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, words[i:end])
	}

	caps := make([]Caption, 0, len(chunks))
	elapsed := 0
	for i, ch := range chunks {
		start := elapsed
		// Proportional slice; the last caption absorbs the remainder so the end
		// lands exactly on totalSec.
		var dur int
		if i == len(chunks)-1 {
			dur = totalSec - start
		} else {
			dur = (len(ch) * totalSec) / len(words)
			if dur < 1 {
				dur = 1
			}
		}
		end := start + dur
		if end > totalSec {
			end = totalSec
		}
		style := ""
		if i == 0 {
			style = "large-bold"
		}
		caps = append(caps, Caption{Text: strings.Join(ch, " "), StartSec: start, EndSec: end, Style: style})
		elapsed = end
	}
	return caps
}
