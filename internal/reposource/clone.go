package reposource

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// DefaultCloneDepth bounds the shallow clone. It must exceed the number of
// commits the pipeline reads (the worker's processing CommitLimit, 20) so the
// commit-log walk stays inside the fetched history; a depth-1 clone cannot
// satisfy a multi-commit read and fails with "object not found".
const DefaultCloneDepth = 50

// GitCloner clones a repository with go-git (no external git binary). It uses a
// per-repository working directory that is replaced on each run, so disk use is
// bounded and no cleanup coordination is needed.
type GitCloner struct {
	Tokens  TokenSource
	BaseURL string // default https://github.com; a local path in tests
	WorkDir string // parent directory for clones
	// Depth bounds the shallow clone; 0 applies DefaultCloneDepth. Must exceed
	// the pipeline's commit-read limit or the log walk hits the shallow boundary.
	Depth  int
	Logger *slog.Logger
}

// Clone performs a shallow, single-branch clone and returns the local path.
func (c *GitCloner) Clone(ctx context.Context, repoFullName, ref string) (string, error) {
	token, err := c.Tokens.Token(ctx, repoFullName)
	if err != nil {
		return "", fmt.Errorf("resolve token: %w", err)
	}

	base := c.BaseURL
	if base == "" {
		base = "https://github.com"
	}
	url := strings.TrimRight(base, "/") + "/" + repoFullName + ".git"

	dir := filepath.Join(c.WorkDir, filepath.FromSlash(repoFullName))
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("clear work dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", fmt.Errorf("create work dir: %w", err)
	}

	depth := c.Depth
	if depth <= 0 {
		depth = DefaultCloneDepth
	}
	opts := &git.CloneOptions{URL: url, Depth: depth, SingleBranch: true}
	if ref != "" {
		opts.ReferenceName = plumbing.ReferenceName(ref)
	}
	if token != "" {
		// The token is passed via Auth, never embedded in the URL, so clone
		// errors do not leak it.
		opts.Auth = &http.BasicAuth{Username: "git", Password: token}
	}

	if _, err := git.PlainCloneContext(ctx, dir, false, opts); err != nil {
		return "", fmt.Errorf("clone %s: %w", url, err)
	}
	if c.Logger != nil {
		c.Logger.Info("cloned repository", slog.String("repo", repoFullName), slog.String("dir", dir))
	}
	return dir, nil
}
