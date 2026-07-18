package voiceover

import "github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"

// planSyncCues derives the narration-to-visual alignment for a scene straight
// from the storyboard, so the voice never describes something before it appears.
// Cues reference real diagrams, code, camera moves, and overlays — nothing is
// invented.
func planSyncCues(sc storyboard.Scene) []SyncCue {
	var out []SyncCue

	for _, d := range sc.Diagrams {
		out = append(out, SyncCue{
			Visual: "Diagram Reveal",
			Cue:    "Begin the explanation only after the diagram has built; name each node as it highlights.",
			Target: d.Source,
		})
	}
	for _, c := range sc.Code {
		out = append(out, SyncCue{
			Visual: "Code Highlight",
			Cue:    "Speak the walkthrough in step with the highlighted lines.",
			Target: c.Language,
		})
	}
	if sc.Camera.Direction != "" && sc.Camera.Direction != "Static" {
		out = append(out, SyncCue{
			Visual: "Camera Move",
			Cue:    "Match the delivery to the camera move — start as it begins, resolve as it settles.",
			Target: sc.Camera.Direction,
		})
	}
	for _, o := range sc.Overlays {
		out = append(out, SyncCue{
			Visual: "Overlay",
			Cue:    "Land the on-screen text as you say it.",
			Target: o.Text,
		})
	}
	return out
}
