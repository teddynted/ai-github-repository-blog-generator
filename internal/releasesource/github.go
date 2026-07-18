// Package releasesource adapts the GitHub REST client to the
// releasecontext.Sources port. It is the I/O layer for the Release Context
// Builder: it fetches repository metadata, the release, the commit/file diff
// between the previous and selected release, and a curated file inventory, and
// maps them into the raw inputs the pure analyzers consume.
package releasesource

import (
	"context"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

// maxContentFiles caps how many text files we fetch content for, bounding the
// number of GitHub API calls per build.
const maxContentFiles = 80

// maxContentBytes skips fetching content for files larger than this.
const maxContentBytes = 512 * 1024

// GitHubClient is the subset of *github.Client the adapter uses (an interface so
// the adapter is unit-testable with a fake).
type GitHubClient interface {
	GetRepositoryDetail(ctx context.Context, owner, name, token string) (github.RepositoryDetail, error)
	GetReleaseByTag(ctx context.Context, owner, name, token, tag string) (github.ReleaseDetail, error)
	ListReleases(ctx context.Context, owner, name, token string) ([]github.ReleaseRef, error)
	CompareCommits(ctx context.Context, owner, name, token, base, head string) ([]github.CommitInfo, []github.ChangedFile, error)
	GetTree(ctx context.Context, owner, name, token, ref string) ([]github.TreeEntry, error)
	GetFileContent(ctx context.Context, owner, name, token, filePath, ref string) (string, error)
}

// GitHubSource implements releasecontext.Sources against the GitHub REST API.
type GitHubSource struct {
	Client GitHubClient
	// Token is the fallback credential; "" works for public repos (rate-limited).
	Token string
	// TokenFor, when set, resolves a repository's credential per request (e.g.
	// the registered PAT from the shared secret), so private repos are read with
	// the same credential used for cloning. It falls back to Token on error.
	TokenFor func(ctx context.Context, repoFullName string) (string, error)

	mu      sync.Mutex
	compare map[string]compareResult // memoized compare per "base...head"
	tokens  map[string]string        // memoized per-repo tokens
}

type compareResult struct {
	commits []github.CommitInfo
	files   []github.ChangedFile
}

var _ rc.Sources = (*GitHubSource)(nil)

// New builds a GitHubSource with a static token. Set TokenFor afterwards for
// per-repository credential resolution.
func New(client GitHubClient, token string) *GitHubSource {
	return &GitHubSource{Client: client, Token: token, compare: map[string]compareResult{}, tokens: map[string]string{}}
}

// token resolves the credential for req's repository, memoized per repo. With no
// TokenFor configured (or on resolver error) it returns the static Token.
func (s *GitHubSource) token(ctx context.Context, req rc.Request) string {
	if s.TokenFor == nil {
		return s.Token
	}
	key := req.FullName()
	s.mu.Lock()
	if t, ok := s.tokens[key]; ok {
		s.mu.Unlock()
		return t
	}
	s.mu.Unlock()

	t, err := s.TokenFor(ctx, key)
	if err != nil || t == "" {
		return s.Token
	}
	s.mu.Lock()
	s.tokens[key] = t
	s.mu.Unlock()
	return t
}

func (s *GitHubSource) RepositoryMeta(ctx context.Context, req rc.Request) (rc.RawRepository, error) {
	d, err := s.Client.GetRepositoryDetail(ctx, req.Owner, req.Repository, s.token(ctx, req))
	if err != nil {
		return rc.RawRepository{}, err
	}
	return rc.RawRepository{
		Owner: req.Owner, Name: req.Repository, FullName: d.FullName, Description: d.Description,
		Topics: d.Topics, Homepage: d.Homepage, License: d.License, Visibility: d.Visibility,
		DefaultBranch: d.DefaultBranch, Language: d.Language, URL: d.HTMLURL,
	}, nil
}

func (s *GitHubSource) ReleaseByTag(ctx context.Context, req rc.Request) (rc.RawRelease, error) {
	d, err := s.Client.GetReleaseByTag(ctx, req.Owner, req.Repository, s.token(ctx, req), req.ReleaseTag)
	if err != nil {
		return rc.RawRelease{}, err
	}
	prev, err := s.previousTag(ctx, req)
	if err != nil {
		return rc.RawRelease{}, err
	}
	rel := rc.RawRelease{
		Tag: d.Tag, Name: d.Name, PublishedAt: d.PublishedAt, Author: d.AuthorLogin,
		Body: d.Body, URL: d.HTMLURL, PreRelease: d.PreRelease, PreviousTag: prev,
	}
	if rel.Tag == "" {
		rel.Tag = req.ReleaseTag
	}
	for _, a := range d.Assets {
		rel.Assets = append(rel.Assets, rc.ReleaseAsset{Name: a.Name, Size: a.Size, ContentType: a.ContentType, URL: a.URL})
	}
	return rel, nil
}

// previousTag returns the tag of the release published immediately before the
// selected one (by published date, newest-first ordering), or "" if none.
func (s *GitHubSource) previousTag(ctx context.Context, req rc.Request) (string, error) {
	releases, err := s.Client.ListReleases(ctx, req.Owner, req.Repository, s.token(ctx, req))
	if err != nil {
		return "", err
	}
	sort.SliceStable(releases, func(i, j int) bool { return releases[i].PublishedAt > releases[j].PublishedAt })
	for i, r := range releases {
		if r.Tag == req.ReleaseTag {
			if i+1 < len(releases) {
				return releases[i+1].Tag, nil
			}
			return "", nil
		}
	}
	return "", nil
}

func (s *GitHubSource) CommitsBetween(ctx context.Context, req rc.Request, previousTag string) ([]rc.RawCommit, error) {
	cr, err := s.compareRange(ctx, req, previousTag)
	if err != nil {
		return nil, err
	}
	out := make([]rc.RawCommit, 0, len(cr.commits))
	for _, c := range cr.commits {
		out = append(out, rc.RawCommit{SHA: c.SHA, Subject: c.Message, Author: c.AuthorName, Date: c.Date, Parents: c.Parents})
	}
	return out, nil
}

func (s *GitHubSource) ChangedFilesBetween(ctx context.Context, req rc.Request, previousTag string) ([]rc.RawChangedFile, error) {
	cr, err := s.compareRange(ctx, req, previousTag)
	if err != nil {
		return nil, err
	}
	out := make([]rc.RawChangedFile, 0, len(cr.files))
	for _, f := range cr.files {
		out = append(out, rc.RawChangedFile{Path: f.Path, Status: f.Status, Additions: f.Additions, Deletions: f.Deletions})
	}
	return out, nil
}

// compareRange fetches (and memoizes) the compare between previousTag and the
// selected release, so CommitsBetween and ChangedFilesBetween share one call.
func (s *GitHubSource) compareRange(ctx context.Context, req rc.Request, previousTag string) (compareResult, error) {
	key := previousTag + "..." + req.ReleaseTag
	s.mu.Lock()
	if cr, ok := s.compare[key]; ok {
		s.mu.Unlock()
		return cr, nil
	}
	s.mu.Unlock()

	commits, files, err := s.Client.CompareCommits(ctx, req.Owner, req.Repository, s.token(ctx, req), previousTag, req.ReleaseTag)
	if err != nil {
		return compareResult{}, err
	}
	cr := compareResult{commits: commits, files: files}
	s.mu.Lock()
	s.compare[key] = cr
	s.mu.Unlock()
	return cr, nil
}

func (s *GitHubSource) Files(ctx context.Context, req rc.Request) ([]rc.RawFile, error) {
	tok := s.token(ctx, req)
	tree, err := s.Client.GetTree(ctx, req.Owner, req.Repository, tok, req.ReleaseTag)
	if err != nil {
		return nil, err
	}
	out := make([]rc.RawFile, 0, len(tree))
	fetched := 0
	for _, e := range tree {
		if e.Type != "blob" || skipPath(e.Path) {
			continue
		}
		f := rc.RawFile{Path: e.Path, Size: e.Size}
		if wantContent(e.Path) && e.Size <= maxContentBytes && fetched < maxContentFiles {
			content, cerr := s.Client.GetFileContent(ctx, req.Owner, req.Repository, tok, e.Path, req.ReleaseTag)
			if cerr != nil && apperror.CodeOf(cerr) != apperror.CodeNotFound {
				return nil, cerr
			}
			f.Content = content
			fetched++
		}
		out = append(out, f)
	}
	return out, nil
}

// wantContent decides which files are worth fetching text for: docs, diagrams,
// IaC templates, and key manifests — exactly what the analyzers read.
func wantContent(p string) bool {
	lp := strings.ToLower(p)
	base := strings.ToLower(path.Base(p))
	ext := strings.ToLower(path.Ext(p))
	switch ext {
	case ".md", ".mmd", ".mdx":
		return true
	}
	if base == "go.mod" || base == "changelog" || base == "readme" {
		return true
	}
	if (ext == ".yaml" || ext == ".yml") && (strings.HasPrefix(lp, "infrastructure/") || strings.Contains(lp, "cloudformation") || strings.Contains(lp, "template")) {
		return true
	}
	return false
}

// skipPath drops vendored, build, and dependency directories from the inventory.
func skipPath(p string) bool {
	lp := strings.ToLower(p)
	for _, pre := range []string{"vendor/", "node_modules/", "dist/", ".git/"} {
		if strings.HasPrefix(lp, pre) || strings.Contains(lp, "/"+pre) {
			return true
		}
	}
	return false
}
