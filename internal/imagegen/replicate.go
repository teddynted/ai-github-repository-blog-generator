package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultReplicateModel is the Replicate model run by default: FLUX.1 [schnell]
// — fast and inexpensive, good for scene backgrounds. Override with
// REPLICATE_IMAGE_MODEL (e.g. "stability-ai/sdxl") to switch models.
const DefaultReplicateModel = "black-forest-labs/flux-schnell"

// replicateBaseURL is the Replicate API root; overridable in tests.
const replicateBaseURL = "https://api.replicate.com/v1"

// replicatePollTimeout bounds how long Generate waits for an async prediction to
// finish after the initial "Prefer: wait" request (fast models usually complete
// within that request; a slow one is polled up to here).
const replicatePollTimeout = 90 * time.Second

// ReplicateClient generates images through Replicate's HTTP API, implementing
// the same Generator port as the Bedrock client so it drops into the renderer
// unchanged. It bypasses the account's (0-RPM, non-adjustable) Bedrock image
// quota entirely.
type ReplicateClient struct {
	http    *http.Client
	baseURL string
	token   string
	model   string
	sleep   func(time.Duration)
}

// NewReplicate builds a client for the given API token and model
// ("owner/name"). A blank model uses DefaultReplicateModel.
func NewReplicate(token, model string) *ReplicateClient {
	if strings.TrimSpace(model) == "" {
		model = DefaultReplicateModel
	}
	return &ReplicateClient{
		http:    &http.Client{Timeout: 2 * time.Minute},
		baseURL: replicateBaseURL,
		token:   token,
		model:   model,
		sleep:   time.Sleep,
	}
}

// prediction is the subset of a Replicate prediction we read.
type prediction struct {
	Status string          `json:"status"` // starting|processing|succeeded|failed|canceled
	Output json.RawMessage `json:"output"` // string OR []string of result URLs
	Error  string          `json:"error"`
	URLs   struct {
		Get string `json:"get"`
	} `json:"urls"`
}

// Generate renders spec into image bytes: it creates a prediction (asking the
// API to wait for completion), polls if the model is still running, then fetches
// the first output URL. Transient 429/5xx responses are retried with backoff.
func (c *ReplicateClient) Generate(ctx context.Context, spec Spec) ([]byte, error) {
	if strings.TrimSpace(spec.Prompt) == "" {
		return nil, errors.New("imagegen: empty prompt")
	}
	if strings.TrimSpace(c.token) == "" {
		return nil, errors.New("imagegen: missing Replicate API token")
	}
	input := map[string]any{
		"prompt":        truncate(spec.Prompt, 2000),
		"aspect_ratio":  aspectFor(spec.Width, spec.Height),
		"output_format": "png",
		"num_outputs":   1,
		"seed":          spec.Seed,
	}
	if n := strings.TrimSpace(spec.NegativePrompt); n != "" {
		input["negative_prompt"] = truncate(n, 2000)
	}
	body, err := json.Marshal(map[string]any{"input": input})
	if err != nil {
		return nil, fmt.Errorf("imagegen: marshal: %w", err)
	}

	pred, err := c.createPrediction(ctx, body)
	if err != nil {
		return nil, err
	}
	pred, err = c.awaitTerminal(ctx, pred)
	if err != nil {
		return nil, err
	}
	if pred.Status != "succeeded" {
		return nil, fmt.Errorf("imagegen: prediction %s: %s", pred.Status, firstNonEmptyStr(pred.Error, "no error detail"))
	}
	url, err := firstOutputURL(pred.Output)
	if err != nil {
		return nil, err
	}
	return c.fetch(ctx, url)
}

// createPrediction POSTs the run request with Prefer: wait so fast models return
// a completed prediction in one round-trip.
func (c *ReplicateClient) createPrediction(ctx context.Context, body []byte) (prediction, error) {
	url := c.baseURL + "/models/" + c.model + "/predictions"
	var pred prediction
	err := c.doJSON(ctx, http.MethodPost, url, body, map[string]string{"Prefer": "wait"}, &pred)
	return pred, err
}

// awaitTerminal polls the prediction's status URL until it reaches a terminal
// state or the poll budget/context expires.
func (c *ReplicateClient) awaitTerminal(ctx context.Context, pred prediction) (prediction, error) {
	deadline := time.Now().Add(replicatePollTimeout)
	for !isTerminal(pred.Status) {
		if pred.URLs.Get == "" {
			return pred, errors.New("imagegen: prediction not terminal and no poll URL")
		}
		if time.Now().After(deadline) {
			return pred, errors.New("imagegen: timed out waiting for prediction")
		}
		c.sleep(time.Second)
		var next prediction
		if err := c.doJSON(ctx, http.MethodGet, pred.URLs.Get, nil, nil, &next); err != nil {
			return pred, err
		}
		pred = next
	}
	return pred, nil
}

// doJSON performs an HTTP request with auth + retry on throttling/5xx and decodes
// a JSON response into out (out may be nil to discard the body).
func (c *ReplicateClient) doJSON(ctx context.Context, method, url string, body []byte, extraHeaders map[string]string, out any) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			c.sleep(time.Duration(attempt) * time.Second)
		}
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rdr)
		if err != nil {
			return fmt.Errorf("imagegen: request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue // network hiccup — retry
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("imagegen: http %d: %s", resp.StatusCode, snippet(data))
			continue // throttled / server error — retry
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("imagegen: http %d: %s", resp.StatusCode, snippet(data))
		}
		if out != nil {
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("imagegen: decode: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("imagegen: request failed after %d attempts: %w", maxRetries, lastErr)
}

// fetch downloads the generated image bytes from a Replicate delivery URL. The
// delivery host serves the file directly, so no auth header is sent.
func (c *ReplicateClient) fetch(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			c.sleep(time.Duration(attempt) * time.Second)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("imagegen: fetch request: %w", err)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("imagegen: fetch http %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("imagegen: fetch http %d", resp.StatusCode)
		}
		if len(data) == 0 {
			return nil, errors.New("imagegen: empty image body")
		}
		return data, nil
	}
	return nil, fmt.Errorf("imagegen: fetch failed after %d attempts: %w", maxRetries, lastErr)
}

// firstOutputURL extracts the first result URL from a Replicate output, which is
// either a bare string or an array of strings depending on the model.
func firstOutputURL(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("imagegen: prediction had no output")
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil && one != "" {
		return one, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		for _, u := range many {
			if u != "" {
				return u, nil
			}
		}
	}
	return "", errors.New("imagegen: no usable output URL")
}

func isTerminal(status string) bool {
	switch status {
	case "succeeded", "failed", "canceled":
		return true
	}
	return false
}

// aspectFor maps pixel dimensions to a Replicate aspect_ratio string.
func aspectFor(w, h int) string {
	if w >= h {
		return "16:9"
	}
	return "9:16"
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func snippet(b []byte) string { return truncate(strings.TrimSpace(string(b)), 300) }

// NewFromEnv selects the image backend from the environment: a Replicate client
// when REPLICATE_API_TOKEN is set (model from REPLICATE_IMAGE_MODEL, default
// FLUX.1 [schnell]), otherwise the Bedrock Nova Canvas client. This lets the
// renderer switch providers by configuration alone.
func NewFromEnv(ctx context.Context) (Generator, error) {
	if token := strings.TrimSpace(os.Getenv("REPLICATE_API_TOKEN")); token != "" {
		return NewReplicate(token, os.Getenv("REPLICATE_IMAGE_MODEL")), nil
	}
	return New(ctx)
}
