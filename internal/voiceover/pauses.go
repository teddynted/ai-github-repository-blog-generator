package voiceover

// Pause durations (milliseconds) map to SSML <break time="…"> values.
const (
	pauseShortMs  = 300
	pauseMediumMs = 600
	pauseLongMs   = 1000
)

// pauseInput carries the storyboard facts the pause planner needs.
type pauseInput struct {
	SceneType  string
	Narration  string
	HasDiagram bool
	HasCode    bool
	IsLast     bool
}

// planPauses inserts purposeful silence markers: a short settle after the
// opening, a medium pause before a diagram reveal or a demonstration, a long
// pause after a key takeaway, and a closing pause. Markers are deterministic and
// deduplicated by position — pacing, not clutter.
func planPauses(in pauseInput) []Pause {
	var out []Pause

	// A brief settle after the first sentence lets the opener land.
	if first := firstSentence(in.Narration); first != "" {
		out = append(out, Pause{
			Type: "short", Position: "opening", DurationMs: pauseShortMs,
			AfterText: first, Note: "Let the opening line settle before continuing.",
		})
	}

	// Pause before a diagram appears so the reveal has room.
	if in.HasDiagram {
		out = append(out, Pause{
			Type: "medium", Position: "before-diagram", DurationMs: pauseMediumMs,
			Note: "Pause before introducing the diagram; let it build.",
		})
	}

	// Pause before a code walkthrough or terminal demonstration.
	if in.HasCode {
		out = append(out, Pause{
			Type: "medium", Position: "before-demonstration", DurationMs: pauseMediumMs,
			Note: "Pause before the demonstration so the viewer can refocus.",
		})
	}

	// Let a key takeaway land on outcome-oriented scenes.
	switch in.SceneType {
	case "results", "lessons", "conclusion":
		out = append(out, Pause{
			Type: "long", Position: "after-key-takeaway", DurationMs: pauseLongMs,
			Note: "Hold after the key takeaway before moving on.",
		})
	}

	// A closing beat hands cleanly to the transition.
	out = append(out, Pause{
		Type: "short", Position: "closing", DurationMs: pauseShortMs,
		Note: "Brief pause before the transition.",
	})

	return dedupePauses(out)
}

// dedupePauses keeps the first pause for each position.
func dedupePauses(in []Pause) []Pause {
	seen := map[string]bool{}
	out := make([]Pause, 0, len(in))
	for _, p := range in {
		if seen[p.Position] {
			continue
		}
		seen[p.Position] = true
		out = append(out, p)
	}
	return out
}
