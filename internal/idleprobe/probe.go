// Package idleprobe provides application-level activity checks for the idle
// detector: it asks n8n and Ollama directly whether they are currently doing
// work. Every probe fails safe — an unreachable or malformed response counts as
// BUSY, so a network blip never contributes to a wrongful stop.
package idleprobe

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// HTTPProbe checks one HTTP endpoint and interprets its JSON body as busy/idle.
type HTTPProbe struct {
	name    string
	url     string
	headers map[string]string
	// busy reports whether the decoded body indicates active work.
	busy   func(body []byte) (bool, error)
	client *http.Client
	logger *slog.Logger
}

// Name identifies the probe (e.g. "n8n", "ollama").
func (p *HTTPProbe) Name() string { return p.name }

// Busy performs the check. On any error it returns true (fail-safe).
func (p *HTTPProbe) Busy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		p.warn("build request failed; treating as busy", err)
		return true
	}
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		p.warn("request failed; treating as busy", err)
		return true
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		p.warn("read failed; treating as busy", err)
		return true
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if p.logger != nil {
			p.logger.Warn("probe non-2xx; treating as busy", "probe", p.name, "status", resp.StatusCode)
		}
		return true
	}
	busy, err := p.busy(body)
	if err != nil {
		p.warn("decode failed; treating as busy", err)
		return true
	}
	return busy
}

func (p *HTTPProbe) warn(msg string, err error) {
	if p.logger != nil {
		p.logger.Warn(msg, "probe", p.name, "error", err.Error())
	}
}

func newClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

// N8N builds a probe that reports busy when n8n has a running execution, via the
// public API: GET {baseURL}/api/v1/executions?status=running&limit=1.
func N8N(baseURL, apiKey string, timeout time.Duration, logger *slog.Logger) *HTTPProbe {
	return &HTTPProbe{
		name:    "n8n",
		url:     baseURL + "/api/v1/executions?status=running&limit=1",
		headers: map[string]string{"X-N8N-API-KEY": apiKey, "Accept": "application/json"},
		client:  newClient(timeout),
		logger:  logger,
		busy: func(body []byte) (bool, error) {
			var r struct {
				Data []json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(body, &r); err != nil {
				return false, err
			}
			return len(r.Data) > 0, nil
		},
	}
}

// Ollama builds a probe that reports busy when a model is resident in memory
// (recently used), via GET {baseURL}/api/ps. A genuinely idle Ollama unloads
// its models after keep_alive, so a loaded model is a sound recent-activity signal.
func Ollama(baseURL string, timeout time.Duration, logger *slog.Logger) *HTTPProbe {
	return &HTTPProbe{
		name:   "ollama",
		url:    baseURL + "/api/ps",
		client: newClient(timeout),
		logger: logger,
		busy: func(body []byte) (bool, error) {
			var r struct {
				Models []json.RawMessage `json:"models"`
			}
			if err := json.Unmarshal(body, &r); err != nil {
				return false, err
			}
			return len(r.Models) > 0, nil
		},
	}
}
