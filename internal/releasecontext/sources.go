package releasecontext

import "context"

// Request is the input to a context build: which repository and which release.
type Request struct {
	Owner      string
	Repository string
	ReleaseTag string
}

// FullName returns "owner/repository".
func (r Request) FullName() string { return r.Owner + "/" + r.Repository }

// Sources is the port the Builder pulls raw material from. An adapter (GitHub
// REST API + git, added in the I/O layer) implements it; tests provide a fake.
// Every method returns already-fetched, in-memory data so the analyzers stay
// pure and deterministic.
type Sources interface {
	// RepositoryMeta returns the repository's metadata.
	RepositoryMeta(ctx context.Context, req Request) (RawRepository, error)
	// ReleaseByTag returns the selected release and the immediately previous
	// release tag ("" if this is the first release).
	ReleaseByTag(ctx context.Context, req Request) (RawRelease, error)
	// CommitsBetween returns commits in (previousTag, releaseTag]; previousTag
	// "" means from the start of history.
	CommitsBetween(ctx context.Context, req Request, previousTag string) ([]RawCommit, error)
	// ChangedFilesBetween returns files changed in (previousTag, releaseTag].
	ChangedFilesBetween(ctx context.Context, req Request, previousTag string) ([]RawChangedFile, error)
	// Files returns the repository's file inventory at the release tag. Content
	// is populated only for text files the analyzers care about (docs, IaC,
	// manifests); large/binary files carry an empty Content and a size.
	Files(ctx context.Context, req Request) ([]RawFile, error)
}

// RawRepository is unprocessed repository metadata from the source.
type RawRepository struct {
	Owner         string
	Name          string
	FullName      string
	Description   string
	Topics        []string
	Homepage      string
	License       string
	Visibility    string
	DefaultBranch string
	Language      string
	URL           string
}

// RawRelease is an unprocessed GitHub Release.
type RawRelease struct {
	Tag         string
	Name        string
	PublishedAt string
	Author      string
	Body        string
	URL         string
	PreRelease  bool
	PreviousTag string
	Assets      []ReleaseAsset
}

// RawCommit is an unprocessed commit.
type RawCommit struct {
	SHA     string
	Subject string
	Author  string
	Date    string
	Parents int // >1 => merge commit
}

// RawChangedFile is an unprocessed changed-file entry.
type RawChangedFile struct {
	Path      string
	Status    string
	Additions int
	Deletions int
}

// RawFile is one file in the repository inventory. Content is empty for files
// the source chose not to fetch (binary/oversized); Path is always set.
type RawFile struct {
	Path    string
	Content string
	Size    int64
}
