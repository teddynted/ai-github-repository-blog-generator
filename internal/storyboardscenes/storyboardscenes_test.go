package storyboardscenes

import (
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
)

func sampleStoryboard() storyboard.Storyboard {
	return storyboard.Storyboard{
		Metadata: storyboard.Metadata{SourceBlogTitle: "Fast Spot Startup with Pre-Baked AMIs"},
		Scenes: []storyboard.Scene{
			{SceneNumber: 1, Type: "introduction", Narration: "Repositories are the one decision that changed everything. Here is why.",
				Visuals: storyboard.Visuals{Description: "A single glowing entry node opening into a layered cloud system."},
				Camera:  storyboard.Camera{Direction: "Push In"}, Duration: storyboard.Duration{RecommendedSec: 6},
				Animations: []storyboard.Animation{{Type: "Fade In"}}},
			{SceneNumber: 2, Type: "problem", Narration: "Boot-time provisioning was slow and non-deterministic.",
				Camera: storyboard.Camera{Direction: "Static"}, Duration: storyboard.Duration{RecommendedSec: 5}},
			{SceneNumber: 3, Type: "architecture", Narration: "A scheduled event triggers a serverless control plane.",
				Visuals:    storyboard.Visuals{Description: "Layered infrastructure planes connected by event arrows."},
				Animations: []storyboard.Animation{{Type: "Diagram Build"}}, Duration: storyboard.Duration{RecommendedSec: 8}},
			{SceneNumber: 4, Type: "generic", Narration: ""}, // skipped (no narration)
		},
	}
}

func TestBuildProducesSDXLScenes(t *testing.T) {
	col := Build(sampleStoryboard(), "teddynted/designing-an-ai-agent-platform-on-aws", "v0.6.0")

	if col.ModelTarget != "stability-ai/sdxl" {
		t.Errorf("model target = %q", col.ModelTarget)
	}
	// The empty-narration scene is skipped; the rest are renumbered 1..3.
	if len(col.Scenes) != 3 {
		t.Fatalf("want 3 renderable scenes, got %d", len(col.Scenes))
	}
	for i, s := range col.Scenes {
		if s.Number != i+1 {
			t.Errorf("scene %d renumbered to %d", i, s.Number)
		}
		if !strings.HasPrefix(s.SDXLPrompt, SharedStylePrompt) {
			t.Errorf("scene %d prompt should lead with the shared style anchor", s.Number)
		}
		if !strings.Contains(s.SDXLPrompt, "Composition: ") {
			t.Errorf("scene %d prompt missing composition", s.Number)
		}
		if s.NegativePrompt != SharedNegativePrompt {
			t.Errorf("scene %d negative prompt mismatch", s.Number)
		}
		if s.DurationSec <= 0 || s.CameraDirection == "" || s.MotionSuggestion == "" || s.Purpose == "" {
			t.Errorf("scene %d incomplete: %+v", s.Number, s)
		}
	}
	// Purpose + camera mapping.
	if col.Scenes[0].Purpose != "Hook" || col.Scenes[0].CameraDirection != "slow push in" {
		t.Errorf("scene 1 = %+v", col.Scenes[0])
	}
	if col.Scenes[1].Purpose != "Problem" {
		t.Errorf("scene 2 purpose = %q", col.Scenes[1].Purpose)
	}
	// Compositions rotate (adjacent scenes differ).
	if col.Scenes[0].Composition == col.Scenes[1].Composition {
		t.Errorf("adjacent scenes share a composition: %q", col.Scenes[0].Composition)
	}
	// A scene with no visuals description falls back to a grounded metaphor, never empty.
	if strings.TrimSpace(col.Scenes[1].SDXLPrompt) == SharedStylePrompt {
		t.Error("scene with no visuals should still carry a focal metaphor")
	}
}

func TestMarkdownStructure(t *testing.T) {
	md := Build(sampleStoryboard(), "acme/widget", "v1.0.0").Markdown()
	for _, want := range []string{
		"# Storyboard Scenes:", "image_model:", "model: stability-ai/sdxl",
		"## Shared Visual Style", "### Shared Negative Prompt",
		"## Scene 01", "### Purpose", "### Narration Alignment", "### Camera Direction",
		"### SDXL Prompt", "### Motion Suggestion", "### Duration",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestBuildDeterministic(t *testing.T) {
	a := Build(sampleStoryboard(), "acme/widget", "v1.0.0").Markdown()
	b := Build(sampleStoryboard(), "acme/widget", "v1.0.0").Markdown()
	if a != b {
		t.Error("Build must be deterministic for the same storyboard + release")
	}
}
