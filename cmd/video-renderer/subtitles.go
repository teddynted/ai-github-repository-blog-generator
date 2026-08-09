package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	pollytypes "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/subtitles"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/terminology"
)

// speechMarkTimes returns the start time (ms, relative to the clip) of each word
// Polly speaks for text, from WORD speech marks. This is timing only — never a
// transcription — so captions built from it are deterministic and script-driven.
func speechMarkTimes(ctx context.Context, p *polly.Client, voice, text string) ([]int, error) {
	out, err := p.SynthesizeSpeech(ctx, &polly.SynthesizeSpeechInput{
		Engine:          pollytypes.EngineNeural,
		OutputFormat:    pollytypes.OutputFormatJson,
		VoiceId:         pollytypes.VoiceId(voice),
		Text:            aws.String(text),
		SpeechMarkTypes: []pollytypes.SpeechMarkType{pollytypes.SpeechMarkTypeWord},
	})
	if err != nil {
		return nil, err
	}
	defer out.AudioStream.Close()
	raw, err := io.ReadAll(out.AudioStream)
	if err != nil {
		return nil, err
	}
	var times []int
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var m struct {
			Time int    `json:"time"`
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(line), &m) == nil && m.Type == "word" {
			times = append(times, m.Time)
		}
	}
	return times, nil
}

// sceneCaptionWords returns a scene's narration words timed for the FINAL video.
// The word VALUES come from the narration text (so terminology and punctuation
// are preserved verbatim); the TIMING comes from Polly word marks, or falls back
// to even spacing across the clip when marks and tokens don't line up 1:1.
func sceneCaptionWords(ctx context.Context, p *polly.Client, voice, narration string, offsetMs, durMs int) []subtitles.TimedWord {
	tokens := strings.Fields(narration)
	if len(tokens) == 0 || durMs <= 0 {
		return nil
	}
	times, err := speechMarkTimes(ctx, p, voice, narration)
	if err != nil || len(times) != len(tokens) {
		times = make([]int, len(tokens))
		for i := range tokens {
			times[i] = i * durMs / len(tokens)
		}
	}
	out := make([]subtitles.TimedWord, len(tokens))
	for i, w := range tokens {
		end := durMs
		if i+1 < len(times) {
			end = times[i+1]
		}
		out[i] = subtitles.TimedWord{Word: w, StartMs: offsetMs + times[i], EndMs: offsetMs + end}
	}
	return out
}

// writeSubtitles builds deterministic .srt and .ass caption files from the timed
// narration words and uploads them next to the final MP4 (sidecars — never burned
// into the frame). Best-effort: a failure is logged, not fatal to the render.
func writeSubtitles(ctx context.Context, s3c *s3.Client, words []subtitles.TimedWord, work, bucket, mp4Key string, w, h int) {
	if len(words) == 0 {
		return
	}
	cues := subtitles.Wrap(words, terminology.Base().Phrases())
	for name, body := range map[string]string{
		"subtitles.srt": subtitles.SRT(cues),
		"subtitles.ass": subtitles.ASS(cues, w, h),
	} {
		local := work + "/" + name
		if err := os.WriteFile(local, []byte(body), 0o644); err != nil {
			log.Printf("write %s failed: %v", name, err)
			continue
		}
		key := path.Join(path.Dir(mp4Key), name)
		if err := putObject(ctx, s3c, bucket, key, local); err != nil {
			log.Printf("upload %s failed: %v", name, err)
			continue
		}
		log.Printf("uploaded s3://%s/%s (%d cues)", bucket, key, len(cues))
	}
}
