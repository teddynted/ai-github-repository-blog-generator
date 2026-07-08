// Package approval implements the optional human-approval gate. When approval
// is required, generated content is not published to the live destination;
// instead it is stashed for review and the gate returns "not approved" so the
// pipeline holds. A reviewer later publishes from the stash (a management UI is
// future work).
package approval

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

// Stasher writes held content somewhere a reviewer can find it.
// *publish.FilePublisher satisfies it (pointed at a pending directory).
type Stasher interface {
	Publish(ctx context.Context, repoFullName string, assets []generation.Content) error
}

// HoldForReview stashes content for review and never auto-approves.
type HoldForReview struct {
	Stash  Stasher
	Logger *slog.Logger
}

// Approve stashes the assets and returns false (held for a human).
func (h *HoldForReview) Approve(ctx context.Context, repoFullName string, assets []generation.Content) (bool, error) {
	if err := h.Stash.Publish(ctx, repoFullName, assets); err != nil {
		return false, fmt.Errorf("stash for review: %w", err)
	}
	if h.Logger != nil {
		h.Logger.Info("content stashed for human approval",
			slog.String("repo", repoFullName), slog.Int("assets", len(assets)))
	}
	return false, nil
}
