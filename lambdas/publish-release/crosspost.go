package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// crosspost publishes the blog to any configured external platforms (Dev.to,
// Hashnode, Medium). Each is optional (skipped when its token is empty) and
// best-effort — a platform failure is recorded, never fatal, since the PR +
// S3 promote already succeeded. Returns a map of platform -> published URL.
func (e *env) crosspost(ctx context.Context, title, body string, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
}) map[string]string {
	out := map[string]string{}
	try := func(name string, fn func() (string, error)) {
		if fn == nil {
			return
		}
		url, err := fn()
		if err != nil {
			logger.Warn("crosspost failed", "platform", name, "error", err.Error())
			return
		}
		if url != "" {
			out[name] = url
			logger.Info("crossposted", "platform", name, "url", url)
		}
	}
	if e.devtoToken != "" {
		try("devto", func() (string, error) { return e.postDevto(ctx, title, body) })
	}
	if e.hashnodeToken != "" && e.hashnodePub != "" {
		try("hashnode", func() (string, error) { return e.postHashnode(ctx, title, body) })
	}
	if e.mediumToken != "" {
		try("medium", func() (string, error) { return e.postMedium(ctx, title, body) })
	}
	return out
}

// splitTitle pulls the first H1 as the title and returns the body without it, so
// platforms that take a separate title field don't render it twice.
func splitTitle(md string) (title, body string) {
	lines := strings.Split(md, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(t, "# "))
			body = strings.TrimLeft(strings.Join(append(append([]string{}, lines[:i]...), lines[i+1:]...), "\n"), "\n")
			return title, body
		}
	}
	return "Release blog", md
}

// --- Dev.to (Forem) ---------------------------------------------------------
// POST https://dev.to/api/articles  (header "api-key")
func (e *env) postDevto(ctx context.Context, title, body string) (string, error) {
	payload := map[string]any{"article": map[string]any{
		"title":         title,
		"body_markdown": body,
		"published":     !e.publishDraft,
	}}
	if e.canonical != "" {
		payload["article"].(map[string]any)["canonical_url"] = e.canonical
	}
	if len(e.tags) > 0 {
		payload["article"].(map[string]any)["tags"] = e.tags
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://dev.to/api/articles", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("api-key", e.devtoToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.forem.api-v1+json")
	var res struct {
		URL string `json:"url"`
	}
	if err := e.doJSON(req, &res); err != nil {
		return "", err
	}
	return res.URL, nil
}

// --- Hashnode (GraphQL) -----------------------------------------------------
// POST https://gql.hashnode.com/  publishPost mutation (Authorization: <token>)
func (e *env) postHashnode(ctx context.Context, title, body string) (string, error) {
	const q = `mutation($input: PublishPostInput!){ publishPost(input:$input){ post{ url } } }`
	input := map[string]any{
		"title":           title,
		"contentMarkdown": body,
		"publicationId":   e.hashnodePub,
	}
	if e.canonical != "" {
		input["originalArticleURL"] = e.canonical
	}
	b, _ := json.Marshal(map[string]any{"query": q, "variables": map[string]any{"input": input}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://gql.hashnode.com/", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", e.hashnodeToken)
	req.Header.Set("Content-Type", "application/json")
	var res struct {
		Data struct {
			PublishPost struct {
				Post struct {
					URL string `json:"url"`
				} `json:"post"`
			} `json:"publishPost"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := e.doJSON(req, &res); err != nil {
		return "", err
	}
	if len(res.Errors) > 0 {
		return "", fmt.Errorf("hashnode: %s", res.Errors[0].Message)
	}
	return res.Data.PublishPost.Post.URL, nil
}

// --- Medium (legacy integration token; API is otherwise retired) ------------
// POST https://api.medium.com/v1/users/{id}/posts  (Authorization: Bearer)
func (e *env) postMedium(ctx context.Context, title, body string) (string, error) {
	// resolve the author id
	me, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.medium.com/v1/me", nil)
	if err != nil {
		return "", err
	}
	me.Header.Set("Authorization", "Bearer "+e.mediumToken)
	var meRes struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := e.doJSON(me, &meRes); err != nil {
		return "", err
	}
	if meRes.Data.ID == "" {
		return "", fmt.Errorf("medium: could not resolve user id (token likely revoked)")
	}
	status := "public"
	if e.publishDraft {
		status = "draft"
	}
	payload := map[string]any{"title": title, "contentFormat": "markdown", "content": "# " + title + "\n\n" + body, "publishStatus": status}
	if e.canonical != "" {
		payload["canonicalUrl"] = e.canonical
	}
	if len(e.tags) > 0 {
		payload["tags"] = e.tags
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.medium.com/v1/users/"+meRes.Data.ID+"/posts", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+e.mediumToken)
	req.Header.Set("Content-Type", "application/json")
	var res struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := e.doJSON(req, &res); err != nil {
		return "", err
	}
	return res.Data.URL, nil
}

func (e *env) doJSON(req *http.Request, out any) error {
	resp, err := e.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %d: %s", req.Method, req.URL.Host, resp.StatusCode, string(raw))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}
