package releasecontext

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// Builder orchestrates the analyzers into a ReleaseContext. It depends only on
// the Sources port, so it is fully unit-testable and reusable from a Lambda, a
// CLI, or a test.
type Builder struct {
	Sources Sources
	// Now supplies the timestamp (injectable for deterministic tests).
	Now    func() time.Time
	Logger *slog.Logger
}

func (b *Builder) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now().UTC()
}

func (b *Builder) log(msg string, args ...any) {
	if b.Logger != nil {
		b.Logger.Info(msg, args...)
	}
}

// Build fetches the raw material for req and assembles the full Release Context.
// Analyzers that find nothing degrade gracefully and record a warning rather
// than failing the whole build.
func (b *Builder) Build(ctx context.Context, req Request) (*ReleaseContext, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if b.Sources == nil {
		return nil, apperror.New(apperror.CodeInternal, "release context builder has no sources configured")
	}

	meta, err := b.Sources.RepositoryMeta(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("repository metadata: %w", err)
	}
	rel, err := b.Sources.ReleaseByTag(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("release %q: %w", req.ReleaseTag, err)
	}
	commits, err := b.Sources.CommitsBetween(ctx, req, rel.PreviousTag)
	if err != nil {
		return nil, fmt.Errorf("commits: %w", err)
	}
	changed, err := b.Sources.ChangedFilesBetween(ctx, req, rel.PreviousTag)
	if err != nil {
		return nil, fmt.Errorf("changed files: %w", err)
	}
	files, err := b.Sources.Files(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("file inventory: %w", err)
	}

	rc := &ReleaseContext{
		SchemaVersion: SchemaVersion,
		ContextID:     newContextID(),
		GeneratedAt:   b.now().Format(time.RFC3339),
	}
	rc.Repository = buildRepository(meta)
	rc.Release = buildRelease(rel)
	rc.Documentation = analyzeDocumentation(files)
	rc.RepositoryStructure = analyzeStructure(files)
	rc.Commits, rc.CommitStats = analyzeCommits(commits)
	rc.ChangedFiles, rc.FileStats = analyzeChangedFiles(changed)
	rc.CloudFormation = analyzeCloudFormation(files)
	rc.Technologies = analyzeTechnologies(files, rc.CloudFormation.Services)
	rc.Mermaid = analyzeMermaid(files)
	rc.Changelog = analyzeChangelog(findChangelog(files), rel.Tag)

	// These depend on the analyses above, so they run last.
	rc.Architecture = analyzeArchitecture(rc)
	rc.Implementation = buildImplementation(rc)
	rc.ContentIntelligence = buildContentIntelligence(rc)

	rc.Warnings = collectWarnings(rc, commits)
	b.log("release context built",
		slog.String("contextId", rc.ContextID),
		slog.String("repository", rc.Repository.FullName),
		slog.String("release", rc.Release.Tag),
		slog.Int("commits", rc.CommitStats.Analyzed),
		slog.Int("changedFiles", rc.FileStats.Total),
		slog.Int("technologies", len(rc.Technologies)),
		slog.Int("warnings", len(rc.Warnings)),
	)
	return rc, nil
}

func buildRepository(m RawRepository) Repository {
	full := m.FullName
	if full == "" {
		full = m.Owner + "/" + m.Name
	}
	r := Repository{
		Owner: m.Owner, Name: m.Name, FullName: full, Description: m.Description,
		Topics: m.Topics, Homepage: m.Homepage, License: m.License, Visibility: m.Visibility,
		DefaultBranch: m.DefaultBranch, Language: m.Language, URL: m.URL,
	}
	r.Summary = repositorySummary(r)
	return r
}

func repositorySummary(r Repository) string {
	parts := []string{fmt.Sprintf("%s is", r.FullName)}
	if r.Description != "" {
		parts = append(parts, "a project that "+lowerFirst(strings.TrimSuffix(r.Description, ".")))
	} else {
		parts = append(parts, "a software project")
	}
	if r.Language != "" {
		parts = append(parts, "written primarily in "+r.Language)
	}
	if r.License != "" {
		parts = append(parts, "under the "+r.License+" license")
	}
	return strings.Join(parts, " ") + "."
}

func buildRelease(r RawRelease) Release {
	rel := Release{
		Tag: r.Tag, Name: r.Name, PublishedAt: r.PublishedAt, Author: r.Author,
		Body: r.Body, URL: r.URL, PreRelease: r.PreRelease, PreviousTag: r.PreviousTag, Assets: r.Assets,
	}
	rel.Summary = releaseSummary(rel)
	return rel
}

func releaseSummary(r Release) string {
	name := r.Name
	if name == "" {
		name = r.Tag
	}
	base := fmt.Sprintf("Release %s", name)
	if r.PublishedAt != "" {
		base += " published " + r.PublishedAt
	}
	if r.Author != "" {
		base += " by " + r.Author
	}
	if s := firstSentence(r.Body); s != "" {
		base += ". " + s
	}
	return base + "."
}

// findChangelog returns the content of a CHANGELOG file from the inventory.
func findChangelog(files []RawFile) string {
	for _, f := range files {
		b := strings.ToLower(path.Base(f.Path))
		if (b == "changelog.md" || b == "changelog") && f.Content != "" {
			return f.Content
		}
	}
	return ""
}

func collectWarnings(rc *ReleaseContext, rawCommits []RawCommit) []string {
	var w []string
	if !rc.Changelog.Found {
		w = append(w, fmt.Sprintf("no CHANGELOG entry found for %s", rc.Release.Tag))
	}
	if len(rc.CloudFormation.Templates) == 0 {
		w = append(w, "no CloudFormation templates found")
	}
	if len(rawCommits) == 0 {
		w = append(w, "no commits found in the release range")
	}
	if rc.RepositoryStructure.FileCount == 0 {
		w = append(w, "empty or unavailable file inventory")
	}
	return w
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
