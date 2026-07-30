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

// DefaultTimeout bounds a single Ollama request. Because Generate is
// non-streaming, the whole completion must arrive within this window — and
// long-form inference on a large model, especially CPU-only, can take many
// minutes. The previous 5-minute ceiling was too low for release blog
// generation (it aborted with "Client.Timeout exceeded while awaiting
// headers"), so the default is generous; override with WithTimeout.
const DefaultTimeout = 15 * time.Minute

// DefaultNumPredict caps the tokens a single completion may generate. Small
// small local models often ignore stop cues and ramble, producing
// huge, slow outputs; this bound keeps each call fast and its output sane.
// Override with WithNumPredict (0 = unbounded, the Ollama default).
const DefaultNumPredict = 2048

// Client talks to a local Ollama server for a fixed model.
type Client struct {
	baseURL    string
	model      string
	http       *http.Client
	retry      retry.Config
	numPredict int
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the server URL (e.g. OLLAMA_BASE_URL, or a test server).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient injects a custom *http.Client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithTimeout overrides the per-request HTTP timeout (e.g. from OLLAMA_TIMEOUT).
// Non-positive values are ignored, keeping DefaultTimeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.http.Timeout = d
		}
	}
}

// WithRetry overrides the retry policy (e.g. faster in tests).
func WithRetry(cfg retry.Config) Option { return func(c *Client) { c.retry = cfg } }

// WithNumPredict overrides the max output tokens per completion. A value <= 0
// removes the cap (the Ollama server default). Use it to allow longer outputs
// (e.g. a full blog via Ollama) or to tighten the cap for faster transforms.
func WithNumPredict(n int) Option { return func(c *Client) { c.numPredict = n } }

// New returns a Client for the given model. Inference on a large model can take
// a while, so the default timeout is generous; transient failures (e.g. the
// model still loading) are retried with backoff.
func New(model string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		model:      model,
		http:       &http.Client{Timeout: DefaultTimeout},
		retry:      retry.Default,
		numPredict: DefaultNumPredict,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

type generateRequest struct {
	Model   string          `json:"model"`
	Prompt  string          `json:"prompt"`
	Stream  bool            `json:"stream"`
	Options *generateOption `json:"options,omitempty"`
}

type generateOption struct {
	NumPredict int `json:"num_predict"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate runs a single non-streaming completion for the prompt and returns
// the generated text. Transient failures are retried with backoff.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	greq := generateRequest{Model: c.model, Prompt: prompt, Stream: false}
	if c.numPredict > 0 {
		greq.Options = &generateOption{NumPredict: c.numPredict}
	}
	raw, _ := json.Marshal(greq)

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
