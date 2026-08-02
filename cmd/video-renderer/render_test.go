package main

import (
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

func TestScenesForFormatFallsBackToStoryboard(t *testing.T) {
	storyboard := []byte(`{"scenes":[
		{"sceneNumber":1,"title":"S1","narration":"one"},
		{"sceneNumber":2,"title":"S2","narration":"two"},
		{"sceneNumber":3,"title":"S3","narration":"three"},
		{"sceneNumber":4,"title":"S4","narration":"four"},
		{"sceneNumber":5,"title":"S5","narration":"five"},
		{"sceneNumber":6,"title":"S6","narration":"six"}
	]}`)
	// No/blank format script → storyboard, capped for the short format.
	got, err := scenesForFormat("tiktok", nil, storyboard)
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if len(got) != shortSceneCap {
		t.Fatalf("expected storyboard capped to %d, got %d", shortSceneCap, len(got))
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

func TestSegmentAndConcatArgs(t *testing.T) {
	seg := segmentArgs("/w/cap.txt", "/w/n.mp3", "/w/s.mp4", 1080, 1920, "/font.ttf")
	if !slices.Contains(seg, "/w/n.mp3") || !slices.Contains(seg, "/w/s.mp4") {
		t.Errorf("segment args missing io: %v", seg)
	}
	joined := strings.Join(seg, " ")
	if !strings.Contains(joined, "color=c=0x0B0B12:s=1080x1920") || !strings.Contains(joined, "textfile=/w/cap.txt") || !strings.Contains(joined, "-shortest") {
		t.Errorf("segment args = %v", seg)
	}
	con := concatArgs("/w/list.txt", "/w/final.mp4")
	cj := strings.Join(con, " ")
	if !strings.Contains(cj, "-f concat") || !slices.Contains(con, "/w/final.mp4") {
		t.Errorf("concat args = %v", con)
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
