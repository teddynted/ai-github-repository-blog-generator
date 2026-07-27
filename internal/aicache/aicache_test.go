package aicache

import (
	"context"
	"errors"
	"testing"
)

type countingModel struct {
	calls int
	reply string
	err   error
}

func (m *countingModel) Generate(_ context.Context, _ string) (string, error) {
	m.calls++
	return m.reply, m.err
}

func TestCacheHitAvoidsSecondCall(t *testing.T) {
	inner := &countingModel{reply: "generated"}
	c := New(inner, t.TempDir(), "anthropic", "claude", "sys", "0.2")

	// First call: miss → underlying invoked, response stored.
	got, err := c.Generate(context.Background(), "prompt A")
	if err != nil || got != "generated" {
		t.Fatalf("first Generate = %q, %v", got, err)
	}
	// Second identical call: hit → underlying NOT invoked, same response.
	got2, _ := c.Generate(context.Background(), "prompt A")
	if got2 != "generated" {
		t.Errorf("cached response = %q", got2)
	}
	if inner.calls != 1 {
		t.Errorf("underlying called %d times, want 1 (second was a cache hit)", inner.calls)
	}
	if h, m := c.Stats(); h != 1 || m != 1 {
		t.Errorf("stats = %d hit / %d miss, want 1/1", h, m)
	}
}

func TestCacheMissesOnChangedInputs(t *testing.T) {
	dir := t.TempDir()
	inner := &countingModel{reply: "x"}
	c := New(inner, dir, "ollama", "qwen", "", "0")

	_, _ = c.Generate(context.Background(), "prompt A")
	_, _ = c.Generate(context.Background(), "prompt B") // different prompt → miss

	// A different fingerprint (e.g. a new model) must not reuse the entry.
	c2 := New(inner, dir, "ollama", "qwen3", "", "0")
	_, _ = c2.Generate(context.Background(), "prompt A")

	if inner.calls != 3 {
		t.Errorf("underlying called %d times, want 3 (all misses)", inner.calls)
	}
}

func TestCacheDoesNotStoreOnError(t *testing.T) {
	dir := t.TempDir()
	inner := &countingModel{err: errors.New("boom")}
	c := New(inner, dir, "p", "m", "", "0")

	if _, err := c.Generate(context.Background(), "p"); err == nil {
		t.Fatal("expected error to propagate")
	}
	// A retry still calls the model (nothing was cached).
	inner.err = nil
	inner.reply = "ok"
	if got, _ := c.Generate(context.Background(), "p"); got != "ok" {
		t.Errorf("after error, retry = %q, want live call", got)
	}
	if inner.calls != 2 {
		t.Errorf("calls = %d, want 2 (error not cached)", inner.calls)
	}
}
