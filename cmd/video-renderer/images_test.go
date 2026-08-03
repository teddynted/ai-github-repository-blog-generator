package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/imagegen"
)

type fakeGen struct {
	png  []byte
	err  error
	last imagegen.Spec
}

func (f *fakeGen) Generate(_ context.Context, spec imagegen.Spec) ([]byte, error) {
	f.last = spec
	return f.png, f.err
}

func TestNovaDims(t *testing.T) {
	if w, h := novaDims(1920, 1080); w != 1280 || h != 720 {
		t.Errorf("16:9 dims = %dx%d, want 1280x720", w, h)
	}
	if w, h := novaDims(1080, 1920); w != 720 || h != 1280 {
		t.Errorf("9:16 dims = %dx%d, want 720x1280", w, h)
	}
}

func TestTruncateWords(t *testing.T) {
	if got := truncateWords("one two three four", 2); got != "one two" {
		t.Errorf("truncateWords = %q", got)
	}
	if got := truncateWords("short", 40); got != "short" {
		t.Errorf("under-limit should be unchanged, got %q", got)
	}
}

func TestSceneImagePromptExcludesTextAndCarriesSubject(t *testing.T) {
	p, neg := sceneImagePrompt(scene{Title: "Event-Driven Ingestion", Narration: "A webhook triggers the pipeline."})
	if !strings.Contains(p, "Event-Driven Ingestion") {
		t.Errorf("prompt should carry the scene title: %q", p)
	}
	if !strings.Contains(strings.ToLower(p), "no text") {
		t.Errorf("prompt must forbid on-image text: %q", p)
	}
	if !strings.Contains(neg, "text") {
		t.Errorf("negative prompt should exclude text, got %q", neg)
	}
}

func TestMaybeSceneImageWritesOnSuccess(t *testing.T) {
	work := t.TempDir()
	gen := &fakeGen{png: []byte("\x89PNGdata")}
	got := maybeSceneImage(context.Background(), gen, scene{Number: 3, Title: "Solution"}, work, 1920, 1080)
	if got == "" {
		t.Fatal("expected a background path on success")
	}
	if filepath.Dir(got) != work {
		t.Errorf("image not written under work dir: %q", got)
	}
	b, err := os.ReadFile(got)
	if err != nil || string(b) != "\x89PNGdata" {
		t.Errorf("written image = %q err=%v", b, err)
	}
	// The seed is stable (the scene number) so a re-render reproduces the image.
	if gen.last.Seed != 3 || gen.last.Width != 1280 {
		t.Errorf("spec = %+v, want seed 3 and 16:9 dims", gen.last)
	}
}

func TestMaybeSceneImageFallsBackOnError(t *testing.T) {
	got := maybeSceneImage(context.Background(), &fakeGen{err: errors.New("throttled after 4 attempts")}, scene{Number: 1}, t.TempDir(), 1080, 1920)
	if got != "" {
		t.Errorf("a generation failure must fall back to the title card (empty path), got %q", got)
	}
}

func TestMaybeSceneImageNilGeneratorIsCard(t *testing.T) {
	if got := maybeSceneImage(context.Background(), nil, scene{Number: 1}, t.TempDir(), 1920, 1080); got != "" {
		t.Errorf("nil generator must yield an empty path, got %q", got)
	}
}
