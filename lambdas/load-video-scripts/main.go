// Command load-video-scripts is the first state of the video state machine. It
// resolves, for each social-video format, the S3 URIs of the structured script
// artifacts the worker already generated (script + storyboard + voiceover JSON)
// and the target output URI in the video bucket, and returns a render job per
// format. It performs no AWS calls — the artifacts follow a deterministic key
// layout (see internal/artifactstore), so this is pure, testable construction.
package main

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
)

// input is the state-machine execution input the worker supplies.
type input struct {
	Owner  string `json:"owner"`
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	Bucket string `json:"bucket"` // content bucket the scripts live in
	Prefix string `json:"prefix"` // content key prefix (default "generated-content")
}

// job is one format's render inputs, consumed by the Fargate renderer via
// container environment overrides in the Parallel state.
type job struct {
	ScriptURI     string `json:"scriptUri"`
	StoryboardURI string `json:"storyboardUri"`
	VoiceoverURI  string `json:"voiceoverUri"`
	OutputURI     string `json:"outputUri"`
	Aspect        string `json:"aspect"`
}

// output is the state-machine payload: one job per format + the video bucket.
type output struct {
	YouTube     job    `json:"youtube"`
	Shorts      job    `json:"shorts"`
	TikTok      job    `json:"tiktok"`
	VideoBucket string `json:"videoBucket"`
}

func main() { lambda.Start(handle) }

func handle(_ context.Context, in input) (output, error) {
	return build(in, os.Getenv("VIDEO_BUCKET")), nil
}

// build resolves the render jobs for the three formats. The script stage names
// mirror the content-suite artifacts: youtube, youtube-shorts, tiktok, plus the
// shared storyboard and voiceover.
func build(in input, videoBucket string) output {
	prefix := in.Prefix
	if prefix == "" {
		prefix = "generated-content"
	}
	storyboard := artifactURI(in.Bucket, prefix, in.Owner, in.Name, in.Tag, "storyboard")
	voiceover := artifactURI(in.Bucket, prefix, in.Owner, in.Name, in.Tag, "voiceover")

	mk := func(stage, aspect string) job {
		return job{
			ScriptURI:     artifactURI(in.Bucket, prefix, in.Owner, in.Name, in.Tag, stage),
			StoryboardURI: storyboard,
			VoiceoverURI:  voiceover,
			OutputURI:     outputURI(videoBucket, in.Owner, in.Name, in.Tag, stage),
			Aspect:        aspect,
		}
	}
	return output{
		YouTube:     mk("youtube", "16:9"),
		Shorts:      mk("youtube-shorts", "9:16"),
		TikTok:      mk("tiktok", "9:16"),
		VideoBucket: videoBucket,
	}
}
