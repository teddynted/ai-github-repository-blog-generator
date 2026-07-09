// Package ollama is a minimal client for a local Ollama server, providing the
// platform's local LLM inference (no external, paid inference API). It uses
// only the standard library and is testable with httptest.
package ollama

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

// DefaultBaseURL is the local Ollama endpoint.
const DefaultBaseURL = "http://localhost:11434"

// Client talks to a local Ollama server for a fixed model.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
	retry   retry.Config
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the server URL (e.g. OLLAMA_BASE_URL, or a test server).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient injects a custom *http.Client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithRetry overrides the retry policy (e.g. faster in tests).
func WithRetry(cfg retry.Config) Option { return func(c *Client) { c.retry = cfg } }

// New returns a Client for the given model. Inference on a large model can take
// a while, so the default timeout is generous; transient failures (e.g. the
// model still loading) are retried with backoff.
func New(model string, opts ...Option) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		model:   model,
		http:    &http.Client{Timeout: 5 * time.Minute},
		retry:   retry.Default,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate runs a single non-streaming completion for the prompt and returns
// the generated text. Transient failures are retried with backoff.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	raw, _ := json.Marshal(generateRequest{Model: c.model, Prompt: prompt, Stream: false})

	var out generateResponse
	err := retry.Do(ctx, c.retry, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(raw))
		if err != nil {
			return apperror.Wrap(err, apperror.CodeInternal, "build ollama request")
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return apperror.Wrap(err, apperror.CodeUpstream, "ollama request failed")
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

		if resp.StatusCode != http.StatusOK {
			return apperror.New(apperror.CodeUpstream, fmt.Sprintf("ollama returned status %d", resp.StatusCode))
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return apperror.Wrap(err, apperror.CodeUpstream, "decode ollama response")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return out.Response, nil
}

// Model returns the configured model name.
func (c *Client) Model() string { return c.model }
