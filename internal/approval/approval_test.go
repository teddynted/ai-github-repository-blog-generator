package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

type fakeStash struct {
	repo   string
	assets []generation.Content
	err    error
}

func (f *fakeStash) Publish(_ context.Context, repoFullName string, assets []generation.Content) error {
	f.repo, f.assets = repoFullName, assets
	return f.err
}

func TestHoldForReviewStashesAndHolds(t *testing.T) {
	stash := &fakeStash{}
	h := &HoldForReview{Stash: stash}
	assets := []generation.Content{{Kind: generation.KindBlog, Markdown: "# x"}}

	approved, err := h.Approve(context.Background(), "acme/widget", assets)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approved {
		t.Error("HoldForReview must never auto-approve")
	}
	if stash.repo != "acme/widget" || len(stash.assets) != 1 {
		t.Errorf("content not stashed: %+v", stash)
	}
}

func TestHoldForReviewStashError(t *testing.T) {
	h := &HoldForReview{Stash: &fakeStash{err: errors.New("disk full")}}
	if _, err := h.Approve(context.Background(), "a/b", nil); err == nil {
		t.Error("expected stash error to propagate")
	}
}
