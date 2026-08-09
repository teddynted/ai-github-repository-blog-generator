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
