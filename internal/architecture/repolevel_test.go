package architecture

import (
	"context"
	"strings"
	"testing"
)

func TestRepoLevelMarkdownIsVersionIndependent(t *testing.T) {
	// hybridPackage (from hybrid_test.go) carries release tag v0.10.0; the
	// repo-level document must NOT leak any version, date, or tag.
	col, err := newGen().Architecture(context.Background(), hybridPackage())
	if err != nil {
		t.Fatalf("Architecture: %v", err)
	}
	md := col.RepoLevelMarkdown()

	for _, want := range []string{
		"# Architecture Diagrams: platform", // bare repo name, owner stripped
		"Repository-level architecture overview",
		"version-independent",
		"## Platform Overview",
		"## Logical Architecture — Hybrid AI Data Flow", // hybrid detected → hybrid title
		"## Architecture Intelligence",
		"| **Architecture style** |",
		"| **Local inference** | Ollama Runtime |",
		"## Generation Context",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("repo-level markdown missing %q\n---\n%s", want, md)
		}
	}
	if strings.Contains(md, "v0.10.0") {
		t.Errorf("repo-level markdown leaked a release version:\n%s", md)
	}
	// The title must carry neither owner prefix nor version.
	if strings.Contains(md, "acme/platform") || strings.Contains(md, "Architecture Diagrams: platform v") {
		t.Error("title should be the bare repo name, no owner, no version")
	}
}

func TestRepoLevelMarkdownNonHybridTitle(t *testing.T) {
	// A plain AWS release uses the neutral "Logical Data Flow" title, not hybrid.
	col, err := newGen().Architecture(context.Background(), samplePackage())
	if err != nil {
		t.Fatalf("Architecture: %v", err)
	}
	md := col.RepoLevelMarkdown()
	if strings.Contains(md, "Hybrid AI Data Flow") {
		t.Errorf("non-hybrid repo-level doc must not use hybrid title:\n%s", md)
	}
	if !strings.Contains(md, "## Logical Architecture — Data Flow") {
		t.Errorf("expected neutral logical title:\n%s", md)
	}
}
