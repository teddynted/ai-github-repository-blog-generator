package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// isoTimestamp matches RFC3339 timestamps some artifacts embed (e.g. SEO
// dateModified); the snapshot normalises them so comparison stays deterministic.
var isoTimestamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)

func normalize(s string) string {
	return isoTimestamp.ReplaceAllString(s, "0001-01-01T00:00:00Z")
}

// fakeModel returns deterministic multi-section prose so the full suite (blog +
// downstream) produces stable output for snapshot testing.
type fakeModel struct{}

func (fakeModel) Generate(_ context.Context, _ string) (string, error) {
	return "## Introduction\n\nThe system decouples events from generation.\n\n" +
		"## Architecture\n\nEventBridge routes to SQS, drained by an EC2 worker.\n\n" +
		"## Conclusion\n\nThe pattern generalises to event-driven workloads.\n", nil
}

// repoRoot walks up to the module root (where go.mod lives).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func loadFixture(t *testing.T) string {
	return filepath.Join(repoRoot(t), "fixtures", "v0.3.0.json")
}

func TestProduceBlogStandalone(t *testing.T) {
	rctx, err := loadContext(loadFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	fm := fakeModel{}
	arts, err := produce(context.Background(), "blog", rctx, fm, func(string) releasegen.Model { return fm }, nil, nil)
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	if len(arts) != 1 || arts[0].kind != "blog" {
		t.Fatalf("produce blog = %+v", arts)
	}
	if arts[0].markdown == "" {
		t.Error("blog markdown empty")
	}
}

func TestProduceAllViaSuite(t *testing.T) {
	rctx, err := loadContext(loadFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	fm := fakeModel{}
	arts, err := produce(context.Background(), "all", rctx, fm, func(string) releasegen.Model { return fm }, nil, nil)
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	kinds := map[string]bool{}
	for _, a := range arts {
		kinds[a.kind] = true
	}
	for _, want := range []string{"blog", "storyboard", "linkedin", "seo-metadata"} {
		if !kinds[want] {
			t.Errorf("suite output missing %q", want)
		}
	}
}

func TestLoadContextErrors(t *testing.T) {
	if _, err := loadContext(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("missing file should error")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o644)
	if _, err := loadContext(bad); err == nil {
		t.Error("invalid JSON should error")
	}
}

func TestBuildModelErrors(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, _, err := buildModel(context.Background(), options{provider: "anthropic"}); err == nil {
		t.Error("anthropic without a key should error")
	}
	if _, _, err := buildModel(context.Background(), options{provider: "bedrock"}); err == nil {
		t.Error("bedrock without --model should error")
	}
	if _, _, err := buildModel(context.Background(), options{provider: "nope"}); err == nil {
		t.Error("unknown provider should error")
	}
}

// TestSnapshots is the deterministic regression gate: the full suite over the
// committed fixture, with a fixed model, must byte-match testdata/expected/*.
// Run `make snapshot-update` (UPDATE_SNAPSHOTS=1) to refresh intentionally —
// snapshots are never overwritten automatically.
func TestSnapshots(t *testing.T) {
	rctx, err := loadContext(loadFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	fm := fakeModel{}
	arts, err := produce(context.Background(), "all", rctx, fm, func(string) releasegen.Model { return fm }, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repoRoot(t), "testdata", "expected")
	update := os.Getenv("UPDATE_SNAPSHOTS") == "1"
	if update {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range arts {
		path := filepath.Join(dir, a.kind+"."+a.ext)
		got := normalize(a.markdown)
		if update {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatalf("write snapshot %s: %v", path, err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing snapshot %s — run `make snapshot-update`", path)
		}
		if normalize(string(want)) != got {
			t.Errorf("%s drifted from snapshot — run `make snapshot-update` if intended", a.kind)
		}
	}
}
