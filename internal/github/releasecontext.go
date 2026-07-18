package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// These read-only methods back the Release Context Builder (Milestone 2). They
// use the same retrying exec path as the rest of the client and map GitHub
// statuses onto typed application errors.

// ReleaseDetail is a GitHub Release with the fields the context builder needs.
type ReleaseDetail struct {
	Tag         string
	Name        string
	Body        string
	PublishedAt string
	AuthorLogin string
	HTMLURL     string
	PreRelease  bool
	Assets      []ReleaseAssetDetail
}

// ReleaseAssetDetail is a downloadable asset attached to a release.
type ReleaseAssetDetail struct {
	Name        string
	Size        int64
	ContentType string
	URL         string
}

// ReleaseRef is a lightweight release entry used to find the previous tag.
type ReleaseRef struct {
	Tag         string
	PublishedAt string
	PreRelease  bool
}

// CommitInfo is a commit from the compare API.
type CommitInfo struct {
	SHA        string
	Message    string
	AuthorName string
	Date       string
	Parents    int
}

// ChangedFile is a file entry from the compare API.
type ChangedFile struct {
	Path      string
	Status    string
	Additions int
	Deletions int
}

// TreeEntry is one node in a recursive git tree.
type TreeEntry struct {
	Path string
	Size int64
	Type string // "blob" | "tree"
}

// RepositoryDetail is the repository metadata the context builder records.
type RepositoryDetail struct {
	FullName      string
	Description   string
	Topics        []string
	Homepage      string
	License       string
	Visibility    string
	DefaultBranch string
	Language      string
	HTMLURL       string
}

// GetRepositoryDetail fetches full repository metadata (topics included).
func (c *Client) GetRepositoryDetail(ctx context.Context, owner, name, token string) (RepositoryDetail, error) {
	var body struct {
		FullName      string   `json:"full_name"`
		Description   string   `json:"description"`
		Topics        []string `json:"topics"`
		Homepage      string   `json:"homepage"`
		Visibility    string   `json:"visibility"`
		DefaultBranch string   `json:"default_branch"`
		Language      string   `json:"language"`
		HTMLURL       string   `json:"html_url"`
		Private       bool     `json:"private"`
		License       *struct {
			SPDXID string `json:"spdx_id"`
			Name   string `json:"name"`
		} `json:"license"`
	}
	path := fmt.Sprintf("/repos/%s/%s", owner, name)
	if err := c.exec(ctx, http.MethodGet, path, token, nil, http.StatusOK, &body); err != nil {
		return RepositoryDetail{}, err
	}
	d := RepositoryDetail{
		FullName: body.FullName, Description: body.Description, Topics: body.Topics,
		Homepage: body.Homepage, Visibility: body.Visibility, DefaultBranch: body.DefaultBranch,
		Language: body.Language, HTMLURL: body.HTMLURL,
	}
	if d.Visibility == "" { // older API shape
		if body.Private {
			d.Visibility = "private"
		} else {
			d.Visibility = "public"
		}
	}
	if body.License != nil {
		d.License = body.License.SPDXID
		if d.License == "" || d.License == "NOASSERTION" {
			d.License = body.License.Name
		}
	}
	return d, nil
}

// GetReleaseByTag fetches a published release by tag.
func (c *Client) GetReleaseByTag(ctx context.Context, owner, name, token, tag string) (ReleaseDetail, error) {
	var body struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		HTMLURL     string `json:"html_url"`
		PreRelease  bool   `json:"prerelease"`
		Author      struct {
			Login string `json:"login"`
		} `json:"author"`
		Assets []struct {
			Name        string `json:"name"`
			Size        int64  `json:"size"`
			ContentType string `json:"content_type"`
			URL         string `json:"browser_download_url"`
		} `json:"assets"`
	}
	path := fmt.Sprintf("/repos/%s/%s/releases/tags/%s", owner, name, url.PathEscape(tag))
	if err := c.exec(ctx, http.MethodGet, path, token, nil, http.StatusOK, &body); err != nil {
		return ReleaseDetail{}, err
	}
	r := ReleaseDetail{
		Tag: body.TagName, Name: body.Name, Body: body.Body, PublishedAt: body.PublishedAt,
		AuthorLogin: body.Author.Login, HTMLURL: body.HTMLURL, PreRelease: body.PreRelease,
	}
	for _, a := range body.Assets {
		r.Assets = append(r.Assets, ReleaseAssetDetail{Name: a.Name, Size: a.Size, ContentType: a.ContentType, URL: a.URL})
	}
	return r, nil
}

// ListReleases returns up to 100 releases (newest first) for previous-tag
// resolution.
func (c *Client) ListReleases(ctx context.Context, owner, name, token string) ([]ReleaseRef, error) {
	var body []struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		PreRelease  bool   `json:"prerelease"`
		Draft       bool   `json:"draft"`
	}
	path := fmt.Sprintf("/repos/%s/%s/releases?per_page=100", owner, name)
	if err := c.exec(ctx, http.MethodGet, path, token, nil, http.StatusOK, &body); err != nil {
		return nil, err
	}
	out := make([]ReleaseRef, 0, len(body))
	for _, r := range body {
		if r.Draft {
			continue
		}
		out = append(out, ReleaseRef{Tag: r.TagName, PublishedAt: r.PublishedAt, PreRelease: r.PreRelease})
	}
	return out, nil
}

// CompareCommits returns the commits and changed files in base...head. A base
// of "" compares the head against the empty tree (the whole history), which
// GitHub does not support directly, so callers pass the repo's first commit or
// use an empty base to signal "list head only" — here an empty base returns
// (nil, nil) so the caller can fall back to a plain commit listing.
func (c *Client) CompareCommits(ctx context.Context, owner, name, token, base, head string) ([]CommitInfo, []ChangedFile, error) {
	if base == "" {
		return nil, nil, nil
	}
	var body struct {
		Commits []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name string `json:"name"`
					Date string `json:"date"`
				} `json:"author"`
			} `json:"commit"`
			Parents []struct {
				SHA string `json:"sha"`
			} `json:"parents"`
		} `json:"commits"`
		Files []struct {
			Filename  string `json:"filename"`
			Status    string `json:"status"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
		} `json:"files"`
	}
	path := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", owner, name, url.PathEscape(base), url.PathEscape(head))
	if err := c.exec(ctx, http.MethodGet, path, token, nil, http.StatusOK, &body); err != nil {
		return nil, nil, err
	}
	commits := make([]CommitInfo, 0, len(body.Commits))
	for _, cm := range body.Commits {
		commits = append(commits, CommitInfo{
			SHA:        cm.SHA,
			Message:    firstLine(cm.Commit.Message),
			AuthorName: cm.Commit.Author.Name,
			Date:       cm.Commit.Author.Date,
			Parents:    len(cm.Parents),
		})
	}
	files := make([]ChangedFile, 0, len(body.Files))
	for _, f := range body.Files {
		files = append(files, ChangedFile{Path: f.Filename, Status: f.Status, Additions: f.Additions, Deletions: f.Deletions})
	}
	return commits, files, nil
}

// GetTree returns the recursive git tree at ref (blobs and subtrees).
func (c *Client) GetTree(ctx context.Context, owner, name, token, ref string) ([]TreeEntry, error) {
	var body struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			Size int64  `json:"size"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	path := fmt.Sprintf("/repos/%s/%s/git/trees/%s?recursive=1", owner, name, url.PathEscape(ref))
	if err := c.exec(ctx, http.MethodGet, path, token, nil, http.StatusOK, &body); err != nil {
		return nil, err
	}
	out := make([]TreeEntry, 0, len(body.Tree))
	for _, e := range body.Tree {
		out = append(out, TreeEntry{Path: e.Path, Type: e.Type, Size: e.Size})
	}
	return out, nil
}

// GetFileContent returns the decoded UTF-8 content of a file at ref. A 404 maps
// to not_found so callers can skip missing files.
func (c *Client) GetFileContent(ctx context.Context, owner, name, token, filePath, ref string) (string, error) {
	var body struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	path := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, name, encodePath(filePath), url.QueryEscape(ref))
	if err := c.exec(ctx, http.MethodGet, path, token, nil, http.StatusOK, &body); err != nil {
		return "", err
	}
	if body.Encoding == "base64" {
		raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(body.Content, "\n", ""))
		if err != nil {
			return "", apperror.Wrap(err, apperror.CodeUpstream, "decode file content")
		}
		return string(raw), nil
	}
	return body.Content, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// encodePath percent-encodes each path segment but keeps the slashes.
func encodePath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}
