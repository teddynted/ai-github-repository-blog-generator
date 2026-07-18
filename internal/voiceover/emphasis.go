package voiceover

import "strings"

// emphasisCues are lead-in phrases that, when present in narration, should
// receive vocal emphasis to guide the listener's attention. Emphasis is for
// clarity, not theatre — so the list is deliberately restrained.
var emphasisCues = []string{
	"notice", "the important point", "pay particular attention", "pay attention",
	"importantly", "crucially", "the key", "key point", "in short",
	"dramatically", "significantly", "the takeaway", "remember", "critically",
}

// planEmphasis returns the words and short phrases in a scene that should be
// stressed. It is deterministic: it surfaces any clarity cue phrases present in
// the narration, then the release tag (a proper noun that must land clearly),
// capped so a scene never over-emphasizes. No word is emitted unless it (or its
// cue) appears in the narration.
func planEmphasis(narration, releaseTag string) []string {
	lc := strings.ToLower(narration)
	var out []string
	for _, cue := range emphasisCues {
		if strings.Contains(lc, cue) {
			out = append(out, cue)
		}
	}
	if releaseTag != "" && containsWord(narration, releaseTag) {
		out = append(out, releaseTag)
	}
	out = dedupe(out)
	if len(out) > maxEmphasisPerScene {
		out = out[:maxEmphasisPerScene]
	}
	return out
}

const maxEmphasisPerScene = 4
