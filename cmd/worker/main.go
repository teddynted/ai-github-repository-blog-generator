// Command worker runs on the EC2 instance and drives the content pipeline: it
// long-polls the SQS events queue for matched publishing events, runs
// process → generate → publish for each, and deletes the message on success
// (leaving failures for SQS to redeliver / dead-letter).
//
// This is the MVP runtime that invokes internal/pipeline. The documented n8n
// SQS-trigger workflow is an alternative orchestration for the same seams
// (see docs/development-plan.md); this worker keeps the flow fully testable.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awssqs"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/pipeline"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/publish"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/webhook"
)

// eventEnvelope unwraps the EventBridge event delivered to SQS; the detail is
// the matched-event payload published by the webhook handler.
type eventEnvelope struct {
	Detail webhook.Event `json:"detail"`
}

// defaultKinds is the content package the worker generates per run.
var defaultKinds = []generation.Kind{
	generation.KindBlog,
	generation.KindReadme,
	generation.KindDocs,
	generation.KindArchitecture,
	generation.KindReleaseNotes,
}

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion", "QueueURL"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	queue := awssqs.New(sqs.NewFromConfig(awsCfg), a.Config.QueueURL)
	pipe := &pipeline.Pipeline{
		// Placeholder processing until the OpenClaw-backed implementation lands.
		Processor: processing.NewPlaceholderProcessor(),
		Generator: &generation.Generator{
			Model:  ollama.New(a.Config.OllamaModel, ollama.WithBaseURL(a.Config.OllamaBaseURL)),
			Logger: a.Logger,
		},
		Publisher: &publish.FilePublisher{Dir: a.Config.OutputDir, Logger: a.Logger},
		Kinds:     defaultKinds,
		Logger:    a.Logger,
	}

	a.Logger.Info("worker started", "queue", a.Config.QueueURL, "model", a.Config.OllamaModel)
	run(ctx, a.Logger, queue, pipe)
	a.Logger.Info("worker stopped")
}

// consumer is the queue behaviour the loop needs (satisfied by *awssqs.Client).
type consumer interface {
	Receive(ctx context.Context, maxMessages, waitSeconds int32) ([]awssqs.Message, error)
	Delete(ctx context.Context, receiptHandle string) error
}

// runner runs one request (satisfied by *pipeline.Pipeline).
type runner interface {
	Run(ctx context.Context, req pipeline.Request) (pipeline.Result, error)
}

func run(ctx context.Context, logger interface{ Error(string, ...any) }, q consumer, p runner) {
	for ctx.Err() == nil {
		msgs, err := q.Receive(ctx, 10, 20)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("receive failed", "error", err.Error())
			continue
		}
		for _, m := range msgs {
			handleMessage(ctx, logger, q, p, m)
		}
	}
}

func handleMessage(ctx context.Context, logger interface{ Error(string, ...any) }, q consumer, p runner, m awssqs.Message) {
	var env eventEnvelope
	if err := json.Unmarshal([]byte(m.Body), &env); err != nil || env.Detail.RepoFullName == "" {
		// Unparseable/irrelevant message: drop it so it does not loop forever.
		logger.Error("unparseable message; dropping", "error", errString(err))
		_ = q.Delete(ctx, m.ReceiptHandle)
		return
	}
	if _, err := p.Run(ctx, pipeline.Request{RepoFullName: env.Detail.RepoFullName, Ref: env.Detail.Ref}); err != nil {
		// Leave the message for SQS to redeliver / dead-letter.
		logger.Error("run failed; leaving message for retry", "repo", env.Detail.RepoFullName, "error", err.Error())
		return
	}
	_ = q.Delete(ctx, m.ReceiptHandle)
}

func errString(err error) string {
	if err == nil {
		return "empty detail"
	}
	return err.Error()
}
