package main

import "testing"

func TestBuildResolvesJobsForAllThreeFormats(t *testing.T) {
	out := build(input{
		Owner:  "teddynted",
		Name:   "designing-an-ai-agent-platform-on-aws",
		Tag:    "v0.6.0",
		Bucket: "blog-gen-content-123-us-east-1",
		Prefix: "generated-content",
	}, "blog-gen-video-123-us-east-1")

	if out.VideoBucket != "blog-gen-video-123-us-east-1" {
		t.Fatalf("videoBucket = %q", out.VideoBucket)
	}

	// The three formats map to the youtube / youtube-shorts / tiktok artifacts;
	// the shared storyboard + voiceover are the same for every format.
	wantScript := "s3://blog-gen-content-123-us-east-1/generated-content/teddynted/designing-an-ai-agent-platform-on-aws/releases/v0.6.0/.artifacts/youtube.json"
	if out.YouTube.ScriptURI != wantScript {
		t.Errorf("youtube script uri = %q", out.YouTube.ScriptURI)
	}
	if out.Shorts.ScriptURI == "" || out.Shorts.ScriptURI == out.YouTube.ScriptURI {
		t.Errorf("shorts script uri should differ: %q", out.Shorts.ScriptURI)
	}
	wantOut := "s3://blog-gen-video-123-us-east-1/teddynted/designing-an-ai-agent-platform-on-aws/v0.6.0/tiktok/final-tiktok.mp4"
	if out.TikTok.OutputURI != wantOut {
		t.Errorf("tiktok output uri = %q", out.TikTok.OutputURI)
	}
	if out.YouTube.StoryboardURI != out.TikTok.StoryboardURI {
		t.Error("storyboard uri must be shared across formats")
	}
	if out.YouTube.Aspect != "16:9" || out.Shorts.Aspect != "9:16" || out.TikTok.Aspect != "9:16" {
		t.Errorf("aspects = %s/%s/%s", out.YouTube.Aspect, out.Shorts.Aspect, out.TikTok.Aspect)
	}
}

func TestBuildDefaultsPrefixAndSanitizes(t *testing.T) {
	out := build(input{Owner: "Acme Corp", Name: "widget", Tag: "v1.0.0", Bucket: "b"}, "vb")
	// Blank prefix -> "generated-content"; "Acme Corp" -> "Acme-Corp".
	want := "s3://b/generated-content/Acme-Corp/widget/releases/v1.0.0/.artifacts/youtube.json"
	if out.YouTube.ScriptURI != want {
		t.Errorf("uri = %q, want %q", out.YouTube.ScriptURI, want)
	}
}
