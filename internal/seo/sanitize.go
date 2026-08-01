package seo

import "strings"

// modelCommentaryMarkers are phrases that signal the Model returned meta-commentary,
// a refusal, or a validation critique instead of the requested value. Such text
// must never reach the artifact — the SEO metadata carries values, not the
// model's reasoning about them.
var modelCommentaryMarkers = []string{
	"as an ai", "as a language model", "i cannot", "i can't", "i apologize",
	"i'm unable", "i am unable", "the draft is", "there are no facts",
	"no facts to", "my instructions", "weak because", "cannot verify",
	"insufficient information", "i don't have enough",
}

// sanitizeModelText returns the Model's output when it is a clean value, or the
// deterministic fallback when the output is empty or leaked meta-commentary. This
// keeps model reasoning ("the draft is weak because…") out of the artifact.
func sanitizeModelText(out, fallback string) string {
	c := collapse(strings.TrimSpace(out))
	if c == "" {
		return fallback
	}
	lc := strings.ToLower(c)
	for _, m := range modelCommentaryMarkers {
		if strings.Contains(lc, m) {
			return fallback
		}
	}
	return c
}
