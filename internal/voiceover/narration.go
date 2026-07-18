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
	if r := collapse(strings.TrimSpace(out)); r != "" {
		return r
	}
	return base
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
