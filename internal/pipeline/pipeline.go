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

// Memory is Repository Memory: it lets the pipeline skip a commit it has already
// published and record new ones. *memory.Store satisfies it. Optional.
type Memory interface {
	AlreadyPublished(ctx context.Context, repoFullName, commitSHA string) (bool, error)
	RecordPublished(ctx context.Context, repoFullName, commitSHA string, kinds []string) error
}

// Request is a single pipeline run (typically derived from a matched event).
type Request struct {
	RepoFullName string
	Ref          string
	CommitSHA    string
}

// Result summarises a run.
type Result struct {
	RepoFullName string
	Published    int
	Skipped      bool // already published for this commit (Repository Memory)
}

// Pipeline runs process → generate → publish.
type Pipeline struct {
	Processor Snapshotter
	Generator ContentGenerator
	Publisher Publisher
	Memory    Memory            // optional; de-duplicates already-published commits
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

	// Repository Memory: skip a commit already published, to avoid duplicates.
	if p.Memory != nil && req.CommitSHA != "" {
		done, err := p.Memory.AlreadyPublished(ctx, req.RepoFullName, req.CommitSHA)
		if err != nil {
			// Memory is an optimisation; a read failure must not block a run.
			p.logMemoryIssue("memory lookup failed; proceeding", req.RepoFullName, err)
		} else if done {
			p.log("skipping already-published commit", req.RepoFullName, 0)
			return Result{RepoFullName: req.RepoFullName, Skipped: true}, nil
		}
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

	// Record what was published so a repeat event for the same commit is skipped.
	if p.Memory != nil && req.CommitSHA != "" && len(assets) > 0 {
		if err := p.Memory.RecordPublished(ctx, req.RepoFullName, req.CommitSHA, kindsOf(assets)); err != nil {
			p.logMemoryIssue("failed to record memory", req.RepoFullName, err)
		}
	}

	res := Result{RepoFullName: req.RepoFullName, Published: len(assets)}
	if genErr != nil {
		return res, fmt.Errorf("generation partial failure: %w", genErr)
	}
	p.log("pipeline complete", req.RepoFullName, len(assets))
	return res, nil
}

func kindsOf(assets []generation.Content) []string {
	kinds := make([]string, 0, len(assets))
	for _, a := range assets {
		kinds = append(kinds, string(a.Kind))
	}
	return kinds
}

func (p *Pipeline) log(msg, repoName string, published int) {
	if p.Logger != nil {
		p.Logger.Info(msg, slog.String("repo", repoName), slog.Int("published", published))
	}
}

func (p *Pipeline) logMemoryIssue(msg, repoName string, err error) {
	if p.Logger != nil {
		p.Logger.Warn(msg, slog.String("repo", repoName), slog.String("error", err.Error()))
	}
}
