// Package pipeline composes the content flow for a matched event: gather a
// repository Snapshot, generate content, and publish it. It is the orchestration
// logic that ties the processing, generation, and publish seams together,
// independent of how it is invoked (an instance worker draining SQS, an n8n exec
// node, etc. — that runtime choice lives outside this package).
package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

// Snapshotter gathers a repository snapshot. *processing.Processor satisfies it.
type Snapshotter interface {
	Process(ctx context.Context, repoFullName, ref string) (processing.Snapshot, error)
}

// ContentGenerator produces content assets. *generation.Generator satisfies it.
type ContentGenerator interface {
	GenerateAll(ctx context.Context, snap processing.Snapshot, kinds ...generation.Kind) ([]generation.Content, error)
}

// Publisher delivers assets to a destination. *publish.LogPublisher satisfies it.
type Publisher interface {
	Publish(ctx context.Context, repoFullName string, assets []generation.Content) error
}

// Request is a single pipeline run (typically derived from a matched event).
type Request struct {
	RepoFullName string
	Ref          string
}

// Result summarises a run.
type Result struct {
	RepoFullName string
	Published    int
}

// Pipeline runs process → generate → publish.
type Pipeline struct {
	Processor Snapshotter
	Generator ContentGenerator
	Publisher Publisher
	Kinds     []generation.Kind // defaults to blog when empty
	Logger    *slog.Logger
}

// Run executes the pipeline. Generation is resilient: if some kinds fail, the
// successful assets are still published, and the per-kind failure is returned
// so the caller can decide whether to retry — a partial package is never lost.
func (p *Pipeline) Run(ctx context.Context, req Request) (Result, error) {
	if req.RepoFullName == "" {
		return Result{}, fmt.Errorf("pipeline: request is missing a repository")
	}

	snap, err := p.Processor.Process(ctx, req.RepoFullName, req.Ref)
	if err != nil {
		return Result{}, fmt.Errorf("process: %w", err)
	}

	kinds := p.Kinds
	if len(kinds) == 0 {
		kinds = []generation.Kind{generation.KindBlog}
	}

	assets, genErr := p.Generator.GenerateAll(ctx, snap, kinds...)

	// Publish whatever succeeded, even on a partial generation failure.
	if len(assets) > 0 {
		if pubErr := p.Publisher.Publish(ctx, req.RepoFullName, assets); pubErr != nil {
			return Result{RepoFullName: req.RepoFullName}, fmt.Errorf("publish: %w", pubErr)
		}
	}

	res := Result{RepoFullName: req.RepoFullName, Published: len(assets)}
	if genErr != nil {
		return res, fmt.Errorf("generation partial failure: %w", genErr)
	}
	p.log("pipeline complete", req.RepoFullName, len(assets))
	return res, nil
}

func (p *Pipeline) log(msg, repoName string, published int) {
	if p.Logger != nil {
		p.Logger.Info(msg, slog.String("repo", repoName), slog.Int("published", published))
	}
}
