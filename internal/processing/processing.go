// Package processing defines the repository-processing pipeline seam: the
// stages that gather raw material from a repository (clone, then retrieve the
// README, documentation, and commits). It deliberately stops short of any
// analysis or content generation — that is Repository Intelligence, a future
// milestone. The MVP ships placeholder implementations (see placeholder.go) so
// the pipeline is wired end to end and the extension points are fixed.
package processing

import (
	"context"
	"fmt"
	"log/slog"
)

// Document is a retrieved documentation file.
type Document struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Commit is a retrieved commit summary.
type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
}

// Analysis is the detected technical profile of a repository.
type Analysis struct {
	Languages       []string `json:"languages"`        // e.g. Go, TypeScript
	PackageManagers []string `json:"package_managers"` // e.g. Go modules, npm
	IaC             []string `json:"iac"`              // e.g. Terraform, CloudFormation
	Containers      []string `json:"containers"`       // e.g. Docker, Docker Compose
	CICD            []string `json:"cicd"`             // e.g. GitHub Actions
}

// Snapshot is the material the generation stage works from.
type Snapshot struct {
	RepoFullName string     `json:"repo_full_name"`
	Ref          string     `json:"ref"`
	LocalPath    string     `json:"local_path"`
	Readme       string     `json:"readme"`
	Docs         []Document `json:"docs"`
	Commits      []Commit   `json:"commits"`
	Analysis     Analysis   `json:"analysis"`
}

// Cloner clones or syncs a repository and returns its local working path.
type Cloner interface {
	Clone(ctx context.Context, repoFullName, ref string) (localPath string, err error)
}

// ReadmeRetriever reads the repository README.
type ReadmeRetriever interface {
	Readme(ctx context.Context, localPath string) (string, error)
}

// DocsRetriever reads documentation files.
type DocsRetriever interface {
	Docs(ctx context.Context, localPath string) ([]Document, error)
}

// CommitRetriever reads recent commits (up to limit).
type CommitRetriever interface {
	Commits(ctx context.Context, localPath string, limit int) ([]Commit, error)
}

// Analyzer detects the repository's technical profile (tech stack, dependency
// managers, IaC, containers, CI/CD).
type Analyzer interface {
	Analyze(ctx context.Context, localPath string) (Analysis, error)
}

// Processor orchestrates the retrieval stages into a Snapshot. It performs no
// analysis or generation; Repository Intelligence consumes the Snapshot later.
type Processor struct {
	Cloner      Cloner
	Readme      ReadmeRetriever
	Docs        DocsRetriever
	Commits     CommitRetriever
	Analyzer    Analyzer // optional; detects the technical profile
	CommitLimit int
	Logger      *slog.Logger
}

// Process gathers a Snapshot for a repository at a ref.
func (p *Processor) Process(ctx context.Context, repoFullName, ref string) (Snapshot, error) {
	limit := p.CommitLimit
	if limit <= 0 {
		limit = 20
	}

	path, err := p.Cloner.Clone(ctx, repoFullName, ref)
	if err != nil {
		return Snapshot{}, fmt.Errorf("clone %s: %w", repoFullName, err)
	}
	p.log("cloned", repoFullName, path)

	readme, err := p.Readme.Readme(ctx, path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read readme: %w", err)
	}
	docs, err := p.Docs.Docs(ctx, path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read docs: %w", err)
	}
	commits, err := p.Commits.Commits(ctx, path, limit)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read commits: %w", err)
	}

	var analysis Analysis
	if p.Analyzer != nil {
		analysis, err = p.Analyzer.Analyze(ctx, path)
		if err != nil {
			return Snapshot{}, fmt.Errorf("analyze: %w", err)
		}
	}

	p.log("snapshot ready", repoFullName, path)
	return Snapshot{
		RepoFullName: repoFullName,
		Ref:          ref,
		LocalPath:    path,
		Readme:       readme,
		Docs:         docs,
		Commits:      commits,
		Analysis:     analysis,
	}, nil
}

func (p *Processor) log(msg, repoName, path string) {
	if p.Logger != nil {
		p.Logger.Info(msg, slog.String("repo", repoName), slog.String("path", path))
	}
}
