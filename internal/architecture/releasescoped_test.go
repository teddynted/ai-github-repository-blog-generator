package architecture

import (
	"context"
	"strings"
	"testing"
)

func TestReleaseScopedMarkdownIsReleaseDerived(t *testing.T) {
	pkg := samplePackage() // Release.Tag v0.2.0, repo "widget"
	pkg.Context.Release.Body = "Adds a pre-baked AMI build pipeline and reduces Spot instance startup latency."
	col, err := newGen().Architecture(context.Background(), pkg)
	if err != nil {
		t.Fatalf("Architecture: %v", err)
	}
	md := ReleaseScopedMarkdown(col, pkg.Context, pkg.Blog)

	for _, want := range []string{
		"# Release Architecture: widget v0.2.0", // titled WITH the release version
		"derived from the release blog",
		"`blog.md`",
		"## Release Context",
		"- **Release:** v0.2.0",
		"## What Changed in This Release",
		"## Affected AWS Components",
		"## Updated Architecture Flow",
		"## Generation Context",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("release-scoped markdown missing %q\n---\n%s", want, md)
		}
	}
	// The forbidden version-independent / repository-level language must be gone.
	for _, bad := range []string{
		"version-independent",
		"Repository-level architecture overview",
		"current repository state",
		"reused across releases",
	} {
		if strings.Contains(md, bad) {
			t.Errorf("release-scoped markdown must not contain %q", bad)
		}
	}
}
