package release

import (
	"context"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
)

// ghAPI is the subset of the GitHub client the release adapter uses. The
// concrete *github.Client satisfies it.
type ghAPI interface {
	ReleaseExistsForTag(ctx context.Context, owner, name, token, tag string) (bool, error)
	CreateRelease(ctx context.Context, owner, name, token string, r github.ReleaseRequest) (string, error)
}

// GitHub adapts the GitHub client to the GitHubReleases port, binding a repo
// (owner/repo) and token.
type GitHub struct {
	API   ghAPI
	Owner string
	Repo  string
	Token string
}

// Authenticated reports whether a token is configured.
func (g *GitHub) Authenticated(context.Context) bool { return g.Token != "" }

// ReleaseExists reports whether a release already exists for tag.
func (g *GitHub) ReleaseExists(ctx context.Context, tag string) (bool, error) {
	return g.API.ReleaseExistsForTag(ctx, g.Owner, g.Repo, g.Token, tag)
}

// CreateRelease publishes a GitHub Release for tag and returns its URL.
func (g *GitHub) CreateRelease(ctx context.Context, tag, name, body string, prerelease bool) (string, error) {
	return g.API.CreateRelease(ctx, g.Owner, g.Repo, g.Token, github.ReleaseRequest{
		Tag: tag, Name: name, Body: body, PreRelease: prerelease,
	})
}

var _ GitHubReleases = (*GitHub)(nil)
