package youtube

import (
	"context"
	"fmt"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
)

// chapters maps every storyboard scene to one chapter (1:1), so chapters align
// with the storyboard and every scene is represented. Each chapter's teaching
// script is built from the voice-over narration plus grounded walkthrough detail
// from the Release Context, then optionally expanded by the Model. Timestamps
// are assigned sequentially starting at startSec (after the hook).
func (g *Generator) chapters(ctx context.Context, pkg ReleasePackage, startSec int) []Chapter {
	voMap := voiceOverByScene(pkg.VoiceOver)
	c := pkg.Context

	chapters := make([]Chapter, 0, len(pkg.Storyboard.Scenes))
	running := startSec
	body := 0 // counts body chapters, for engagement cadence

	for _, sc := range pkg.Storyboard.Scenes {
		vo := voMap[sc.SceneNumber]

		base := chapterBase(sc, vo, c)
		script := g.expandChapter(ctx, sc, base)
		// Compress an over-long expansion (#6) so a chapter never runs away — with
		// tighter caps for the two summary chapters — then size the slot to the
		// capped narration so timeline and metadata stay consistent (#4).
		script = capWordsAtSentence(script, chapterWordCap(sc.Title))
		dur := chapterDurationFromScript(script, g.wpm())

		isBody := sc.Type != "introduction" && sc.Type != "conclusion"
		if isBody {
			body++
		}

		ch := Chapter{
			Number:              len(chapters) + 1,
			Title:               sc.Title,
			Type:                sc.Type,
			Timestamp:           stamp(running, running+dur.TargetSec),
			Duration:            dur,
			Script:              script,
			VisualReferences:    visualReferencesFor(sc.Type, c),
			StoryboardScenes:    []int{sc.SceneNumber},
			VoiceOverReferences: []int{sc.SceneNumber},
			Demonstration:       planDemonstration(sc),
			Callouts:            planCallouts(sc.Type, c),
			Engagement:          planEngagement(sc.Type, body),
			Transition:          transitionFor(vo, sc),
			WordCount:           wordCount(script),
		}
		chapters = append(chapters, ch)
		running += dur.TargetSec
	}
	return chapters
}

// chapterWordCap is the maximum spoken words a chapter may carry. The two
// summary chapters are held tighter (they otherwise re-list the same components);
// every other chapter is bounded so a runaway expansion is compressed.
func chapterWordCap(title string) int {
	switch t := strings.ToLower(title); {
	case strings.Contains(t, "architecture summary"):
		return 120
	case strings.Contains(t, "architecture diagram"):
		return 140
	default:
		return 180
	}
}

// chapterBase assembles the grounded teaching narration for a chapter: the
// voice-over narration first, then any deep-walkthrough detail from the context.
func chapterBase(sc storyboard.Scene, vo voiceover.Scene, c *rc.ReleaseContext) string {
	var b strings.Builder
	if vo.Narration != "" {
		b.WriteString(vo.Narration)
	} else if sc.Narration != "" {
		b.WriteString(sc.Narration)
	}
	if detail := walkthroughDetail(sc.Type, c); detail != "" {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(detail)
	}
	return collapse(b.String())
}

// expandChapter uses the Model to turn the grounded base into fuller, engaging
// teaching narration — never adding facts. Falls back to the base.
func (g *Generator) expandChapter(ctx context.Context, sc storyboard.Scene, base string) string {
	if g.Model == nil || base == "" {
		return base
	}
	out, err := g.Model.Generate(ctx, chapterPrompt(sc.Title, sc.Type, base))
	if err != nil {
		return base
	}
	if r := collapse(strings.TrimSpace(out)); r != "" {
		return r
	}
	return base
}

func chapterPrompt(title, typ, base string) string {
	return fmt.Sprintf(
		"You are scripting one chapter of a long-form technical YouTube video.\n"+
			"Chapter: %q (type: %s).\n\n"+
			"Expand the NOTES below into engaging, spoken teaching narration — an experienced engineer explaining "+
			"to a peer. Keep it accurate and on-topic, roughly 3–6 sentences. Use ONLY the facts in the notes; do NOT "+
			"invent architecture, numbers, or file names. Output only the narration.\n\nNOTES:\n%s",
		title, typ, base)
}

// transitionFor prefers the voice-over's spoken bridge; otherwise falls back to
// the storyboard scene's transition type.
func transitionFor(vo voiceover.Scene, sc storyboard.Scene) string {
	if collapse(vo.Transition) != "" {
		return vo.Transition
	}
	if sc.Transition.Type != "" {
		return "Transition: " + sc.Transition.Type
	}
	return "Cut to the next chapter."
}

func voiceOverByScene(vo voiceover.VoiceOverScript) map[int]voiceover.Scene {
	m := make(map[int]voiceover.Scene, len(vo.Scenes))
	for _, s := range vo.Scenes {
		m[s.SceneNumber] = s
	}
	return m
}
