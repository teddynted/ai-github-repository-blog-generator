package publishing

import (
	"context"
	"encoding/base64"
)

// GitHubPublisher publishes Markdown/docs and images to a GitHub repository via
// the Contents API (create/update file). It can publish generated docs, release
// notes, and image assets. https://docs.github.com/rest/repos/contents
type GitHubPublisher struct {
	HTTP    HTTPDoer
	Creds   Credentials
	Owner   string
	Repo    string
	BaseURL string // default https://api.github.com
}

func NewGitHubPublisher(http HTTPDoer, creds Credentials, owner, repo string) *GitHubPublisher {
	return &GitHubPublisher{HTTP: http, Creds: creds, Owner: owner, Repo: repo, BaseURL: "https://api.github.com"}
}

func (p *GitHubPublisher) Name() Platform { return PlatformGitHub }

func (p *GitHubPublisher) Supports(t ContentType) bool {
	switch t {
	case TypeBlog, TypeDiagram, TypeImage, TypeThumbnail:
		return true
	default:
		return false
	}
}

func (p *GitHubPublisher) Validate(c Content, m PlatformMetadata) error {
	if !p.Creds.has(envGitHubToken) {
		return errAuth("GITHUB_TOKEN is not set")
	}
	if p.Owner == "" || p.Repo == "" {
		return errInvalid("GitHub publisher needs an owner and repository")
	}
	if m.Extra["path"] == "" {
		return errInvalid("GitHub publisher needs a target path")
	}
	return nil
}

type ghContentReq struct {
	Message string `json:"message"`
	Content string `json:"content"` // base64
	Branch  string `json:"branch,omitempty"`
	SHA     string `json:"sha,omitempty"`
}

type ghContentResp struct {
	Content struct {
		HTMLURL string `json:"html_url"`
		SHA     string `json:"sha"`
		Path    string `json:"path"`
	} `json:"content"`
}

func (p *GitHubPublisher) Publish(ctx context.Context, c Content, m PlatformMetadata) (PublicationResult, error) {
	path := m.Extra["path"]
	url := p.BaseURL + "/repos/" + p.Owner + "/" + p.Repo + "/contents/" + path
	headers := map[string]string{
		"Authorization": "Bearer " + p.Creds.get(envGitHubToken),
		"Accept":        "application/vnd.github+json",
	}
	// Look up an existing SHA so an update overwrites rather than 409s.
	sha := ""
	var existing struct {
		SHA string `json:"sha"`
	}
	if _, err := doJSON(ctx, p.HTTP, "GET", url+"?ref="+firstNonEmpty(m.Extra["branch"], "main"), headers, nil, &existing); err == nil {
		sha = existing.SHA
	}

	req := ghContentReq{
		Message: m.Extra["commitMessage"],
		Content: base64.StdEncoding.EncodeToString([]byte(m.Body)),
		Branch:  m.Extra["branch"],
		SHA:     sha,
	}
	var out ghContentResp
	_, err := doJSON(ctx, p.HTTP, "PUT", url, headers, req, &out)
	if err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{Platform: PlatformGitHub, PlatformID: out.Content.SHA, URL: out.Content.HTMLURL}, nil
}

func (p *GitHubPublisher) Update(ctx context.Context, id string, c Content, m PlatformMetadata) (PublicationResult, error) {
	return p.Publish(ctx, c, m) // Publish already upserts by path
}

func (p *GitHubPublisher) Delete(ctx context.Context, path string) error {
	url := p.BaseURL + "/repos/" + p.Owner + "/" + p.Repo + "/contents/" + path
	headers := map[string]string{"Authorization": "Bearer " + p.Creds.get(envGitHubToken), "Accept": "application/vnd.github+json"}
	var existing struct {
		SHA string `json:"sha"`
	}
	if _, err := doJSON(ctx, p.HTTP, "GET", url, headers, nil, &existing); err != nil {
		return err
	}
	_, err := doJSON(ctx, p.HTTP, "DELETE", url, headers, map[string]string{"message": "docs: remove " + path, "sha": existing.SHA}, nil)
	return err
}

func (p *GitHubPublisher) GetStatus(ctx context.Context, path string) (string, error) {
	url := p.BaseURL + "/repos/" + p.Owner + "/" + p.Repo + "/contents/" + path
	var out struct {
		SHA string `json:"sha"`
	}
	if _, err := doJSON(ctx, p.HTTP, "GET", url, map[string]string{"Authorization": "Bearer " + p.Creds.get(envGitHubToken)}, nil, &out); err != nil {
		return "", err
	}
	if out.SHA != "" {
		return "published", nil
	}
	return "absent", nil
}
