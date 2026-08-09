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
	version string // resolved once for versioned models (e.g. SDXL)
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
	pred, err := c.createPrediction(ctx, spec)
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

// createPrediction builds the model-appropriate input and POSTs the run request
// with Prefer: wait so fast models return a completed prediction in one
// round-trip. Two Replicate shapes are supported:
//
//   - Official models (FLUX.1 [schnell]) run at /models/<owner>/<name>/predictions
//     and take an aspect_ratio; they IGNORE negative_prompt.
//   - Versioned/community models (stability-ai/sdxl) run at /predictions with a
//     resolved version id and explicit width/height, and DO honour
//     negative_prompt — which is how we suppress hallucinated in-image text.
func (c *ReplicateClient) createPrediction(ctx context.Context, spec Spec) (prediction, error) {
	var pred prediction
	if c.usesVersionedEndpoint() {
		version, err := c.resolveVersion(ctx)
		if err != nil {
			return pred, err
		}
		input := map[string]any{
			"prompt":      truncate(spec.Prompt, 2000),
			"width":       snapDim(spec.Width),
			"height":      snapDim(spec.Height),
			"num_outputs": 1,
			"seed":        spec.Seed,
		}
		if n := strings.TrimSpace(spec.NegativePrompt); n != "" {
			input["negative_prompt"] = truncate(n, 2000)
		}
		body, err := json.Marshal(map[string]any{"version": version, "input": input})
		if err != nil {
			return pred, fmt.Errorf("imagegen: marshal: %w", err)
		}
		err = c.doJSON(ctx, http.MethodPost, c.baseURL+"/predictions", body, map[string]string{"Prefer": "wait"}, &pred)
		return pred, err
	}

	input := map[string]any{
		"prompt":        truncate(spec.Prompt, 2000),
		"aspect_ratio":  aspectFor(spec.Width, spec.Height),
		"output_format": "png",
		"num_outputs":   1,
		"seed":          spec.Seed,
	}
	body, err := json.Marshal(map[string]any{"input": input})
	if err != nil {
		return pred, fmt.Errorf("imagegen: marshal: %w", err)
	}
	err = c.doJSON(ctx, http.MethodPost, c.baseURL+"/models/"+c.modelName()+"/predictions", body, map[string]string{"Prefer": "wait"}, &pred)
	return pred, err
}

// usesVersionedEndpoint reports whether the configured model must run through the
// versioned /predictions endpoint (community models like SDXL) rather than the
// official /models/<owner>/<name>/predictions endpoint. A model pinned as
// "owner/name:version" or any SDXL-family model uses the versioned path.
func (c *ReplicateClient) usesVersionedEndpoint() bool {
	return strings.Contains(c.model, ":") || strings.Contains(strings.ToLower(c.model), "sdxl")
}

// modelName strips any ":version" suffix, leaving "owner/name".
func (c *ReplicateClient) modelName() string {
	if i := strings.IndexByte(c.model, ':'); i >= 0 {
		return c.model[:i]
	}
	return c.model
}

// resolveVersion returns the version id for the configured model: the pinned
// suffix when the model is "owner/name:version", otherwise the model's latest
// version fetched once from the API and cached.
func (c *ReplicateClient) resolveVersion(ctx context.Context) (string, error) {
	if i := strings.IndexByte(c.model, ':'); i >= 0 {
		return c.model[i+1:], nil
	}
	if c.version != "" {
		return c.version, nil
	}
	var meta struct {
		LatestVersion struct {
			ID string `json:"id"`
		} `json:"latest_version"`
	}
	if err := c.doJSON(ctx, http.MethodGet, c.baseURL+"/models/"+c.modelName(), nil, nil, &meta); err != nil {
		return "", fmt.Errorf("imagegen: resolve version for %s: %w", c.modelName(), err)
	}
	if meta.LatestVersion.ID == "" {
		return "", fmt.Errorf("imagegen: no latest version for %s", c.modelName())
	}
	c.version = meta.LatestVersion.ID
	return c.version, nil
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
			c.sleep(backoffDur(attempt))
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
			c.sleep(backoffDur(attempt))
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

// backoffDur is the exponential backoff before retry attempt n (1-based):
// 2, 4, 8, 16, 32… seconds, capped at 45s — long enough for a 429 rate-limit
// window to clear during a bursty multi-format render.
func backoffDur(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > 45*time.Second {
		d = 45 * time.Second
	}
	return d
}

// aspectFor maps pixel dimensions to a Replicate aspect_ratio string.
func aspectFor(w, h int) string {
	if w >= h {
		return "16:9"
	}
	return "9:16"
}

// snapDim rounds a dimension to a multiple of 8 (SDXL requires this) and clamps
// it to a sane [512, 1536] range so the versioned models accept it.
func snapDim(d int) int {
	if d <= 0 {
		d = 1024
	}
	d = (d / 8) * 8
	if d < 512 {
		d = 512
	}
	if d > 1536 {
		d = 1536
	}
	return d
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
