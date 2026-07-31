package voiceover

import (
	"context"
	"fmt"
	"strings"
)

// narration returns the spoken voice-over for a scene. The storyboard narration
// is already grounded in the Release Context, so it is the base text and the
// deterministic fallback. When a Model is configured, it is asked ONLY to refine
// that text into natural, engineer-to-engineer spoken delivery — never to add
// facts. Any model error or empty result falls back to the storyboard narration
// verbatim, so nothing is ever invented.
func (g *Generator) narration(ctx context.Context, base, sceneTitle, sceneType, dir string) string {
	base = collapse(base)
	if g.Model == nil || base == "" {
		return base
	}
	out, err := g.Model.Generate(ctx, narrationPrompt(sceneTitle, sceneType, dir, base))
	if err != nil {
		return base
	}
	if r := stripPreamble(collapse(strings.TrimSpace(out))); r != "" {
		// Refinement polishes spoken delivery — it must not SHORTEN the narration.
		// The storyboard narration is already sized to fill the scene's slot, so a
		// shorter rewrite under-fills the allocation and leaves dead air. Keep the
		// base when the rewrite drops words; a longer rewrite is bounded later by
		// fitNarrationToBudget.
		if wordCount(r) < wordCount(base) {
			return base
		}
		return r
	}
	return base
}

// stripPreamble removes assistant-style framing a refinement model prepends to
// the narration ("Here's the refined narration:", "Here is the rewritten
// version:") plus a single pair of wrapping quotes, leaving only the spoken
// words. Without this the framing leaks verbatim into the TTS script. Mirrors
// storyboard.stripNarrationPreamble.
func stripPreamble(s string) string {
	s = strings.TrimSpace(s)
	// Drop a short leading clause ending at the first ':' when it reads as the
	// model describing what it produced rather than the narration itself.
	if i := strings.IndexByte(s, ':'); i > 0 && i < 160 {
		head := strings.ToLower(s[:i])
		for _, m := range []string{"here is", "here's", "here are", "sure", "certainly",
			"refined", "rewritten", "rewrite", "revised", "version of", "as requested",
			"spoken", "narration", "the draft", "the base"} {
			if strings.Contains(head, m) {
				s = strings.TrimSpace(s[i+1:])
				break
			}
		}
	}
	s = strings.TrimSpace(s)
	for _, q := range []struct{ open, close string }{{"\"", "\""}, {"'", "'"}, {"“", "”"}} {
		if strings.HasPrefix(s, q.open) && strings.HasSuffix(s, q.close) && len(s) > len(q.open)+len(q.close) {
			s = strings.TrimSpace(s[len(q.open) : len(s)-len(q.close)])
			break
		}
	}
	return s
}

// fitNarrationToBudget trims narration to the words speakable within a scene's
// allocated slot at the given rate, on sentence boundaries only (never
// mid-sentence). The narration model is asked for "roughly the same length" but
// can still return more than the storyboard's base — this is the hard guarantee
// that the refined narration still fits its allocation, so the voice-over never
// reports a scene "over". Keeps whole leading sentences that fit (always at least
// the first); returns the input unchanged when it already fits.
//
// The budget includes fitToleranceSec so it matches the "fits" test exactly:
// narration that would already pass (est <= allocated + tolerance) is never
// trimmed. Using the bare allocation here dropped a whole trailing sentence for
// being a word or two over, collapsing narration well below its slot and leaving
// dead air — only narration that genuinely would NOT fit should be cut.
func fitNarrationToBudget(narration string, allocatedSec, wpm int) string {
	if allocatedSec <= 0 {
		return narration
	}
	budget := int(float64(allocatedSec+fitToleranceSec) * wordsPerSecond(wpm))
	if budget < 1 || wordCount(narration) <= budget {
		return narration
	}
	var kept []string
	used := 0
	for _, s := range sentences(narration) {
		w := wordCount(s)
		if len(kept) > 0 && used+w > budget {
			break
		}
		kept = append(kept, s)
		used += w
		if used >= budget {
			break
		}
	}
	if len(kept) == 0 {
		return narration
	}
	return collapse(strings.Join(kept, " "))
}

func narrationPrompt(title, sceneType, dir, base string) string {
	return fmt.Sprintf(
		"You are a narration director refining voice-over for one scene of a technical video.\n"+
			"Scene: %q (type: %s). Voice direction: %s.\n\n"+
			"Rewrite the NARRATION below so it sounds like an experienced software engineer "+
			"teaching another engineer: clear, natural, spoken (not written) English, warm but "+
			"precise. Keep it to 2–3 sentences and roughly the same length. Use ONLY the facts in "+
			"the narration — do NOT add features, numbers, or architecture, and do NOT describe "+
			"visuals. Output only the spoken narration text.\n\n"+
			"NARRATION:\n%s",
		title, sceneType, dir, base,
	)
}
