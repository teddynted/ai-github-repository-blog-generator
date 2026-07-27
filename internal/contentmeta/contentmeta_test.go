package contentmeta

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

func sampleContext() *rc.ReleaseContext {
	return &rc.ReleaseContext{
		Repository: rc.Repository{Owner: "acme", Name: "widget", FullName: "acme/widget"},
		Release:    rc.Release{Tag: "v0.3.0"},
		Commits:    []rc.Commit{{SHA: "abc123"}, {SHA: "def456"}},
	}
}

func fixedOpts() Options {
	return Options{
		GeneratorVersion: "gen-sha",
		Now:              func() time.Time { return time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC) },
		NewID:            func() string { return "gid-1" },
	}
}

func TestBuildRecordsProvenanceAndVersions(t *testing.T) {
	assets := []generation.Content{
		{Kind: "blog", Markdown: "# post", Provider: "claude", Model: "claude-opus", PromptVersion: "blog@6"},
		{Kind: "architecture-diagram", Markdown: "<svg/>", Ext: "svg", Provider: "deterministic", PromptVersion: "architecture-diagram@1"},
		{Kind: Kind, Markdown: "{}", Ext: Ext}, // the manifest itself — must be excluded
	}
	results := []generation.PutResult{
		{Kind: "blog", Ext: "", Key: "…/blog.md", VersionID: "v-blog"},
		{Kind: "architecture-diagram", Ext: "svg", Key: "…/architecture-diagram.svg", VersionID: "v-svg"},
	}

	m := Build(sampleContext(), assets, results, fixedOpts())

	if m.GenerationID != "gid-1" || m.SchemaVersion != SchemaVersion {
		t.Errorf("id/schema = %q / %q", m.GenerationID, m.SchemaVersion)
	}
	if m.Owner != "acme" || m.Repository != "acme/widget" || m.ReleaseTag != "v0.3.0" {
		t.Errorf("identity = %+v", m)
	}
	if m.GitCommit != "abc123" { // newest analysed commit = release head
		t.Errorf("gitCommit = %q, want abc123", m.GitCommit)
	}
	if m.GeneratorVersion != "gen-sha" || m.GeneratedAt != "2026-07-27T12:00:00Z" {
		t.Errorf("generator/time = %q / %q", m.GeneratorVersion, m.GeneratedAt)
	}
	if len(m.Artifacts) != 2 {
		t.Fatalf("artifacts = %d, want 2 (metadata excluded)", len(m.Artifacts))
	}

	blog := m.Artifacts["blog.md"]
	if blog.Provider != "claude" || blog.Model != "claude-opus" || blog.PromptVersion != "blog@6" {
		t.Errorf("blog provenance = %+v", blog)
	}
	if blog.S3VersionID != "v-blog" {
		t.Errorf("blog versionId = %q", blog.S3VersionID)
	}
	wantHash := hex.EncodeToString(func() []byte { s := sha256.Sum256([]byte("# post")); return s[:] }())
	if blog.SHA256 != wantHash {
		t.Errorf("blog sha256 = %q, want %q", blog.SHA256, wantHash)
	}

	svg := m.Artifacts["architecture-diagram.svg"]
	if svg.Provider != "deterministic" || svg.S3VersionID != "v-svg" {
		t.Errorf("svg provenance = %+v", svg)
	}
	if _, ok := m.Artifacts["metadata.json"]; ok {
		t.Error("the manifest must not describe itself")
	}
}

func TestBuildRecordsExperiment(t *testing.T) {
	opts := fixedOpts()
	opts.ExperimentID = "prompt-v7_claude"
	m := Build(sampleContext(), []generation.Content{{Kind: "blog", Markdown: "x"}}, nil, opts)
	if m.ExperimentID != "prompt-v7_claude" {
		t.Errorf("experiment = %q", m.ExperimentID)
	}
	c, _ := m.Content()
	if c.ExperimentID != "prompt-v7_claude" {
		t.Errorf("content experiment = %q (must route the manifest into experiments/)", c.ExperimentID)
	}
}

func TestBuildOmitsVersionWhenNoResults(t *testing.T) {
	// Filesystem publishing reports no versions — the manifest simply omits them.
	assets := []generation.Content{{Kind: "blog", Markdown: "x", Provider: "claude"}}
	m := Build(sampleContext(), assets, nil, fixedOpts())
	if m.Artifacts["blog.md"].S3VersionID != "" {
		t.Error("versionId should be empty without publish results")
	}
	if m.Artifacts["blog.md"].SHA256 == "" {
		t.Error("sha256 must always be recorded")
	}
}

func TestContentIsValidJSONArtifact(t *testing.T) {
	m := Build(sampleContext(), []generation.Content{{Kind: "blog", Markdown: "x"}}, nil, fixedOpts())
	c, err := m.Content()
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	if c.Kind != Kind || c.Ext != Ext || c.Release != "v0.3.0" {
		t.Errorf("content envelope = %+v", c)
	}
	var round Metadata
	if err := json.Unmarshal([]byte(c.Markdown), &round); err != nil {
		t.Fatalf("metadata.json is not valid JSON: %v", err)
	}
	if round.GenerationID != "gid-1" {
		t.Errorf("round-tripped id = %q", round.GenerationID)
	}
}

func TestBuildLatestPointer(t *testing.T) {
	l := BuildLatest("v0.3.0", "gid-1", []string{"blog.md", "architecture-diagram.svg"},
		func() time.Time { return time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC) })
	if l.Release != "v0.3.0" || l.GenerationID != "gid-1" || l.PromotedAt != "2026-07-27T12:00:00Z" || len(l.Artifacts) != 2 {
		t.Errorf("pointer = %+v", l)
	}
	c, err := l.Content()
	if err != nil {
		t.Fatalf("Content: %v", err)
	}
	if c.Kind != LatestKind || c.Ext != LatestExt {
		t.Errorf("envelope = %+v", c)
	}
	var round LatestPointer
	if err := json.Unmarshal([]byte(c.Markdown), &round); err != nil {
		t.Fatalf("latest.json invalid: %v", err)
	}
	if round.Release != "v0.3.0" {
		t.Errorf("round release = %q", round.Release)
	}
}

func TestNewGenerationIDUniqueAndSortable(t *testing.T) {
	a := NewGenerationID()
	b := NewGenerationID()
	if a == "" || a == b {
		t.Errorf("ids should be non-empty and unique: %q %q", a, b)
	}
}
