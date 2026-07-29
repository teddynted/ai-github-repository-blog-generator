package architecture

import (
	"context"
	"strings"
	"testing"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

func TestSanitizeOverviewRepairsThinContextArtifacts(t *testing.T) {
	in := "An event-driven, AWS-native system built from 2 major components on . A standard-go project of 65 files organized into 4 top-level directories."
	got := sanitizeOverview(in)
	if strings.Contains(got, " on .") {
		t.Errorf("empty-service ' on .' fragment not repaired: %q", got)
	}
	if strings.Contains(got, "65 files") || strings.Contains(got, "of 65") {
		t.Errorf("file count not stripped: %q", got)
	}
	if strings.Contains(got, "major components") || strings.Contains(got, "built from") {
		t.Errorf("machine-generated 'built from N major components' clause not stripped: %q", got)
	}
	if !strings.Contains(got, "An event-driven, AWS-native system.") {
		t.Errorf("expected clean lead sentence, got %q", got)
	}
}

func TestArchitectureStyleFromEventDrivenProse(t *testing.T) {
	// A context whose structured event/messaging fields are empty but whose own
	// architecture prose calls the system event-driven must classify as
	// Event-driven, not fall back to "Application".
	ctx := &rc.ReleaseContext{
		SchemaVersion: "1.0.0",
		Repository:    rc.Repository{Name: "demo", FullName: "acme/demo"},
		Architecture: rc.Architecture{
			Overview:    "An event-driven, AWS-native system.",
			AWSServices: []string{"Amazon S3"}, // storage only — not messaging/serverless
		},
		RepositoryStructure: rc.RepositoryStructure{
			Directories: []rc.DirectoryInfo{{Path: "cmd"}, {Path: "internal"}},
		},
	}
	col, err := newGen().Architecture(context.Background(), ReleasePackage{Context: ctx, Blog: releasegen.BlogPost{}})
	if err != nil {
		t.Fatalf("Architecture: %v", err)
	}
	if !strings.Contains(col.ContentIntelligence.ArchitectureStyle, "Event-driven") {
		t.Errorf("expected Event-driven style from prose, got %q", col.ContentIntelligence.ArchitectureStyle)
	}
}
