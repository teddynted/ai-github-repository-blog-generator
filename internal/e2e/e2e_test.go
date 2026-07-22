// Package e2e drives the full content pipeline from the committed sample-release
// fixtures in testdata/. It is the repository's end-to-end / acceptance /
// regression test: it loads a canonical Release Context + blog, runs the whole
// content suite offline (deterministic, no model, no network), and verifies the
// artifacts against committed goldens and grounding expectations.
//
// See testdata/README.md for how the fixtures are structured and regenerated.
package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentsuite"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// testdataDir walks up from the test's working directory to the module root
// (where go.mod lives) and returns its testdata/ directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate module root (go.mod)")
		}
		dir = parent
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

var isoTimestamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)

// normalizeTimestamps replaces volatile RFC3339 timestamps with a sentinel so
// generated JSON can be compared byte-for-byte against a committed golden.
func normalizeTimestamps(s string) string {
	return isoTimestamp.ReplaceAllString(s, "0001-01-01T00:00:00Z")
}

func loadContext(t *testing.T, td string) *rc.ReleaseContext {
	t.Helper()
	var rctx rc.ReleaseContext
	if err := json.Unmarshal([]byte(read(t, filepath.Join(td, "sample-release-context.json"))), &rctx); err != nil {
		t.Fatalf("parse sample-release-context.json: %v", err)
	}
	return &rctx
}

// TestEndToEndFromSampleRelease is the acceptance/regression test: it runs the
// full suite on the committed sample release and validates every stage.
func TestEndToEndFromSampleRelease(t *testing.T) {
	td := testdataDir(t)
	rctx := loadContext(t, td)
	blogMD := read(t, filepath.Join(td, "golden-blog.md"))
	// Title is derived from the blog's front matter exactly as cmd/generate-all
	// does, so downstream stages embed the same SourceBlogTitle as the goldens.
	blog := releasegen.BlogPost{Title: blogTitle(blogMD), Markdown: blogMD}

	suite := (&contentsuite.Orchestrator{}).Run(context.Background(), rctx, &blog)

	// 1. Manifest: all 11 stages, in canonical milestone order, none failed.
	if len(suite.Manifest.Stages) != 11 {
		t.Fatalf("want 11 stages, got %d", len(suite.Manifest.Stages))
	}
	for i, st := range suite.Manifest.Stages {
		if st.Milestone != i+3 {
			t.Errorf("stage %d out of order: milestone %d", i, st.Milestone)
		}
	}
	if suite.Manifest.Failed != 0 {
		var bad []string
		for _, st := range suite.Manifest.Stages {
			if st.Status == contentsuite.StageFailed {
				bad = append(bad, st.Name+": "+st.Error)
			}
		}
		t.Errorf("no stage should fail on the sample release; failed: %v", bad)
	}
	// The rich sample release grounds every stage → a full run.
	if suite.Manifest.Produced != 11 {
		t.Errorf("sample release should produce all 11 artifacts, got %d (skipped %d)", suite.Manifest.Produced, suite.Manifest.Skipped)
	}

	// 2. Grounding (acceptance): every artifact references the actual release.
	for _, a := range suite.Artifacts() {
		if !containsAny(a.Markdown, "Widget", "v1.0.0", "EventBridge", "SQS", "widget") {
			t.Errorf("artifact %q is not grounded in the release", a.Kind)
		}
	}

	// 3. Regression: the Markdown artifacts must byte-match their committed goldens
	//    (these goldens are the orchestrator's own canonical output).
	byKind := map[string]string{}
	for _, a := range suite.Artifacts() {
		byKind[a.Kind] = a.Markdown
	}
	goldenMD := map[string]string{
		"storyboard": "golden-storyboard.md",
		"youtube":    "golden-youtube.md",
		"linkedin":   "golden-linkedin.md",
		"x-thread":   "golden-x-thread.md",
	}
	for kind, file := range goldenMD {
		got, ok := byKind[kind]
		if !ok {
			t.Errorf("suite did not produce %q", kind)
			continue
		}
		want := read(t, filepath.Join(td, file))
		if got != want {
			t.Errorf("%s drifted from %s — regenerate goldens if intended (see testdata/README.md)", kind, file)
		}
	}
}

// TestJSONGoldensAreValidAndGrounded validates the structured JSON fixtures
// (SEO + thumbnail): they must be valid JSON, carry their key fields, and be
// grounded in the release. Timestamps are normalized so the check is stable.
func TestJSONGoldensAreValidAndGrounded(t *testing.T) {
	td := testdataDir(t)

	var seo map[string]any
	seoRaw := normalizeTimestamps(read(t, filepath.Join(td, "golden-seo.json")))
	if err := json.Unmarshal([]byte(seoRaw), &seo); err != nil {
		t.Fatalf("golden-seo.json invalid JSON: %v", err)
	}
	if !containsAny(seoRaw, "widget", "Widget", "eventbridge", "aws") {
		t.Error("golden-seo.json is not grounded in the release")
	}

	var thumb map[string]any
	thumbRaw := normalizeTimestamps(read(t, filepath.Join(td, "golden-thumbnail.json")))
	if err := json.Unmarshal([]byte(thumbRaw), &thumb); err != nil {
		t.Fatalf("golden-thumbnail.json invalid JSON: %v", err)
	}
	if !containsAny(thumbRaw, "widget", "Widget", "v1.0.0", "thumbnail") {
		t.Error("golden-thumbnail.json is not grounded in the release")
	}
}

// TestSampleReleaseNotesAndChangelogPresent is a smoke check that the raw input
// fixtures exist and describe the sample release.
func TestSampleReleaseNotesAndChangelogPresent(t *testing.T) {
	td := testdataDir(t)
	notes := read(t, filepath.Join(td, "sample-release-notes.md"))
	changelog := read(t, filepath.Join(td, "sample-changelog.md"))
	if !strings.Contains(notes, "v1.0.0") || !containsAny(notes, "EventBridge", "SQS") {
		t.Error("sample-release-notes.md should describe the v1.0.0 release")
	}
	if !strings.Contains(changelog, "[1.0.0]") {
		t.Error("sample-changelog.md should contain the 1.0.0 entry")
	}
}

// blogTitle mirrors cmd/generate-all's front-matter/H1 title extraction, so the
// fixtures and the pipeline agree on the blog title.
func blogTitle(md string) string {
	for _, ln := range strings.Split(md, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "title:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "title:")), "\"'")
		}
		if strings.HasPrefix(t, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(t, "# "))
		}
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
