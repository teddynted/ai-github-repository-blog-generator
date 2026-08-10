package subtitles

import (
	"strings"
	"testing"
)

// tw builds a sequence of TimedWords at a fixed cadence for deterministic tests.
func tw(words ...string) []TimedWord {
	out := make([]TimedWord, len(words))
	t := 0
	for i, w := range words {
		out[i] = TimedWord{Word: w, StartMs: t, EndMs: t + 400}
		t += 400
	}
	return out
}

func TestWrapConstraintsAndSentenceBreaks(t *testing.T) {
	words := tw("GitHub", "sends", "webhooks", "to", "EventBridge.", "EventBridge", "invokes", "Lambda", "which", "dispatches", "to", "OpenClaw.")
	cues := Wrap(words, nil)
	if len(cues) < 2 {
		t.Fatalf("want at least 2 cues (sentence breaks), got %d", len(cues))
	}
	for _, c := range cues {
		if len(c.Lines) > MaxLines {
			t.Errorf("cue has %d lines (max %d): %v", len(c.Lines), MaxLines, c.Lines)
		}
		for _, ln := range c.Lines {
			if len(ln) > MaxLineChars {
				t.Errorf("line exceeds %d chars: %q", MaxLineChars, ln)
			}
		}
		if c.EndMs < c.StartMs {
			t.Errorf("cue end before start: %+v", c)
		}
	}
	// Cues must be monotonic and non-overlapping (each starts at/after the prior end).
	for i := 1; i < len(cues); i++ {
		if cues[i].StartMs < cues[i-1].EndMs {
			t.Errorf("cue %d starts (%d) before cue %d ends (%d)", i, cues[i].StartMs, i-1, cues[i-1].EndMs)
		}
	}
	// The first cue ends at the first sentence boundary.
	if !strings.Contains(strings.Join(cues[0].Lines, " "), "EventBridge") {
		t.Errorf("first cue = %v", cues[0].Lines)
	}
}

func TestAtomicPhraseNeverSplit(t *testing.T) {
	// "Amazon Bedrock" must stay on one line even near the wrap boundary.
	words := tw("The", "orchestrator", "falls", "back", "to", "Amazon", "Bedrock", "quickly.")
	cues := Wrap(words, []string{"Amazon Bedrock", "AWS Lambda"})
	joinedLines := map[string]bool{}
	for _, c := range cues {
		for _, ln := range c.Lines {
			joinedLines[ln] = true
		}
	}
	// No line should contain "Amazon" without "Bedrock" (i.e. never split).
	for ln := range joinedLines {
		if strings.Contains(ln, "Amazon") && !strings.Contains(ln, "Amazon Bedrock") {
			t.Errorf("Amazon Bedrock split across lines: %q", ln)
		}
	}
}

func TestTerminologyPreservedVerbatim(t *testing.T) {
	words := tw("OpenClaw", "runs", "Ollama", "then", "n8n", "logs", "to", "CloudWatch.")
	out := SRT(Wrap(words, nil))
	for _, term := range []string{"OpenClaw", "Ollama", "n8n", "CloudWatch"} {
		if !strings.Contains(out, term) {
			t.Errorf("protected term %q not preserved verbatim in SRT:\n%s", term, out)
		}
	}
}

func TestSRTFormat(t *testing.T) {
	cues := []Cue{{StartMs: 0, EndMs: 2500, Lines: []string{"GitHub sends webhooks", "to EventBridge"}}}
	got := SRT(cues)
	want := "1\n00:00:00,000 --> 00:00:02,500\nGitHub sends webhooks\nto EventBridge\n\n"
	if got != want {
		t.Errorf("SRT =\n%q\nwant\n%q", got, want)
	}
}

func TestASSWordPopBoxesEachWordVerbatim(t *testing.T) {
	words := tw("OpenClaw", "runs", "n8n")
	got := ASSWordPop(words, 1080, 1920)
	// One Dialogue event per spoken word, each on the pink box style.
	if n := strings.Count(got, "Dialogue: 0,"); n != len(words) {
		t.Errorf("want %d word events, got %d:\n%s", len(words), n, got)
	}
	for _, want := range []string{
		"PlayResX: 1080", "PlayResY: 1920",
		"Style: Pop,DejaVu Sans,120,", wordBoxColour, // pink/red box, fontsize=min/9
		",3,30,0,5,",     // BorderStyle=3 box, pad=30, centred
		`\pos(540,1382)`, // centred word position
		`\fscx82\fscy82\t(0,80,\fscx100\fscy100)`, // pop-in animation
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ASSWordPop missing %q:\n%s", want, got)
		}
	}
	// Protected terminology must stay verbatim (never uppercased/transcribed).
	for _, term := range []string{"OpenClaw", "n8n"} {
		if !strings.Contains(got, ","+term+"\n") && !strings.Contains(got, "}"+term+"\n") {
			t.Errorf("term %q not preserved verbatim as a word event:\n%s", term, got)
		}
	}
}

func TestASSWordPopGuardsZeroLengthCue(t *testing.T) {
	got := ASSWordPop([]TimedWord{{Word: "solo", StartMs: 1000, EndMs: 1000}}, 1080, 1920)
	if !strings.Contains(got, "0:00:01.00,0:00:01.20") {
		t.Errorf("zero-length cue not extended to a visible span:\n%s", got)
	}
}

func TestASSHasVerticalStyleAndTiming(t *testing.T) {
	got := ASS([]Cue{{StartMs: 0, EndMs: 2500, Lines: []string{"line one", "line two"}}}, 1080, 1920)
	for _, want := range []string{
		"PlayResX: 1080", "PlayResY: 1920",
		"Style: Default,Inter,54,&H00FFFFFF",
		"Dialogue: 0,0:00:00.00,0:00:02.50,Default,,0,0,0,,line one\\Nline two",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ASS missing %q:\n%s", want, got)
		}
	}
}
