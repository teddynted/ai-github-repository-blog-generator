// Package reposource provides real implementations of the processing ports:
// a git cloner (go-git), filesystem README/docs retrieval, and a git commit
// reader. It replaces the placeholder processor for actual repository content.
package reposource

import (
	"context"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
)

// RepoLookup resolves repository metadata. *metadata.Store satisfies it.
type RepoLookup interface {
	Get(ctx context.Context, fullName string) (repo.Repository, bool, error)
}

// PATGetter reads a repository's PAT by reference. *secrets.Store satisfies it.
type PATGetter interface {
	PAT(ctx context.Context, ref string) (string, error)
}

// TokenSource returns the clone credential for a repository.
type TokenSource interface {
	Token(ctx context.Context, repoFullName string) (string, error)
}

// MetaTokenSource resolves a repository's PAT via its metadata secret reference.
type MetaTokenSource struct {
	Meta    RepoLookup
	Secrets PATGetter
}

// Token confirms the repository is registered, then returns its PAT from the
// shared secret, keyed by the repository full name ("<owner>/<name>").
func (m *MetaTokenSource) Token(ctx context.Context, repoFullName string) (string, error) {
	_, ok, err := m.Meta.Get(ctx, repoFullName)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", apperror.New(apperror.CodeNotFound, "repository is not registered")
	}
	return m.Secrets.PAT(ctx, repoFullName)
}
