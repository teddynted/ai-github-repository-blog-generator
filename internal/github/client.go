// Package github is a minimal GitHub REST client covering the operations the
// MVP needs: validating repository access with a PAT and creating a push
// webhook. It uses only the standard library so it is easy to test with
// httptest, and it maps GitHub's HTTP statuses onto typed application errors.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/retry"
)

// DefaultBaseURL is the public GitHub REST API root.
const DefaultBaseURL = "https://api.github.com"

// RepoInfo is the subset of repository fields the platform records.
type RepoInfo struct {
	ID            int64
	FullName      string
	DefaultBranch string
}

// WebhookConfig describes the push webhook to create.
type WebhookConfig struct {
	URL    string
	Secret string
	Events []string
}

// Client talks to the GitHub REST API. Construct it with New.
type Client struct {
	baseURL string
	http    *http.Client
	retry   retry.Config
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API root (used in tests).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient injects a custom *http.Client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithRetry overrides the retry policy (e.g. faster in tests).
func WithRetry(cfg retry.Config) Option { return func(c *Client) { c.retry = cfg } }

// New returns a Client with sensible defaults. Transient failures (network,
// unexpected 5xx) are retried; 401/403/404 fail fast.
func New(opts ...Option) *Client {
	c := &Client{baseURL: DefaultBaseURL, http: &http.Client{Timeout: 15 * time.Second}, retry: retry.Default}
	for _, o := range opts {
		o(c)
	}
	return c
}

// GetRepository validates that the PAT can read the repository and returns its
// core metadata. A 401/403 maps to unauthorized, 404 to not_found.
func (c *Client) GetRepository(ctx context.Context, owner, name, pat string) (RepoInfo, error) {
	var body struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.exec(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", owner, name), pat, nil, http.StatusOK, &body); err != nil {
		return RepoInfo{}, err
	}
	return RepoInfo{ID: body.ID, FullName: body.FullName, DefaultBranch: body.DefaultBranch}, nil
}

// CreateWebhook creates a push webhook and returns its ID. A 403 indicates the
// PAT lacks webhook (admin:repo_hook) permission.
func (c *Client) CreateWebhook(ctx context.Context, owner, name, pat string, cfg WebhookConfig) (int64, error) {
	payload := map[string]any{
		"name":   "web",
		"active": true,
		"events": cfg.Events,
		"config": map[string]any{
			"url":          cfg.URL,
			"content_type": "json",
			"secret":       cfg.Secret,
			"insecure_ssl": "0",
		},
	}
	raw, _ := json.Marshal(payload)
	var body struct {
		ID int64 `json:"id"`
	}
	if err := c.exec(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/hooks", owner, name), pat, raw, http.StatusCreated, &body); err != nil {
		return 0, err
	}
	return body.ID, nil
}

// UpdateWebhook updates an existing webhook's config (used when a repository's
// webhook secret is rotated on re-registration, so GitHub keeps signing with
// the current secret).
func (c *Client) UpdateWebhook(ctx context.Context, owner, name, pat string, hookID int64, cfg WebhookConfig) error {
	payload := map[string]any{
		"active": true,
		"events": cfg.Events,
		"config": map[string]any{
			"url":          cfg.URL,
			"content_type": "json",
			"secret":       cfg.Secret,
			"insecure_ssl": "0",
		},
	}
	raw, _ := json.Marshal(payload)
	path := fmt.Sprintf("/repos/%s/%s/hooks/%d", owner, name, hookID)
	return c.exec(ctx, http.MethodPatch, path, pat, raw, http.StatusOK, nil)
}

// DeleteWebhook removes a repository's webhook. A 404 (already gone) is treated
// as success so deletion is idempotent.
func (c *Client) DeleteWebhook(ctx context.Context, owner, name, pat string, hookID int64) error {
	path := fmt.Sprintf("/repos/%s/%s/hooks/%d", owner, name, hookID)
	err := c.exec(ctx, http.MethodDelete, path, pat, nil, http.StatusNoContent, nil)
	if apperror.CodeOf(err) == apperror.CodeNotFound {
		return nil
	}
	return err
}

// exec builds and executes a request, retrying transient failures. The request
// is rebuilt each attempt so the body reader is fresh.
func (c *Client) exec(ctx context.Context, method, path, pat string, reqBody []byte, wantStatus int, out any) error {
	return retry.Do(ctx, c.retry, func() error {
		req, err := c.newRequest(ctx, method, path, pat, reqBody)
		if err != nil {
			return err
		}
		return c.do(req, wantStatus, out)
	})
}

func (c *Client) newRequest(ctx context.Context, method, path, pat string, body []byte) (*http.Request, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "build github request")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+pat)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// do executes the request, enforces the expected status, and decodes the body
// into out (when non-nil). Non-2xx statuses become typed errors.
func (c *Client) do(req *http.Request, wantStatus int, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return apperror.Wrap(err, apperror.CodeUpstream, "github request failed")
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != wantStatus {
		return statusError(resp.StatusCode)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return apperror.Wrap(err, apperror.CodeUpstream, "decode github response")
		}
	}
	return nil
}

func statusError(code int) error {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return apperror.New(apperror.CodeUnauthorized, "github rejected the token (check access and permissions)")
	case http.StatusNotFound:
		return apperror.New(apperror.CodeNotFound, "repository not found or not accessible with this token")
	default:
		return apperror.New(apperror.CodeUpstream, fmt.Sprintf("unexpected github status %d", code))
	}
}
