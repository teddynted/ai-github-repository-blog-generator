package reposource

import (
	"context"
	"errors"
	"fmt"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

// GitCommits reads recent commits from a local working copy using go-git.
type GitCommits struct{}

// Commits returns up to limit commits from HEAD, newest first.
func (GitCommits) Commits(_ context.Context, localPath string, limit int) ([]processing.Commit, error) {
	r, err := git.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	iter, err := r.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	defer iter.Close()

	var commits []processing.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		if limit > 0 && len(commits) >= limit {
			return storer.ErrStop
		}
		commits = append(commits, processing.Commit{
			SHA:     c.Hash.String(),
			Message: c.Message,
			Author:  c.Author.Name,
		})
		return nil
	})
	// A shallow clone only holds the fetched slice of history; walking to a
	// parent commit beyond the shallow boundary yields ErrObjectNotFound. Treat
	// that as the end of available history and return what we collected, rather
	// than failing the whole run.
	if err != nil && !errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, fmt.Errorf("iterate log: %w", err)
	}
	return commits, nil
}
