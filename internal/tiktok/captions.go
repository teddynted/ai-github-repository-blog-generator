package tiktok

import "strings"

// captionMaxWords bounds a single burned-in caption chunk for readability.
const captionMaxWords = 5

// techTerms are highlighted in captions when they appear (TikTok viewers scan
// for the technical hook). Matched case-insensitively as whole words.
var techTerms = map[string]bool{
	"aws": true, "lambda": true, "cloudformation": true, "sqs": true, "s3": true,
	"iam": true, "github": true, "go": true, "golang": true, "json": true,
	"yaml": true, "api": true, "bedrock": true, "ollama": true, "eventbridge": true,
	"dynamodb": true, "architecture": true, "serverless": true, "ec2": true,
}

// planCaptions splits the full script into short, timed, burned-in subtitle
// chunks spread across the duration. Timing is proportional to each chunk's word
// count. The first chunk (hook) is emphasized; chunks containing a technical
// term are styled as highlights.
func planCaptions(script string, totalSec int) []Caption {
	words := strings.Fields(script)
	if len(words) == 0 || totalSec <= 0 {
		return nil
	}

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
		caps = append(caps, Caption{
			Text: strings.Join(ch, " "), StartSec: start, EndSec: end, Style: captionStyle(i, ch),
		})
		elapsed = end
	}
	return caps
}

func captionStyle(index int, chunk []string) string {
	if index == 0 {
		return "large-bold"
	}
	for _, w := range chunk {
		if techTerms[strings.ToLower(strings.TrimFunc(w, isPunct))] {
			return "highlight"
		}
	}
	return ""
}

func isPunct(r rune) bool {
	return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
}
