package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/imagegen"
)

// imageGenerator is the minimal image port the renderer needs, so scene-image
// generation is testable with a fake (the real *imagegen.Client satisfies it).
type imageGenerator interface {
	Generate(ctx context.Context, spec imagegen.Spec) ([]byte, error)
}

// sceneImagesEnabled reports whether per-scene AI background generation is on.
// It is off unless ENABLE_SCENE_IMAGES=true, so the working pipeline is never
// destabilised by image-model throttling — a miss always falls back to a card.
func sceneImagesEnabled() bool { return strings.EqualFold(os.Getenv("ENABLE_SCENE_IMAGES"), "true") }

// novaDims returns Nova Canvas-safe generation dimensions for the frame aspect
// (each a multiple of 16, within the pixel budget). ffmpeg later scales the
// result up to the actual frame, so a matching aspect is all that matters.
func novaDims(w, h int) (int, int) {
	if w >= h {
		return 1280, 720 // 16:9
	}
	return 720, 1280 // 9:16
}

// sceneImagePrompt builds a grounded text-to-image prompt for a scene: a
// consistent editorial style plus the scene's own title and a little narration
// context, so the background illustrates the scene. Text is explicitly excluded
// (the renderer burns the caption in itself).
func sceneImagePrompt(sc scene) (prompt, negative string) {
	topic := strings.TrimSpace(sc.Title)
	context := truncateWords(strings.TrimSpace(sc.Narration), 40)
	prompt = "Editorial flat-vector isometric illustration for a software architecture explainer video. " +
		"Subject: " + topic + ". " + context + " " +
		"Deep slate background, teal and amber accents, clean geometric shapes, subtle grid, cinematic depth. No text, no words, no letters, no logos."
	negative = "text, words, letters, captions, watermark, logo, ui, frame, border, low quality, blurry"
	return prompt, negative
}

// maybeSceneImage best-effort generates a background image for a scene and
// writes it to the work dir, returning its path. Any failure (disabled, init
// error, throttled-out, write error) returns "" so the caller falls back to the
// title card. The scene number seeds generation so a re-render is stable.
func maybeSceneImage(ctx context.Context, gen imageGenerator, sc scene, work string, w, h int) string {
	if gen == nil {
		return ""
	}
	prompt, negative := sceneImagePrompt(sc)
	gw, gh := novaDims(w, h)
	png, err := gen.Generate(ctx, imagegen.Spec{Prompt: prompt, NegativePrompt: negative, Width: gw, Height: gh, Seed: sc.Number})
	if err != nil {
		log.Printf("scene %d image unavailable, using title card: %v", sc.Number, err)
		return ""
	}
	path := filepath.Join(work, fmt.Sprintf("scene_%d_bg.png", sc.Number))
	if err := os.WriteFile(path, png, 0o644); err != nil {
		log.Printf("scene %d image write failed, using title card: %v", sc.Number, err)
		return ""
	}
	log.Printf("scene %d: generated background image", sc.Number)
	return path
}

// truncateWords keeps the first n whitespace-separated words of s, so a long
// narration contributes context to the prompt without dominating it.
func truncateWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) <= n {
		return s
	}
	return strings.Join(fields[:n], " ")
}
