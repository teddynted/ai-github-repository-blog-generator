package voiceover

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
)

// sampleStoryboard builds a small but representative storyboard covering an
// intro, an architecture scene (with a diagram), an implementation scene (with
// code), and a conclusion — enough to exercise every planner.
func sampleStoryboard() storyboard.Storyboard {
	scenes := []storyboard.Scene{
		{
			SceneNumber: 1, Title: "Introduction", Type: "introduction",
			Narration: "Welcome. This release ships a Release Context builder for GitHub. Notice how it grounds every downstream artifact.",
			Duration:  storyboard.Duration{RecommendedSec: 12, MinSec: 9, MaxSec: 17, Pacing: "medium"},
			Camera:    storyboard.Camera{Direction: "Slow Zoom In"},
			Overlays:  []storyboard.Overlay{{Kind: "Title", Text: "widget v0.2.0"}},
		},
		{
			SceneNumber: 2, Title: "Architecture and Design", Type: "architecture",
			Narration: "The system is event-driven on AWS Lambda and Amazon SQS. CloudFormation provisions every resource.",
			Duration:  storyboard.Duration{RecommendedSec: 15, MinSec: 12, MaxSec: 20, Pacing: "medium"},
			Camera:    storyboard.Camera{Direction: "Diagram Focus"},
			Diagrams:  []storyboard.DiagramRef{{Source: "docs/architecture.md", Type: "flowchart", HighlightNodes: []string{"A", "B"}}},
		},
		{
			SceneNumber: 3, Title: "Implementation Details", Type: "implementation",
			Narration: "The builder is a pure Go package behind a Sources port. The important point is that it stays testable.",
			Duration:  storyboard.Duration{RecommendedSec: 14, MinSec: 11, MaxSec: 19, Pacing: "medium"},
			Camera:    storyboard.Camera{Direction: "Push In"},
			Code:      []storyboard.CodeRef{{Language: "go", Instruction: "Highlight function"}},
		},
		{
			SceneNumber: 4, Title: "Conclusion", Type: "conclusion",
			Narration: "That's widget v0.2.0. Try it on your own repository today.",
			Duration:  storyboard.Duration{RecommendedSec: 8, MinSec: 5, MaxSec: 13, Pacing: "fast"},
			Camera:    storyboard.Camera{Direction: "Slow Zoom Out"},
		},
	}
	total := 0
	for _, s := range scenes {
		total += s.Duration.RecommendedSec
	}
	return storyboard.Storyboard{
		SchemaVersion: storyboard.SchemaVersion,
		Metadata:      storyboard.Metadata{Repository: "acme/widget", Release: "v0.2.0", SourceBlogTitle: "Inside widget v0.2.0"},
		Video:         storyboard.VideoSpec{SceneCount: len(scenes), TotalDurationSec: total},
		Scenes:        scenes,
	}
}

func newGen() *Generator {
	return &Generator{Now: func() time.Time { return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC) }}
}

func TestVoiceOverEndToEnd(t *testing.T) {
	sb := sampleStoryboard()
	script, err := newGen().VoiceOver(context.Background(), sb)
	if err != nil {
		t.Fatalf("VoiceOver: %v", err)
	}

	if script.SchemaVersion != SchemaVersion || script.Metadata.Repository != "acme/widget" {
		t.Errorf("envelope = %+v", script.Metadata)
	}
	if len(script.Scenes) != 4 || script.Metadata.SceneCount != 4 {
		t.Fatalf("scenes = %d", len(script.Scenes))
	}

	// Narration is carried over verbatim from the storyboard (Model == nil).
	if script.Scenes[0].Narration != collapse(sb.Scenes[0].Narration) {
		t.Errorf("narration should be storyboard verbatim, got %q", script.Scenes[0].Narration)
	}

	// Every scene: narration + timing + voice direction + transition set.
	for i, sc := range script.Scenes {
		if sc.Narration == "" || sc.VoiceDirection == "" || sc.Transition == "" {
			t.Errorf("scene %d under-populated: %+v", i+1, sc)
		}
		if sc.Duration.AllocatedSec == 0 {
			t.Errorf("scene %d missing timing", i+1)
		}
	}

	// Structural validation passes.
	if probs := script.Validate(); len(probs) != 0 {
		t.Errorf("validation problems: %v", probs)
	}

	// JSON round-trips (it is the canonical downstream input).
	blob, err := json.Marshal(script)
	if err != nil || !strings.Contains(string(blob), "\"schemaVersion\":\"1.0.0\"") {
		t.Errorf("marshal: %v", err)
	}

	// Content intelligence populated.
	if script.ContentIntelligence.TotalWords == 0 || script.ContentIntelligence.EstimatedSpeakingTime == "" {
		t.Errorf("intelligence = %+v", script.ContentIntelligence)
	}
}

func TestTimelineSequentialAndGapFree(t *testing.T) {
	script, _ := newGen().VoiceOver(context.Background(), sampleStoryboard())
	prevEnd := 0
	for i, sc := range script.Scenes {
		if sc.Timestamp.StartSec != prevEnd {
			t.Errorf("scene %d start %d not contiguous with prev end %d", i+1, sc.Timestamp.StartSec, prevEnd)
		}
		if sc.Timestamp.EndSec <= sc.Timestamp.StartSec {
			t.Errorf("scene %d end %d not after start %d", i+1, sc.Timestamp.EndSec, sc.Timestamp.StartSec)
		}
		prevEnd = sc.Timestamp.EndSec
	}
	// First timestamp label is zero-padded MM:SS.
	if script.Scenes[0].Timestamp.Label != "00:00–00:12" {
		t.Errorf("first label = %q, want 00:00–00:12", script.Scenes[0].Timestamp.Label)
	}
}

func TestClockFormatsMinutes(t *testing.T) {
	cases := map[int]string{0: "00:00", 9: "00:09", 65: "01:05", 605: "10:05"}
	for sec, want := range cases {
		if got := clock(sec); got != want {
			t.Errorf("clock(%d) = %q, want %q", sec, got, want)
		}
	}
}

func TestSpeechSecondsAndFit(t *testing.T) {
	// 26 words at 156 wpm (2.6 wps) ≈ 10s.
	if got := speechSeconds(26, 156); got != 10 {
		t.Errorf("speechSeconds(26) = %d, want 10", got)
	}
	// Fits when estimate <= allocated + tolerance.
	d := planDuration(strings.Repeat("word ", 26), 12, 156)
	if !d.Fits {
		t.Errorf("expected fit: %+v", d)
	}
	// A long narration in a short slot does not fit.
	d = planDuration(strings.Repeat("word ", 200), 8, 156)
	if d.Fits {
		t.Errorf("expected not-fit: %+v", d)
	}
}

func TestPronunciationGroundedAndUnique(t *testing.T) {
	// Multi-word term wins over its sub-token, and only present terms appear.
	p := planPronunciation("We call the API Gateway and store JSON in DynamoDB.")
	terms := map[string]bool{}
	for _, e := range p {
		if terms[e.Term] {
			t.Errorf("duplicate term %q", e.Term)
		}
		terms[e.Term] = true
	}
	if !terms["API Gateway"] || !terms["JSON"] || !terms["DynamoDB"] {
		t.Errorf("expected API Gateway, JSON, DynamoDB; got %v", terms)
	}
	// "API" alone must be suppressed in favour of "API Gateway".
	if terms["API"] {
		t.Errorf("sub-token API should be suppressed by API Gateway: %v", terms)
	}
	// A term that is absent must never appear.
	if terms["Kubernetes"] {
		t.Errorf("Kubernetes not in text but was emitted")
	}
	// Acronyms carry a spell-out say-as hint.
	for _, e := range p {
		if e.Term == "JSON" && e.SayAs == "" {
			t.Errorf("JSON missing say-as hint")
		}
	}
}

func TestPronunciationEmptyText(t *testing.T) {
	if p := planPronunciation("Just some plain prose with no jargon at all."); len(p) != 0 {
		t.Errorf("expected no pronunciation entries, got %+v", p)
	}
}

func TestEmphasisSurfacesCuesAndTag(t *testing.T) {
	e := planEmphasis("Notice how the important point lands. Ship v0.2.0 today.", "v0.2.0")
	joined := strings.Join(e, "|")
	if !strings.Contains(joined, "notice") || !strings.Contains(joined, "the important point") {
		t.Errorf("expected cue phrases, got %v", e)
	}
	if !strings.Contains(joined, "v0.2.0") {
		t.Errorf("expected release tag emphasized, got %v", e)
	}
	// Absent tag is not emphasized.
	if e := planEmphasis("Plain narration.", "v9.9.9"); len(e) != 0 {
		t.Errorf("expected no emphasis, got %v", e)
	}
}

func TestPausesPurposeful(t *testing.T) {
	p := planPauses(pauseInput{SceneType: "architecture", Narration: "First sentence. Second sentence.", HasDiagram: true, HasCode: false})
	positions := map[string]bool{}
	for _, x := range p {
		positions[x.Position] = true
	}
	for _, want := range []string{"opening", "before-diagram", "closing"} {
		if !positions[want] {
			t.Errorf("missing pause position %q in %v", want, positions)
		}
	}
	// A results scene gets an after-key-takeaway long pause.
	p = planPauses(pauseInput{SceneType: "results", Narration: "It works.", HasDiagram: false, HasCode: false})
	var hasLong bool
	for _, x := range p {
		if x.Position == "after-key-takeaway" && x.Type == "long" {
			hasLong = true
		}
	}
	if !hasLong {
		t.Errorf("results scene should have a long after-key-takeaway pause: %+v", p)
	}
	// Positions are unique.
	seen := map[string]bool{}
	for _, x := range p {
		if seen[x.Position] {
			t.Errorf("duplicate pause position %q", x.Position)
		}
		seen[x.Position] = true
	}
}

func TestSyncCuesFromStoryboard(t *testing.T) {
	sc := storyboard.Scene{
		Camera:   storyboard.Camera{Direction: "Diagram Focus"},
		Diagrams: []storyboard.DiagramRef{{Source: "docs/architecture.md"}},
		Code:     []storyboard.CodeRef{{Language: "go"}},
	}
	cues := planSyncCues(sc)
	kinds := map[string]bool{}
	for _, c := range cues {
		kinds[c.Visual] = true
	}
	for _, want := range []string{"Diagram Reveal", "Code Highlight", "Camera Move"} {
		if !kinds[want] {
			t.Errorf("missing sync cue %q in %v", want, kinds)
		}
	}
	// A Static camera contributes no camera cue.
	if cues := planSyncCues(storyboard.Scene{Camera: storyboard.Camera{Direction: "Static"}}); len(cues) != 0 {
		t.Errorf("static camera should yield no cues, got %+v", cues)
	}
}

func TestTransitionsBridgeAndClose(t *testing.T) {
	// Non-last scene bridges to the next by type.
	got := planTransition("architecture", "implementation", "Implementation Details", false)
	if !strings.Contains(strings.ToLower(got), "architecture") || !strings.Contains(strings.ToLower(got), "built") {
		t.Errorf("bridge = %q", got)
	}
	// Last scene gets a closing line, not a bridge.
	last := planTransition("conclusion", "", "", true)
	if !strings.Contains(strings.ToLower(last), "thanks for watching") {
		t.Errorf("closing = %q", last)
	}
}

func TestVoiceDirectionByType(t *testing.T) {
	if d := directionFor("introduction"); d.Energy != "high" {
		t.Errorf("intro energy = %q, want high", d.Energy)
	}
	if d := directionFor("implementation"); d.Pace != "Slow" {
		t.Errorf("implementation pace = %q, want Slow", d.Pace)
	}
	if d := directionFor("totally-unknown"); d.Direction == "" || d.Energy != "medium" {
		t.Errorf("unknown type should fall back: %+v", d)
	}
}

func TestIntelligenceMetadata(t *testing.T) {
	script, _ := newGen().VoiceOver(context.Background(), sampleStoryboard())
	ci := script.ContentIntelligence
	if ci.AverageWordsPerMinute != defaultWordsPerMinute {
		t.Errorf("wpm = %d, want %d", ci.AverageWordsPerMinute, defaultWordsPerMinute)
	}
	if ci.RecommendedLanguage != "en-US" {
		t.Errorf("language = %q", ci.RecommendedLanguage)
	}
	if ci.UniquePronunciations == 0 {
		t.Errorf("expected some unique pronunciations across scenes")
	}
	if !strings.Contains(ci.EstimatedSpeakingTime, ":") {
		t.Errorf("speaking time not M:SS: %q", ci.EstimatedSpeakingTime)
	}
}

func TestMarkdownRenders(t *testing.T) {
	script, _ := newGen().VoiceOver(context.Background(), sampleStoryboard())
	md := script.Markdown()
	for _, want := range []string{
		"# Voice-over Script", "## Scene 1", "**Timestamp:**", "**Voice:**",
		"**Pronunciation:**", "**Transition:**", "## Production Intelligence",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

type fakeModel struct{ calls int }

func (f *fakeModel) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return "Refined engineer-to-engineer narration for this scene.", nil
}

func TestModelPolishesNarration(t *testing.T) {
	fm := &fakeModel{}
	g := &Generator{Model: fm, Now: newGen().Now}
	script, err := g.VoiceOver(context.Background(), sampleStoryboard())
	if err != nil {
		t.Fatal(err)
	}
	if fm.calls != len(script.Scenes) {
		t.Errorf("model called %d times, want %d (one per scene)", fm.calls, len(script.Scenes))
	}
	if script.Scenes[0].Narration != "Refined engineer-to-engineer narration for this scene." {
		t.Errorf("narration not taken from model: %q", script.Scenes[0].Narration)
	}
}

type errModel struct{}

func (errModel) Generate(_ context.Context, _ string) (string, error) {
	return "", context.DeadlineExceeded
}

func TestModelErrorFallsBackToStoryboard(t *testing.T) {
	g := &Generator{Model: errModel{}, Now: newGen().Now}
	script, err := g.VoiceOver(context.Background(), sampleStoryboard())
	if err != nil {
		t.Fatal(err)
	}
	if script.Scenes[0].Narration != collapse(sampleStoryboard().Scenes[0].Narration) {
		t.Errorf("expected fallback to storyboard narration, got %q", script.Scenes[0].Narration)
	}
}

func TestRequiresScenes(t *testing.T) {
	if _, err := newGen().VoiceOver(context.Background(), storyboard.Storyboard{}); err == nil {
		t.Error("expected error for storyboard with no scenes")
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	// Empty narration, bad timing, non-contiguous timestamps, missing transition.
	bad := VoiceOverScript{
		SchemaVersion: "1.0.0",
		Metadata:      Metadata{SceneCount: 1},
		Scenes: []Scene{{
			SceneNumber: 1, Title: "x", Narration: "  ",
			Duration:  Duration{AllocatedSec: 0, Fits: false},
			Timestamp: Timestamp{StartSec: 5, EndSec: 3},
		}},
	}
	probs := bad.Validate()
	if len(probs) == 0 {
		t.Fatal("expected validation problems")
	}
	joined := strings.Join(probs, "\n")
	for _, want := range []string{"empty narration", "missing timing", "missing transition"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected problem %q in:\n%s", want, joined)
		}
	}
}

func TestValidateCatchesDuplicatePronunciation(t *testing.T) {
	script, _ := newGen().VoiceOver(context.Background(), sampleStoryboard())
	// Inject a duplicate pronunciation entry into a valid script.
	script.Scenes[0].Pronunciation = append(script.Scenes[0].Pronunciation,
		Pronunciation{Term: "JSON"}, Pronunciation{Term: "JSON"})
	probs := script.Validate()
	if !strings.Contains(strings.Join(probs, "\n"), "duplicate pronunciation") {
		t.Errorf("expected duplicate pronunciation problem, got %v", probs)
	}
}
