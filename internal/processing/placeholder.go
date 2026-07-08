package processing

import "context"

// The types below are PLACEHOLDER implementations for the MVP. They return
// representative canned data instead of touching a real repository, so the
// pipeline is wired and testable without cloning or analysis. Repository
// Intelligence replaces these with real integrations (e.g. OpenClaw-backed
// clone + retrieval) without changing the Processor or its ports.

// PlaceholderCloner returns a deterministic fake working path.
type PlaceholderCloner struct{}

// Clone does not clone; it returns a synthetic local path.
func (PlaceholderCloner) Clone(_ context.Context, repoFullName, _ string) (string, error) {
	return "/tmp/blog-gen/placeholder/" + repoFullName, nil
}

// PlaceholderReadme returns canned README content.
type PlaceholderReadme struct{}

// Readme returns placeholder README text.
func (PlaceholderReadme) Readme(_ context.Context, _ string) (string, error) {
	return "# Placeholder README\n\nRepository retrieval is not yet implemented (Repository Intelligence, future milestone).\n", nil
}

// PlaceholderDocs returns a single canned document.
type PlaceholderDocs struct{}

// Docs returns placeholder documentation.
func (PlaceholderDocs) Docs(_ context.Context, _ string) ([]Document, error) {
	return []Document{{
		Path:    "docs/placeholder.md",
		Content: "Placeholder documentation. Real retrieval arrives with Repository Intelligence.",
	}}, nil
}

// PlaceholderCommits returns canned commit summaries.
type PlaceholderCommits struct{}

// Commits returns up to limit placeholder commits.
func (PlaceholderCommits) Commits(_ context.Context, _ string, limit int) ([]Commit, error) {
	all := []Commit{
		{SHA: "0000001", Message: "blog: placeholder trigger commit", Author: "placeholder"},
		{SHA: "0000002", Message: "chore: placeholder", Author: "placeholder"},
	}
	if limit >= 0 && limit < len(all) {
		return all[:limit], nil
	}
	return all, nil
}

// NewPlaceholderProcessor wires the placeholder stages into a Processor.
func NewPlaceholderProcessor() *Processor {
	return &Processor{
		Cloner:      PlaceholderCloner{},
		Readme:      PlaceholderReadme{},
		Docs:        PlaceholderDocs{},
		Commits:     PlaceholderCommits{},
		CommitLimit: 20,
	}
}

// Guards: placeholders satisfy their ports.
var (
	_ Cloner          = PlaceholderCloner{}
	_ ReadmeRetriever = PlaceholderReadme{}
	_ DocsRetriever   = PlaceholderDocs{}
	_ CommitRetriever = PlaceholderCommits{}
)
