// Package aicache is a local, on-disk response cache for the content-generation
// model port. It wraps any Model (Anthropic, Bedrock, Ollama) so that identical
// requests during local prompt iteration reuse a stored response instead of
// paying to call the provider again.
//
// The cache key is a hash of everything that affects a response — a run-level
// fingerprint (provider, model, system prompt, temperature) plus the per-call
// user prompt — so changing any of them misses the cache and regenerates. It is
// intended for local development only; production never wires it.
package aicache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
)

// Model is the inference port (matches the platform-wide generate contract).
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// entry is the on-disk cache record. Prompt is stored for human inspection of
// the .cache directory; Response is what is replayed.
type entry struct {
	Fingerprint string `json:"fingerprint"`
	Prompt      string `json:"prompt"`
	Response    string `json:"response"`
}

// Cache wraps a Model with a content-addressed on-disk cache under Dir. The
// Fingerprint captures the run-level inputs (provider|model|system|temperature)
// so two runs with different settings never collide.
type Cache struct {
	Inner       Model
	Dir         string
	Fingerprint string
	hits        int64
	misses      int64
}

// New returns a Cache. fingerprintParts are the run-level inputs that, together
// with each prompt, determine a response (e.g. provider, model, system prompt,
// temperature).
func New(inner Model, dir string, fingerprintParts ...string) *Cache {
	return &Cache{Inner: inner, Dir: dir, Fingerprint: hash(fingerprintParts...)}
}

// Generate returns a cached response when one exists for this fingerprint+prompt,
// otherwise calls the wrapped model and stores the result.
func (c *Cache) Generate(ctx context.Context, prompt string) (string, error) {
	key := hash(c.Fingerprint, prompt)
	path := filepath.Join(c.Dir, key+".json")

	if data, err := os.ReadFile(path); err == nil {
		var e entry
		if json.Unmarshal(data, &e) == nil {
			atomic.AddInt64(&c.hits, 1)
			return e.Response, nil
		}
	}
	atomic.AddInt64(&c.misses, 1)

	resp, err := c.Inner.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(c.Dir, 0o755); err == nil {
		if data, mErr := json.MarshalIndent(entry{Fingerprint: c.Fingerprint, Prompt: prompt, Response: resp}, "", "  "); mErr == nil {
			_ = os.WriteFile(path, data, 0o644)
		}
	}
	return resp, nil
}

// Stats returns the cache hit/miss counts for this run.
func (c *Cache) Stats() (hits, misses int) {
	return int(atomic.LoadInt64(&c.hits)), int(atomic.LoadInt64(&c.misses))
}

// hash returns a hex SHA-256 over the parts, joined with a NUL separator so
// distinct part boundaries can't collide ("ab"+"c" vs "a"+"bc").
func hash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%s\x00", p)
	}
	return hex.EncodeToString(h.Sum(nil))
}
