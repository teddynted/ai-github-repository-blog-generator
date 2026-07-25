package modelfallback

import (
	"context"
	"errors"
	"testing"
)

type stubModel struct {
	out    string
	err    error
	calls  int
	prompt string
}

func (s *stubModel) Generate(_ context.Context, prompt string) (string, error) {
	s.calls++
	s.prompt = prompt
	return s.out, s.err
}

func TestPrimarySucceedsNoFallback(t *testing.T) {
	primary := &stubModel{out: "claude wrote this"}
	secondary := &stubModel{out: "ollama wrote this"}
	f := &Fallback{Primary: primary, Secondary: secondary}

	got, err := f.Generate(context.Background(), "write the blog")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "claude wrote this" {
		t.Errorf("text = %q", got)
	}
	if secondary.calls != 0 {
		t.Errorf("secondary should not be called when primary succeeds (calls=%d)", secondary.calls)
	}
}

func TestFallsBackOnPrimaryError(t *testing.T) {
	primary := &stubModel{err: errors.New("Operation not allowed")}
	secondary := &stubModel{out: "ollama wrote this"}
	f := &Fallback{Primary: primary, Secondary: secondary}

	got, err := f.Generate(context.Background(), "write the blog")
	if err != nil {
		t.Fatalf("Generate should fall back, got err: %v", err)
	}
	if got != "ollama wrote this" {
		t.Errorf("text = %q (should be the secondary's)", got)
	}
	if secondary.prompt != "write the blog" {
		t.Errorf("secondary got prompt %q", secondary.prompt)
	}
}

func TestNoSecondaryReturnsPrimaryError(t *testing.T) {
	primary := &stubModel{err: errors.New("boom")}
	f := &Fallback{Primary: primary}
	if _, err := f.Generate(context.Background(), "x"); err == nil {
		t.Fatal("expected the primary error when no secondary is configured")
	}
}

func TestContextCancellationIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	primary := &stubModel{err: context.Canceled}
	secondary := &stubModel{out: "ollama"}
	f := &Fallback{Primary: primary, Secondary: secondary}

	if _, err := f.Generate(ctx, "x"); err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if secondary.calls != 0 {
		t.Errorf("secondary must not run after context cancellation (calls=%d)", secondary.calls)
	}
}
