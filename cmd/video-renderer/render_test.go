package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestParseScenesSkipsEmptyAndDefaults(t *testing.T) {
	js := []byte(`{"scenes":[
		{"sceneNumber":1,"title":"Intro","narration":"Welcome to the platform."},
		{"sceneNumber":2,"title":"","narration":"  "},
		{"title":"Wrap","narration":"That's a wrap."}
	]}`)
	got, err := parseScenes(js)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 narratable scenes, got %d: %+v", len(got), got)
	}
	if got[0].Title != "Intro" || got[0].Number != 1 {
		t.Errorf("scene0 = %+v", got[0])
	}
	// Third scene had no sceneNumber -> defaults to its index+1 (3); non-empty title.
	if got[1].Number != 3 || got[1].Title != "Wrap" {
		t.Errorf("scene1 = %+v", got[1])
	}
}

func TestParseScenesUnwrapsVersionedEnvelope(t *testing.T) {
	// The content suite writes artifacts wrapped in {"promptVersion","artifact"};
	// the renderer must unwrap to the inner storyboard, not see zero scenes.
	js := []byte(`{"promptVersion":"storyboard@1","artifact":{"scenes":[
		{"sceneNumber":1,"title":"Intro","narration":"Welcome."}
	]}}`)
	got, err := parseScenes(js)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Intro" {
		t.Fatalf("want the enveloped scene, got %+v", got)
	}
	// A bare storyboard (legacy, pre-envelope) must still parse.
	bare := []byte(`{"scenes":[{"sceneNumber":1,"title":"Bare","narration":"Hi."}]}`)
	if b, err := parseScenes(bare); err != nil || len(b) != 1 || b[0].Title != "Bare" {
		t.Fatalf("bare storyboard = %+v err=%v", b, err)
	}
}

func TestParseFormatScenesUnwrapsVersionedEnvelope(t *testing.T) {
	shorts := []byte(`{"promptVersion":"youtube-shorts@1","artifact":{"shorts":[
		{"id":1,"scenes":[{"number":1,"overlay":"HOOK","narration":"Quick hook."}]}
	]}}`)
	got := parseFormatScenes(shorts, "shorts")
	if len(got) != 1 || got[0].Title != "HOOK" {
		t.Fatalf("want the enveloped short scene, got %+v", got)
	}
}

func TestScenesForFormatUsesNativeScriptForShortFormats(t *testing.T) {
	storyboard := []byte(`{"scenes":[{"sceneNumber":1,"title":"Long","narration":"Long-form scene."}]}`)
	shorts := []byte(`{"shorts":[{"id":1,"title":"Short A","scenes":[
		{"number":1,"overlay":"HOOK","narration":"Quick hook."},
		{"number":2,"overlay":"POINT","narration":"The point."}
	]},{"id":2,"scenes":[{"number":1,"narration":"other short"}]}]}`)

	got, err := scenesForFormat("youtube-shorts", shorts, storyboard)
	if err != nil {
		t.Fatalf("shorts: %v", err)
	}
	// Uses the FIRST short's scenes with the overlay as the caption — not the storyboard.
	if len(got) != 2 || got[0].Title != "HOOK" || got[0].Narration != "Quick hook." {
		t.Fatalf("shorts scenes = %+v", got)
	}

	tiktok := []byte(`{"videos":[{"id":1,"scenes":[{"number":1,"overlay":"TT","narration":"Tik narration."}]}]}`)
	tt, err := scenesForFormat("tiktok", tiktok, storyboard)
	if err != nil || len(tt) != 1 || tt[0].Title != "TT" {
		t.Fatalf("tiktok scenes = %+v err=%v", tt, err)
	}
}

func TestScenesForFormatShortFormatIsDurationCapped(t *testing.T) {
	// 20 scenes with long narration; the short cut must trim each scene's
	// narration + caption and keep only enough scenes to land near the target.
	var parts []string
	for i := 1; i <= 20; i++ {
		parts = append(parts, fmt.Sprintf(`{"sceneNumber":%d,"title":"Scene %d: A Fairly Long Heading About Some Concept","narration":"This is a long narration sentence that keeps going with plenty of words. And here is a whole second sentence as well."}`, i, i))
	}
	storyboard := []byte(`{"scenes":[` + strings.Join(parts, ",") + `]}`)
	got, err := scenesForFormat("tiktok", nil, storyboard)
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if len(got) == 0 || len(got) >= 20 {
		t.Fatalf("short cut should be duration-capped, got %d of 20 scenes", len(got))
	}
	total := 0.0
	for _, s := range got {
		if wc := len(strings.Fields(s.Narration)); wc > shortMaxNarrationWords {
			t.Errorf("narration not trimmed (%d words): %q", wc, s.Narration)
		}
		if wc := len(strings.Fields(strings.TrimSuffix(s.Title, "…"))); wc > shortCaptionMaxWords {
			t.Errorf("caption not shortened: %q", s.Title)
		}
		total += float64(len(strings.Fields(s.Narration)))/shortWordsPerSec + shortScenePadSec
	}
	if total > shortTargetSec+shortScenePadSec {
		t.Errorf("estimated runtime %.1fs exceeds target %.0fs", total, shortTargetSec)
	}
}

func TestScenesForFormatYouTubeUsesStoryboard(t *testing.T) {
	storyboard := []byte(`{"scenes":[{"sceneNumber":1,"title":"Intro","narration":"n"}]}`)
	// A youtube script must be ignored in favour of the storyboard it references.
	got, err := scenesForFormat("youtube", []byte(`{"chapters":[]}`), storyboard)
	if err != nil || len(got) != 1 || got[0].Title != "Intro" {
		t.Fatalf("youtube scenes = %+v err=%v", got, err)
	}
}

func TestCapScenes(t *testing.T) {
	in := []scene{{Number: 1}, {Number: 2}, {Number: 3}}
	if len(capScenes(in, 2)) != 2 {
		t.Error("expected cap to 2")
	}
	if len(capScenes(in, 0)) != 3 {
		t.Error("n<=0 keeps all")
	}
	if len(capScenes(in, 10)) != 3 {
		t.Error("n>len keeps all")
	}
}

func TestDimsFor(t *testing.T) {
	if w, h := dimsFor("16:9"); w != 1920 || h != 1080 {
		t.Errorf("16:9 = %dx%d", w, h)
	}
	// Anything non-16:9 is vertical.
	for _, a := range []string{"9:16", "", "4:5"} {
		if w, h := dimsFor(a); w != 1080 || h != 1920 {
			t.Errorf("%q = %dx%d, want 1080x1920", a, w, h)
		}
	}
}

func TestSegmentArgsColorAndImageBackground(t *testing.T) {
	// No background image → solid colour source.
	seg := segmentArgs("/w/cap.txt", "/w/n.mp3", "/w/s.mp4", 1080, 1920, "/font.ttf", "", "", 0, false)
	joined := strings.Join(seg, " ")
	if !strings.Contains(joined, "color=c=0x0F172A:s=1080x1920") || !strings.Contains(joined, "textfile=/w/cap.txt") || !strings.Contains(joined, "-shortest") {
		t.Errorf("colour segment args = %v", seg)
	}
	// Title card: large, centred on both axes.
	if !strings.Contains(joined, "x=(w-text_w)/2:y=(h-text_h)/2") {
		t.Errorf("title not centred: %v", seg)
	}
	if !slices.Contains(seg, "/w/n.mp3") || !slices.Contains(seg, "/w/s.mp4") {
		t.Errorf("segment args missing io: %v", seg)
	}

	// With a scene image background → loop the image, cover-crop it, then caption.
	dseg := segmentArgs("/w/cap.txt", "/w/n.mp3", "/w/s.mp4", 1920, 1080, "/font.ttf", "/w/scene_bg.png", "", 0, false)
	dj := strings.Join(dseg, " ")
	if !slices.Contains(dseg, "/w/scene_bg.png") || strings.Contains(dj, "color=c=") {
		t.Errorf("image bg not used: %v", dseg)
	}
	if !strings.Contains(dj, "scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080") {
		t.Errorf("image cover-crop filter missing: %v", dseg)
	}
}

func TestSegmentArgsCameraMotion(t *testing.T) {
	// A move on an image scene adds a zoompan Ken Burns chain outputting the frame.
	seg := segmentArgs("/w/cap.txt", "/w/n.mp3", "/w/s.mp4", 1080, 1920, "/font.ttf", "/w/bg.png", "dolly_in", 4.0, false)
	j := strings.Join(seg, " ")
	if !strings.Contains(j, "zoompan=z='min(zoom+0.0010,1.12)'") || !strings.Contains(j, "s=1080x1920") {
		t.Errorf("dolly_in zoompan missing: %v", seg)
	}
	// Empty move → static cover crop, no zoompan.
	stat := strings.Join(segmentArgs("/w/cap.txt", "/w/n.mp3", "/w/s.mp4", 1080, 1920, "/font.ttf", "/w/bg.png", "", 4.0, false), " ")
	if strings.Contains(stat, "zoompan") {
		t.Errorf("no-move segment should not zoompan: %s", stat)
	}
}

func TestSegmentArgsAnimatedDiagram(t *testing.T) {
	// An animated diagram is a short looping video with NO caption overlaid, bounded
	// to the narration by an explicit -t (so the looped video never overshoots into
	// trailing silence), plus -shortest as a backstop.
	seg := segmentArgs("/w/cap.txt", "/w/n.mp3", "/w/s.mp4", 1080, 1920, "/font.ttf", "/w/scene_1_diagram.mp4", "", 5.0, true)
	j := strings.Join(seg, " ")
	if !strings.Contains(j, "-stream_loop -1 -i /w/scene_1_diagram.mp4") {
		t.Errorf("animated diagram not stream-looped: %v", seg)
	}
	if !strings.Contains(j, "-i /w/n.mp3 -map 0:v -map 1:a") {
		t.Errorf("animated diagram must map video+audio explicitly: %v", seg)
	}
	if strings.Contains(j, "zoompan") || strings.Contains(j, "force_original_aspect_ratio") {
		t.Errorf("animated diagram should not zoompan/cover-crop (motion is baked in): %s", j)
	}
	// No caption is drawn on a diagram.
	if strings.Contains(j, "drawtext") || strings.Contains(j, "textfile") || strings.Contains(j, "-vf") {
		t.Errorf("animated diagram must not overlay a caption: %s", j)
	}
	// BOTH streams forced to exactly durSec (apad + constant fps + shared -t), never
	// -shortest, so concatenated segments stay in sync.
	if !strings.Contains(j, "-af apad -r 30 -t 5.000") || strings.Contains(j, "-shortest") {
		t.Errorf("animated diagram must be apad+fps+-t bounded, no -shortest: %v", seg)
	}
}

func TestSegmentArgsBothStreamsBounded(t *testing.T) {
	// durSec>0 → apad + -r 30 + -t (both streams exactly durSec), no -shortest.
	withDur := strings.Join(segmentArgs("/w/c.txt", "/w/n.mp3", "/w/s.mp4", 1080, 1920, "/f.ttf", "", "", 4.0, false), " ")
	if strings.Contains(withDur, "-shortest") || !strings.Contains(withDur, "-af apad -r 30 -t 4.000") {
		t.Errorf("card with known duration must be apad+fps+-t bounded: %s", withDur)
	}
	// durSec<=0 → -shortest fallback (duration unknown), no apad (would run forever).
	noDur := strings.Join(segmentArgs("/w/c.txt", "/w/n.mp3", "/w/s.mp4", 1080, 1920, "/f.ttf", "", "", 0, false), " ")
	if !strings.Contains(noDur, "-shortest") || strings.Contains(noDur, "apad") {
		t.Errorf("card with unknown duration must fall back to -shortest, no apad: %s", noDur)
	}
}

func TestShortCaptionAndTrimNarration(t *testing.T) {
	if c := shortCaption("Ollama First, Bedrock When It Isn't: Two Inference Engines, One Contract"); len(strings.Fields(strings.TrimSuffix(c, "…"))) > shortCaptionMaxWords {
		t.Errorf("caption too long: %q", c)
	}
	// Prefers the punchy segment before a colon when it's short enough.
	if c := shortCaption("Ollama → Bedrock: the fallback that never drops a request"); c != "Ollama → Bedrock" {
		t.Errorf("caption = %q, want the pre-colon segment", c)
	}
	// Captions are COMPLETE — never a truncated "…", even for a long unbroken title.
	for _, title := range []string{
		"Turning A Long Unbroken Heading Into A Complete Caption Here Now",
		"How A GitHub Webhook Becomes An Automated Agent Run End To End",
	} {
		if c := shortCaption(title); strings.Contains(c, "…") {
			t.Errorf("caption must not be truncated with an ellipsis: %q", c)
		}
	}
	n := trimNarration("This is the first sentence and it runs on with a great many words indeed. Second one here.", shortMaxNarrationWords)
	if wc := len(strings.Fields(n)); wc > shortMaxNarrationWords {
		t.Errorf("narration not trimmed: %d words %q", wc, n)
	}
	if strings.Contains(n, "Second one") {
		t.Errorf("narration should keep only the first sentence: %q", n)
	}
}

func TestTitleCaptionNotWordCapped(t *testing.T) {
	full := "Designing a Resilient AI Agent Platform on AWS: Self-Hosted Inference with a Managed Fallback"
	// The title card keeps the complete main title (before the colon) — never the
	// 7-word content-caption cap that clipped it to "…Platform on".
	if c := titleCaption(full); c != "Designing a Resilient AI Agent Platform on AWS" {
		t.Errorf("titleCaption = %q, want the complete main title", c)
	}
	// A title with no separator is kept whole, not word-capped.
	whole := "One Release Becomes Blogs Videos And Social Posts Automatically"
	if c := titleCaption(whole); c != whole {
		t.Errorf("titleCaption should keep a separator-less title whole: %q", c)
	}
	// optimizeForShort must route a title scene through titleCaption, not shortCaption.
	out := optimizeForShort([]scene{
		{Number: 1, Type: "title", Title: full, Narration: "Here is the opening hook line."},
		{Number: 2, Type: "architecture", Title: "Why OpenClaw Reaches for Ollama Before Amazon Bedrock", Narration: "OpenClaw runs Ollama first."},
	})
	if len(out) == 0 || out[0].Title != "Designing a Resilient AI Agent Platform on AWS" {
		t.Errorf("title scene caption = %q, want the complete title", out[0].Title)
	}
	if strings.HasSuffix(out[0].Title, " on") {
		t.Errorf("title scene must not be clipped mid-phrase: %q", out[0].Title)
	}
}

func TestForceRender(t *testing.T) {
	// Default (unset): the already-exists skip is in effect.
	t.Setenv("FORCE_RENDER", "")
	if forceRender() {
		t.Error("FORCE_RENDER unset should not force a render")
	}
	for _, v := range []string{"true", "TRUE", "True"} {
		t.Setenv("FORCE_RENDER", v)
		if !forceRender() {
			t.Errorf("FORCE_RENDER=%q should force a render", v)
		}
	}
	t.Setenv("FORCE_RENDER", "1")
	if forceRender() {
		t.Error(`only "true" (case-insensitive) forces a render, not "1"`)
	}
}

func TestConcatArgs(t *testing.T) {
	con := concatArgs("/w/list.txt", "/w/final.mp4", "")
	cj := strings.Join(con, " ")
	if !strings.Contains(cj, "-f concat") || !slices.Contains(con, "/w/final.mp4") {
		t.Errorf("concat args = %v", con)
	}
	if strings.Contains(cj, "-vf") {
		t.Errorf("no assFile should mean no burn-in filter: %v", con)
	}
	// With an assFile the one-word captions are burned in during the same encode.
	burn := strings.Join(concatArgs("/w/list.txt", "/w/final.mp4", "/w/words.ass"), " ")
	if !strings.Contains(burn, "-vf ass=/w/words.ass") {
		t.Errorf("expected ass burn-in filter, got %q", burn)
	}
}

func TestVisualTextReadsBothShapes(t *testing.T) {
	// Short-format "visual" string.
	if got := visualText(rawScene{Visual: "Show the CLI command."}); got != "Show the CLI command." {
		t.Errorf("short-format visual = %q", got)
	}
	// Storyboard "visuals" object with a description (the YouTube path).
	sb := rawScene{Visuals: json.RawMessage(`{"description":"An isometric render of services exchanging events."}`)}
	if got := visualText(sb); got != "An isometric render of services exchanging events." {
		t.Errorf("storyboard visuals = %q", got)
	}
	// Tolerant of a bare string / array; empty when absent.
	if got := visualText(rawScene{Visuals: json.RawMessage(`"just a string"`)}); got != "just a string" {
		t.Errorf("string visuals = %q", got)
	}
	if got := visualText(rawScene{}); got != "" {
		t.Errorf("no direction should be empty, got %q", got)
	}
}

func TestParseS3URI(t *testing.T) {
	b, k, err := parseS3URI("s3://bucket/a/b/c.json")
	if err != nil || b != "bucket" || k != "a/b/c.json" {
		t.Fatalf("valid: b=%q k=%q err=%v", b, k, err)
	}
	for _, bad := range []string{"https://x/y", "s3://only-bucket", "s3://", ""} {
		if _, _, err := parseS3URI(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestOverviewKeysHeroSpine(t *testing.T) {
	// Services collected across scenes → flow-ordered hero spine, capped, OpenClaw
	// as the orchestrator between the workflow and the inference backends.
	scenes := []scene{
		{Type: "title", Title: "Designing a Resilient AI Agent Platform"},
		{Type: "architecture", Title: "Why OpenClaw Reaches for Ollama Before Amazon Bedrock"},
		{Type: "architecture", Title: "From a GitHub Webhook Through EventBridge and Lambda"},
		{Type: "architecture", Title: "The Shared EFS Workspace Where OpenClaw and n8n Meet"},
		{Type: "architecture", Title: "What OpenClaw and Lambda Stream to CloudWatch"},
	}
	got := overviewKeys(scenes)
	if len(got) == 0 || len(got) > overviewMaxNodes {
		t.Fatalf("overview keys = %v", got)
	}
	// Flow order: github before openclaw before ollama; openclaw present as hub.
	pos := map[string]int{}
	for i, k := range got {
		pos[k] = i
	}
	if _, ok := pos["openclaw"]; !ok {
		t.Errorf("overview must include OpenClaw hub: %v", got)
	}
	if pos["github"] > pos["openclaw"] || pos["openclaw"] > pos["ollama"] {
		t.Errorf("overview not in flow order (github<openclaw<ollama): %v", got)
	}
}

func TestNeighborsFromReleaseGraph(t *testing.T) {
	// Neighbors are derived from co-occurrence in the release's own scenes — no
	// hardcoded per-repo relationships. A lone component expands into its
	// flow-downstream neighbors (its outputs), closest-in-flow first.
	scenes := []scene{
		{Title: "Why OpenClaw Reaches for Ollama Before Amazon Bedrock", Narration: "OpenClaw runs Ollama first, then Amazon Bedrock."},
		{Title: "The Shared EFS Workspace Where OpenClaw and n8n Meet", Narration: "OpenClaw and n8n share EFS."},
		{Title: "What OpenClaw and Lambda Stream to CloudWatch", Narration: "OpenClaw and Lambda log to CloudWatch."},
	}
	adj := coOccurrence(scenes)
	if got := strings.Join(neighborsFor("openclaw", adj), ","); got != "bedrock,ollama" {
		t.Errorf("neighborsFor(openclaw) = %q, want bedrock,ollama (deterministic)", got)
	}
	// A component with no co-occurrence (nothing to connect to) → no neighbors → card.
	if n := neighborsFor("s3", map[string]map[string]bool{}); n != nil {
		t.Errorf("expected nil neighbors, got %v", n)
	}
}
