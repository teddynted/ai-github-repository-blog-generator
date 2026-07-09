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
	"strings"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/approval"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/archdiagram"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awssqs"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/memory"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metadata"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metrics"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/notify"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/pipeline"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/publish"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/reposource"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/review"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/secrets"
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
	if err := a.Config.Require("AWSRegion", "QueueURL", "RepositoriesTable", "SecretsPrefix"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	queue := awssqs.New(sqs.NewFromConfig(awsCfg), a.Config.QueueURL)

	// Real repository processing: resolve the PAT from the repo's metadata
	// secret reference, then clone and read the working copy.
	meta := metadata.New(dynamodb.NewFromConfig(awsCfg), a.Config.RepositoriesTable)
	sec := secrets.New(secretsmanager.NewFromConfig(awsCfg), a.Config.SecretsPrefix)
	processor := &processing.Processor{
		Cloner: &reposource.GitCloner{
			Tokens:  &reposource.MetaTokenSource{Meta: meta, Secrets: sec},
			WorkDir: a.Config.WorkDir,
			Logger:  a.Logger,
		},
		Readme:      reposource.FSReadme{},
		Docs:        reposource.FSDocs{},
		Commits:     reposource.GitCommits{},
		Analyzer:    reposource.FSAnalyzer{},
		CommitLimit: 20,
		Logger:      a.Logger,
	}

	// Optional human-approval gate: when required, hold content for review
	// (stashed under PendingDir) instead of publishing.
	var approver pipeline.Approver
	if a.Config.RequireHumanApproval {
		approver = &approval.HoldForReview{
			Stash:  &publish.FilePublisher{Dir: a.Config.PendingDir, Logger: a.Logger},
			Logger: a.Logger,
		}
	}

	// Publish to S3 when a bucket is configured, otherwise to the local
	// filesystem (on the instance /data is the persistent EBS volume).
	var publisher pipeline.Publisher = &publish.FilePublisher{Dir: a.Config.OutputDir, Logger: a.Logger}
	if a.Config.OutputS3Bucket != "" {
		publisher = publish.NewS3(s3.NewFromConfig(awsCfg), a.Config.OutputS3Bucket, a.Config.OutputS3Prefix, a.Logger)
	}

	pipe := &pipeline.Pipeline{
		Processor: processor,
		Generator: &generation.Generator{
			Model:  ollama.New(a.Config.OllamaModel, ollama.WithBaseURL(a.Config.OllamaBaseURL)),
			Logger: a.Logger,
		},
		Publisher:  publisher,
		Memory:     &memory.Store{Dir: a.Config.MemoryDir, Logger: a.Logger},
		Reviewer:   review.Reviewer{},
		Approver:   approver,
		Diagrammer: archdiagram.Diagrammer{},
		Kinds:      defaultKinds,
		Logger:     a.Logger,
	}

	notifiers := notify.Multi{&notify.LogNotifier{Logger: a.Logger}}
	if a.Config.NotifyWebhookURL != "" {
		notifiers = append(notifiers, notify.NewWebhook(a.Config.NotifyWebhookURL, a.Logger))
	}
	// Resolve the SMTP password: a Secrets Manager reference wins over a plaintext
	// env value, so the credential need never sit in the instance's env file.
	smtpPassword := a.Config.SMTPPassword
	if a.Config.SMTPPasswordSecret != "" {
		if pw, err := sec.Value(ctx, a.Config.SMTPPasswordSecret); err != nil {
			// Email is optional; a resolution failure must not stop the worker.
			a.Logger.Warn("could not resolve SMTP password secret; email disabled",
				"error", err.Error())
			smtpPassword = ""
		} else {
			smtpPassword = pw
		}
	}
	if a.Config.NotifyEmailFrom != "" && a.Config.NotifyEmailTo != "" &&
		a.Config.SMTPUsername != "" && smtpPassword != "" {
		sender := notify.NewSMTP(a.Config.SMTPHost, a.Config.SMTPPort,
			a.Config.SMTPUsername, smtpPassword)
		notifiers = append(notifiers, notify.NewEmail(
			sender, a.Config.NotifyEmailFrom,
			splitCSV(a.Config.NotifyEmailTo), a.Logger))
	}
	var notifier notify.Notifier = notifiers
	meter := metrics.New(metrics.Namespace, os.Stdout)

	a.Logger.Info("worker started", "queue", a.Config.QueueURL, "model", a.Config.OllamaModel)
	run(ctx, a.Logger, queue, pipe, notifier, meter)
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

// meter emits metrics (satisfied by *metrics.Emitter).
type meter interface {
	Count(name string)
	CountN(name string, n float64)
}

func run(ctx context.Context, logger interface{ Error(string, ...any) }, q consumer, p runner, n notify.Notifier, mt meter) {
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
			handleMessage(ctx, logger, q, p, n, mt, m)
		}
	}
}

func handleMessage(ctx context.Context, logger interface{ Error(string, ...any) }, q consumer, p runner, n notify.Notifier, mt meter, m awssqs.Message) {
	var env eventEnvelope
	if err := json.Unmarshal([]byte(m.Body), &env); err != nil || env.Detail.RepoFullName == "" {
		// Unparseable/irrelevant message: drop it so it does not loop forever.
		logger.Error("unparseable message; dropping", "error", errString(err))
		_ = q.Delete(ctx, m.ReceiptHandle)
		return
	}

	repo := env.Detail.RepoFullName
	mt.Count("RunsStarted")
	res, err := p.Run(ctx, pipeline.Request{
		RepoFullName: repo,
		Ref:          env.Detail.Ref,
		CommitSHA:    env.Detail.CommitSHA,
	})
	if err != nil {
		// Leave the message for SQS to redeliver / dead-letter.
		logger.Error("run failed; leaving message for retry", "repo", repo, "error", err.Error())
		mt.Count("RunsFailed")
		_ = n.Notify(ctx, notify.Event{Repo: repo, Status: notify.StatusFailed, Err: err.Error()})
		return
	}

	switch {
	case res.Skipped:
		mt.Count("RunsSkipped")
	case res.Held:
		mt.Count("RunsHeld")
		_ = n.Notify(ctx, notify.Event{Repo: repo, Status: notify.StatusHeld})
	default:
		mt.Count("RunsSucceeded")
		mt.CountN("AssetsGenerated", float64(res.Published))
		_ = n.Notify(ctx, notify.Event{Repo: repo, Status: notify.StatusPublished, Assets: res.Published})
	}
	_ = q.Delete(ctx, m.ReceiptHandle)
}

func errString(err error) string {
	if err == nil {
		return "empty detail"
	}
	return err.Error()
}

// splitCSV splits a comma-separated list, trimming spaces and dropping blanks.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
