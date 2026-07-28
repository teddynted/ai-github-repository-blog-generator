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
		"# Architecture Diagrams: acme/platform",
		"Repository-level architecture overview",
		"version-independent",
		"## Platform Overview",
		"## Hybrid AI Data Flow", // hybrid detected → hybrid title
		"## Architecture Intelligence",
		"| **Architecture style** |",
		"| **Local inference** | Ollama |",
		"## Generation Context",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("repo-level markdown missing %q\n---\n%s", want, md)
		}
	}
	if strings.Contains(md, "v0.10.0") {
		t.Errorf("repo-level markdown leaked a release version:\n%s", md)
	}
	// The title must not carry a version after the repo name.
	if strings.Contains(md, "Architecture Diagrams: acme/platform v") {
		t.Error("title should be repo-only, no version")
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
	if !strings.Contains(md, "## Logical Data Flow") {
		t.Errorf("expected neutral logical title:\n%s", md)
	}
}
