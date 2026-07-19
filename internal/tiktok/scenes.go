package tiktok

import "strings"

// beat is one narrative unit that becomes a scene.
type beat struct {
	kind      sceneKind
	narration string
}

// planScenes builds the scene-by-scene breakdown for a TikTok: a hook scene, a
// problem scene, one or two solution scenes, and an outro scene that carries the
// takeaway, engagement prompt, and CTA. Durations split across the total so they
// sum exactly. Visuals/camera/animation are grounded by the topic.
func planScenes(t topic, hook, problem, solution, takeaway, engagement, cta string, totalSec int, visuals []Visual) []Scene {
	beats := buildBeats(hook, problem, solution, takeaway, engagement, cta)
	durs := splitDuration(totalSec, len(beats))

	scenes := make([]Scene, len(beats))
	for i, bt := range beats {
		isLast := i == len(beats)-1
		scenes[i] = Scene{
			Number:      i + 1,
			DurationSec: durs[i],
			Narration:   bt.narration,
			Visual:      visualForScene(bt.kind, i, visuals),
			Camera:      cameraFor(t.Topic, bt.kind),
			Animation:   animationFor(t.Topic, bt.kind),
			Overlay:     overlayFor(bt.kind, t),
			Transition:  transitionFor(bt.kind, isLast),
		}
	}
	return scenes
}

func buildBeats(hook, problem, solution, takeaway, engagement, cta string) []beat {
	beats := []beat{{kind: kindHook, narration: collapse(hook)}}
	if collapse(problem) != "" {
		beats = append(beats, beat{kind: kindProblem, narration: collapse(problem)})
	}

	sols := splitSentences(solution)
	switch {
	case len(sols) >= 2:
		beats = append(beats,
			beat{kind: kindSolution, narration: sols[0]},
			beat{kind: kindSolution, narration: strings.Join(sols[1:], " ")},
		)
	case len(sols) == 1:
		beats = append(beats, beat{kind: kindSolution, narration: sols[0]})
	}

	outro := collapse(strings.TrimSpace(takeaway + " " + engagement + " " + cta))
	beats = append(beats, beat{kind: kindOutro, narration: outro})
	return beats
}

func visualForScene(kind sceneKind, index int, visuals []Visual) string {
	if len(visuals) == 0 {
		return "Full-screen title card."
	}
	if kind == kindOutro {
		return visuals[len(visuals)-1].Description
	}
	return visuals[index%len(visuals)].Description
}

func overlayFor(kind sceneKind, t topic) string {
	switch kind {
	case kindHook:
		return t.Topic
	case kindOutro:
		return "Follow for more →"
	default:
		return ""
	}
}
