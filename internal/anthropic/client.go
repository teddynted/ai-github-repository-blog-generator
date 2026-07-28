// Package anthropic is a technical-writer model backed by Anthropic's own API
// (api.anthropic.com), an alternative to Bedrock that needs only an API key —
// no AWS model-access authorization. It implements the same generate port
// (Generate(ctx, prompt) -> string) every content generator depends on, so it
// drops in wherever the Bedrock or Ollama writer went.
//
// The API key is a credential: it is supplied by the caller (resolved from
// Secrets Manager in production, never hard-coded), held in memory only, and
// never logged — the client redacts it from its own String/error output.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is Anthropic's API endpoint.
	DefaultBaseURL = "https://api.anthropic.com"
	// DefaultModel is a strong, cost-effective default for long-form technical
	// writing. Override with Config.Model / ANTHROPIC_MODEL.
	DefaultModel = "claude-sonnet-4-5"
	// apiVersion is the pinned Anthropic API version header.
	apiVersion = "2023-06-01"
)

// doer is the minimal HTTP surface the client needs, so tests inject a stub or
// an httptest server without a real network call.
type doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config parameterises the client. Zero values are filled with defaults by New.
type Config struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	// System is the writer-persona system prompt applied to every request.
	System string
	// BaseURL overrides the endpoint (tests); defaults to DefaultBaseURL.
	BaseURL string
	// HTTP overrides the HTTP client (tests); defaults to a 5-minute-timeout client.
	HTTP doer
}

// Client generates text with Claude via the Anthropic API.
type Client struct {
	http        doer
	baseURL     string
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	system      string
}

// New builds a Client. It returns an error when no API key is supplied, so a
// misconfiguration fails fast rather than at first call.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("anthropic: API key is required")
	}
	c := &Client{
		http:        cfg.HTTP,
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		system:      cfg.System,
	}
	if c.baseURL == "" {
		c.baseURL = DefaultBaseURL
	}
	if c.model == "" {
		c.model = DefaultModel
	}
	if c.maxTokens <= 0 {
		// 8192 leaves ample headroom for a full ~2,500-word article plus its
		// Conclusion; 4096 truncated long articles mid-section (the model never
		// reached the Conclusion, failing validation). max_tokens is an upper
		// bound — only tokens actually generated are billed.
		c.maxTokens = 8192
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: 5 * time.Minute}
	}
	return c, nil
}

type requestBody struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"`
	Messages    []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseBody struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Error      *apiError      `json:"error,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Generate sends a single-turn prompt to Claude and returns the text.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	if c == nil || c.http == nil {
		return "", errors.New("anthropic: client not initialised")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("anthropic: empty prompt")
	}

	body, err := json.Marshal(requestBody{
		Model:       c.model,
		MaxTokens:   c.maxTokens,
		Temperature: c.temperature,
		System:      c.system,
		Messages:    []message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("anthropic: build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("anthropic: read response: %w", err)
	}

	var out responseBody
	if uerr := json.Unmarshal(raw, &out); uerr != nil {
		// On a non-2xx with unparseable body, surface the status, not the body
		// (which may echo request content).
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("anthropic: %s: unexpected response", resp.Status)
		}
		return "", fmt.Errorf("anthropic: decode response: %w", uerr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if out.Error != nil {
			return "", fmt.Errorf("anthropic: %s: %s: %s", resp.Status, out.Error.Type, out.Error.Message)
		}
		return "", fmt.Errorf("anthropic: %s", resp.Status)
	}

	text := joinText(out.Content)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("anthropic: empty completion (stop_reason=%q)", out.StopReason)
	}
	return text, nil
}

func joinText(blocks []contentBlock) string {
	var b strings.Builder
	for _, blk := range blocks {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}

// String redacts the API key so the client never leaks it through fmt.
func (c *Client) String() string {
	return fmt.Sprintf("anthropic.Client{model:%s, key:REDACTED}", c.model)
}
