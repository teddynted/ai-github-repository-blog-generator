package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentsuite"
)

// writeFixtures writes a Release Context JSON + a well-formed blog into dir and
// returns their paths. The context has groundable infra so architecture succeeds.
func writeFixtures(t *testing.T, dir string) (ctxPath, blogPath string) {
	t.Helper()
	ctx := `{
      "schemaVersion":"1.0.0","contextId":"cli-1",
      "repository":{"owner":"acme","name":"demo","fullName":"acme/demo","language":"Go"},
      "release":{"tag":"v1.0.0","name":"First","body":"Adds an EventBridge pipeline and an SQS buffer."},
      "architecture":{"overview":"Event-driven platform","awsServices":["AWS Lambda","Amazon SQS"]},
      "cloudformation":{"templates":["infra/stack.yaml"],"services":["AWS Lambda","Amazon SQS"],
        "resources":[{"logicalId":"Queue","type":"AWS::SQS::Queue","service":"Amazon SQS","category":"Messaging"}]},
      "mermaid":[{"type":"flowchart","nodes":["A","B"],"summary":"Flow"}]
    }`
	blog := "# First release\n\ntags: [go, aws]\n\nIntro.\n\n## The problem\n\nManual publishing was slow.\n\n## The solution\n\nAn event-driven pipeline on AWS.\n\n## How it works\n\nA webhook triggers EventBridge, buffered by SQS.\n"
	ctxPath = filepath.Join(dir, "ctx.json")
	blogPath = filepath.Join(dir, "post.md")
	if err := os.WriteFile(ctxPath, []byte(ctx), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blogPath, []byte(blog), 0o644); err != nil {
		t.Fatal(err)
	}
	return ctxPath, blogPath
}

func TestRunGeneratesAllArtifactsOffline(t *testing.T) {
	dir := t.TempDir()
	ctxPath, blogPath := writeFixtures(t, dir)
	out := filepath.Join(dir, "artifacts")

	code := run([]string{"--context", ctxPath, "--blog", blogPath, "--offline", "--out", out})
	if code != 0 {
		t.Fatalf("run exit = %d, want 0", code)
	}

	// The manifest is written and reports a full, healthy run.
	raw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	var m contentsuite.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest invalid JSON: %v", err)
	}
	if m.Repository != "acme/demo" || m.Release != "v1.0.0" {
		t.Errorf("manifest identity wrong: %+v", m)
	}
	if len(m.Stages) != 11 {
		t.Errorf("want 11 stages, got %d", len(m.Stages))
	}
	if m.Produced != 11 || m.Failed != 0 {
		t.Errorf("want produced=11 failed=0, got produced=%d skipped=%d failed=%d", m.Produced, m.Skipped, m.Failed)
	}

	// Every produced stage's artifact file exists and is non-empty.
	for _, st := range m.Stages {
		if st.Artifact == "" {
			continue
		}
		fi, err := os.Stat(filepath.Join(out, st.Artifact))
		if err != nil || fi.Size() == 0 {
			t.Errorf("artifact %s missing or empty: %v", st.Artifact, err)
		}
	}
}

func TestRunRequiresContext(t *testing.T) {
	if code := run([]string{"--offline"}); code != 2 {
		t.Errorf("missing --context should exit 2, got %d", code)
	}
}

func TestRunOfflineRequiresBlog(t *testing.T) {
	dir := t.TempDir()
	ctxPath, _ := writeFixtures(t, dir)
	if code := run([]string{"--context", ctxPath, "--offline"}); code != 2 {
		t.Errorf("--offline without --blog should exit 2, got %d", code)
	}
}

func TestRunSkipsGracefullyOnThinRelease(t *testing.T) {
	// A section-less blog + a context with no infrastructure: the run still
	// succeeds (exit 0) with skips recorded, not failures.
	dir := t.TempDir()
	ctxPath := filepath.Join(dir, "thin.json")
	blogPath := filepath.Join(dir, "thin.md")
	_ = os.WriteFile(ctxPath, []byte(`{"schemaVersion":"1.0.0","contextId":"thin",
      "repository":{"fullName":"acme/thin"},"release":{"tag":"v0.1.0","body":"A small fix."}}`), 0o644)
	_ = os.WriteFile(blogPath, []byte("# Small fix\n\nFixes a race in the SQS drainer.\n"), 0o644)
	out := filepath.Join(dir, "thin-artifacts")

	if code := run([]string{"--context", ctxPath, "--blog", blogPath, "--offline", "--out", out}); code != 0 {
		t.Fatalf("thin release should still exit 0, got %d", code)
	}
	raw, _ := os.ReadFile(filepath.Join(out, "manifest.json"))
	var m contentsuite.Manifest
	_ = json.Unmarshal(raw, &m)
	if m.Produced == 0 {
		t.Error("thin release should still produce some artifacts")
	}
	if m.Skipped == 0 {
		t.Error("thin release should record graceful skips (architecture/shorts/tiktok)")
	}
}

func TestBlogTitle(t *testing.T) {
	if got := blogTitle("---\ntitle: Hello World\n---\n# Body\n"); got != "Hello World" {
		t.Errorf("front-matter title: %q", got)
	}
	if got := blogTitle("# Just an H1\n\nbody"); got != "Just an H1" {
		t.Errorf("h1 title: %q", got)
	}
	if got := blogTitle("no title here"); got != "" {
		t.Errorf("no title should be empty, got %q", got)
	}
}
