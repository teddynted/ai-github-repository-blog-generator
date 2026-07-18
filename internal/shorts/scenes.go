package shorts

import "strings"

// beat is one narrative unit that becomes a scene.
type beat struct {
	kind      sceneKind
	narration string
}

// planScenes builds the scene-by-scene breakdown for a Short: a hook scene, one
// or two core scenes, and a takeaway/CTA scene. Durations are split across the
// Short's total so they sum exactly. Visuals/camera/animation are grounded by
// the angle.
func planScenes(c candidate, hook, core, takeaway, cta string, totalSec int, visuals []Visual) []Scene {
	beats := buildBeats(hook, core, takeaway, cta)
	durs := splitDuration(totalSec, len(beats))

	scenes := make([]Scene, len(beats))
	for i, bt := range beats {
		isLast := i == len(beats)-1
		scenes[i] = Scene{
			Number:      i + 1,
			DurationSec: durs[i],
			Visual:      visualForScene(bt.kind, i, visuals),
			Camera:      cameraFor(c.Angle, bt.kind),
			Animation:   animationFor(c.Angle, bt.kind),
			Overlay:     overlayFor(bt.kind, c),
			Transition:  transitionFor(bt.kind, isLast),
			Narration:   bt.narration,
		}
	}
	return scenes
}

// buildBeats splits the script into hook / core (1–2) / takeaway beats.
func buildBeats(hook, core, takeaway, cta string) []beat {
	beats := []beat{{kind: kindHook, narration: collapse(hook)}}

	coreSentences := splitSentences(core)
	switch {
	case len(coreSentences) >= 2:
		beats = append(beats,
			beat{kind: kindCore, narration: coreSentences[0]},
			beat{kind: kindCore, narration: strings.Join(coreSentences[1:], " ")},
		)
	case len(coreSentences) == 1:
		beats = append(beats, beat{kind: kindCore, narration: coreSentences[0]})
	}

	tail := collapse(strings.TrimSpace(takeaway + " " + cta))
	beats = append(beats, beat{kind: kindTakeaway, narration: tail})
	return beats
}

func splitSentences(s string) []string {
	s = collapse(s)
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if (s[i] == '.' || s[i] == '!' || s[i] == '?') && (i+1 >= len(s) || s[i+1] == ' ') {
			if seg := strings.TrimSpace(s[start : i+1]); seg != "" {
				out = append(out, seg)
			}
			start = i + 1
		}
	}
	if seg := strings.TrimSpace(s[start:]); seg != "" {
		out = append(out, seg)
	}
	return out
}

// visualForScene picks a grounded visual description for a scene. The hook uses
// the first visual, core scenes step through them, and the takeaway closes on
// the repo end card (the last visual).
func visualForScene(kind sceneKind, index int, visuals []Visual) string {
	if len(visuals) == 0 {
		return "Full-screen title card."
	}
	if kind == kindTakeaway {
		return visuals[len(visuals)-1].Description
	}
	return visuals[index%len(visuals)].Description
}

func overlayFor(kind sceneKind, c candidate) string {
	switch kind {
	case kindHook:
		return c.Angle
	case kindTakeaway:
		return "Watch the full video →"
	default:
		return ""
	}
}
