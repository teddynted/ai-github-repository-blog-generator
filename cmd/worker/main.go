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

	"github.com/teddynted/ai-github-repository-blog-generator/internal/airouter"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/anthropic"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/approval"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/archdiagram"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awssqs"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/bedrockclaude"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentsuite"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/engineeringanalysis"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/intake"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/memory"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metadata"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metrics"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/notify"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/pipeline"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/promptversion"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/publish"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasepipeline"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasesource"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/reposource"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/review"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/secrets"
)

// eventEnvelope unwraps the EventBridge event delivered to SQS; the detail is
// the intake.Event published by a trigger source (webhook or manual /process).
type eventEnvelope struct {
	Detail intake.Event `json:"detail"`
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
	if err := a.Config.Require("AWSRegion", "QueueURL", "RepositoriesTable", "RepoSecretID"); err != nil {
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
	sec := secrets.New(secretsmanager.NewFromConfig(awsCfg), a.Config.RepoSecretID)
	// Resolves a repository's registered PAT from the shared secret; used both to
	// clone and to read the repo via the GitHub API for the release pipeline.
	tokenSource := &reposource.MetaTokenSource{Meta: meta, Secrets: sec}
	processor := &processing.Processor{
		Cloner: &reposource.GitCloner{
			Tokens:  tokenSource,
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
			Model:  ollama.New(a.Config.OllamaModel, ollama.WithBaseURL(a.Config.OllamaBaseURL), ollama.WithTimeout(a.Config.OllamaTimeout)),
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

	// Release-content path (Milestone 3): a published-release event builds a
	// Release Context and generates release-focused content from it, reusing the
	// same reviewer + publisher. GITHUB_TOKEN is optional (public repos work
	// rate-limited); it reads the repo over the GitHub API, not by cloning.
	releaseSrc := releasesource.New(github.New(), os.Getenv("GITHUB_TOKEN"))
	// Read private repos with each repo's registered PAT (same credential as
	// cloning); GITHUB_TOKEN is the fallback for public repos / unregistered.
	releaseSrc.TokenFor = tokenSource.Token
	// Two-stage model split. Stage 2 (engineering analysis) always uses the local
	// Ollama model. Stage 3 (writing) is routed per artifact by the Hybrid AI
	// Router: high-value public-facing content (blog, architecture, LinkedIn, X
	// thread) goes to Claude when configured; commodity artifacts (SEO, visual
	// assets, short-form scripts) stay on Ollama — quality where it matters, cheap
	// where it doesn't. Claude provider preference: Anthropic API (key) > Bedrock
	// (IAM). Every routed selection wraps a per-call fallback to Ollama, so an
	// unavailable provider degrades gracefully rather than losing a stage.
	analysisModel := ollama.New(a.Config.OllamaModel, ollama.WithBaseURL(a.Config.OllamaBaseURL), ollama.WithTimeout(a.Config.OllamaTimeout))

	// Resolve the Anthropic API key: a Secrets Manager reference wins over a
	// plaintext env value, so the credential need never sit in the instance's env.
	anthropicKey := a.Config.AnthropicAPIKey
	if a.Config.AnthropicAPIKeySecret != "" {
		if k, kerr := sec.Value(ctx, a.Config.AnthropicAPIKeySecret); kerr != nil {
			a.Logger.Warn("could not resolve Anthropic API key secret; skipping Anthropic writer", "error", kerr.Error())
		} else {
			anthropicKey = k
		}
	}

	// Register providers: Ollama always; Claude via Anthropic API (preferred) or
	// Bedrock when configured. A Claude init failure just leaves it unregistered,
	// so its routes fall through to Ollama.
	providers := map[string]airouter.Model{airouter.ProviderOllama: analysisModel}
	claudeModel := "" // the effective Claude model id, for release-metadata provenance
	switch {
	case anthropicKey != "":
		am := a.Config.AnthropicModel
		if am == "" {
			am = anthropic.DefaultModel
		}
		if cl, cerr := anthropic.New(anthropic.Config{APIKey: anthropicKey, Model: a.Config.AnthropicModel, System: bedrockclaude.WriterPersona}); cerr != nil {
			a.Logger.Warn("anthropic client init failed; Claude routes degrade to local", "error", cerr.Error())
		} else {
			providers[airouter.ProviderClaude] = cl
			claudeModel = am
			a.Logger.Info("premium writer registered: claude via anthropic api", "model", am)
		}
	case a.Config.BedrockModelID != "":
		if claude, berr := bedrockclaude.NewFromAWS(ctx, a.Config.AWSRegion, bedrockclaude.Config{ModelID: a.Config.BedrockModelID, System: bedrockclaude.WriterPersona}); berr != nil {
			a.Logger.Warn("bedrock claude init failed; Claude routes degrade to local", "error", berr.Error())
		} else {
			providers[airouter.ProviderClaude] = claude
			claudeModel = a.Config.BedrockModelID
			a.Logger.Info("premium writer registered: claude on bedrock", "model", a.Config.BedrockModelID)
		}
	}

	// Hybrid routing policy: the recommended default, overridable via the
	// AI_ROUTING_RULES env var (JSON). Ollama is both the default and the fallback.
	rules, rerr := airouter.ParseRules(a.Config.AIRoutingRules)
	if rerr != nil {
		a.Logger.Warn("invalid AI_ROUTING_RULES; using default policy", "error", rerr.Error())
		rules = airouter.DefaultRules()
	}
	router := airouter.New(providers, rules, airouter.ProviderOllama, airouter.ProviderOllama, a.Logger)
	a.Logger.Info("hybrid ai routing configured",
		"providers", router.Providers(), "decisions", router.Decisions(airouter.AllKinds))
	routed := func(kind string) releasegen.Model { return router.ModelFor(kind) }

	// Provenance for the release metadata: which provider/model/prompt produced
	// each artifact. Routed kinds report their effective provider + model; kinds
	// with no routed model (the deterministic SVG diagram) report "deterministic".
	decisions := router.Decisions(airouter.AllKinds)
	providerModels := map[string]string{
		airouter.ProviderOllama: a.Config.OllamaModel,
		airouter.ProviderClaude: claudeModel,
	}
	provenance := func(kind string) (provider, model, promptVersion string) {
		prov, ok := decisions[kind]
		if !ok {
			return "deterministic", "", promptversion.For(kind)
		}
		return prov, providerModels[prov], promptversion.For(kind)
	}

	releasePipe := &releasepipeline.Pipeline{
		Builder: &rc.Builder{
			Sources: releaseSrc,
			Logger:  a.Logger,
		},
		// Stage 2: extract the structured engineering analysis with the local model.
		Analyzer: &engineeringanalysis.Analyzer{Model: analysisModel, Fallback: providers[airouter.ProviderClaude], Logger: a.Logger},
		// Stage 3: the writer model produces the FULL artifact set (blog → storyboard
		// → voice-over → YouTube → Shorts → TikTok → visual assets → SEO →
		// architecture → LinkedIn → X thread), all grounded in the analysis and gated
		// by the same review/publish stages. A thin release degrades gracefully.
		Generator:        &releasegen.Generator{Model: routed("blog"), Logger: a.Logger},
		Suite:            &contentsuite.Orchestrator{Model: analysisModel, ModelFor: routed, Provenance: provenance, Logger: a.Logger},
		GeneratorVersion: os.Getenv("WORKER_VERSION"),
		Reviewer:         review.Reviewer{},
		Publisher:        publisher,
		Logger:           a.Logger,
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
	run(ctx, a.Logger, queue, pipe, releasePipe, notifier, meter)
	a.Logger.Info("worker stopped")
}

// consumer is the queue behaviour the loop needs (satisfied by *awssqs.Client).
type consumer interface {
	Receive(ctx context.Context, maxMessages, waitSeconds int32) ([]awssqs.Message, error)
	Delete(ctx context.Context, receiptHandle string) error
}

// runner runs one snapshot request (satisfied by *pipeline.Pipeline).
type runner interface {
	Run(ctx context.Context, req pipeline.Request) (pipeline.Result, error)
}

// releaseRunner builds and publishes content for a release (satisfied by
// *releasepipeline.Pipeline). Optional — nil disables the release path and
// release events fall through to the snapshot pipeline.
type releaseRunner interface {
	Run(ctx context.Context, req rc.Request) (releasepipeline.Result, error)
}

// meter emits metrics (satisfied by *metrics.Emitter).
type meter interface {
	Count(name string)
	CountN(name string, n float64)
}

func run(ctx context.Context, logger interface{ Error(string, ...any) }, q consumer, p runner, rr releaseRunner, n notify.Notifier, mt meter) {
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
			handleMessage(ctx, logger, q, p, rr, n, mt, m)
		}
	}
}

func handleMessage(ctx context.Context, logger interface{ Error(string, ...any) }, q consumer, p runner, rr releaseRunner, n notify.Notifier, mt meter, m awssqs.Message) {
	var env eventEnvelope
	if err := json.Unmarshal([]byte(m.Body), &env); err != nil || env.Detail.RepoFullName == "" {
		// Unparseable/irrelevant message: drop it so it does not loop forever.
		logger.Error("unparseable message; dropping", "error", errString(err))
		_ = q.Delete(ctx, m.ReceiptHandle)
		return
	}

	// Any event that names a release (a published-release webhook, or a manual
	// /process request with a releaseTag) takes the release-content path: build
	// a Release Context, then generate release-focused content.
	if rr != nil && (env.Detail.Source == "release" || env.Detail.ReleaseTag != "") {
		handleReleaseMessage(ctx, logger, q, rr, n, mt, env, m)
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

// handleReleaseMessage runs the release-content pipeline for a published-release
// event: build the Release Context for the tag, generate and publish content,
// then ack the message. Failures leave the message for SQS to redeliver.
func handleReleaseMessage(ctx context.Context, logger interface{ Error(string, ...any) }, q consumer, rr releaseRunner, n notify.Notifier, mt meter, env eventEnvelope, m awssqs.Message) {
	repo := env.Detail.RepoFullName
	owner, name := env.Detail.Owner, env.Detail.Name
	tag := env.Detail.ReleaseTag
	if tag == "" {
		tag = strings.TrimPrefix(env.Detail.Ref, "refs/tags/")
	}
	if owner == "" || name == "" || tag == "" {
		logger.Error("release event missing owner/repo/tag; dropping", "repo", repo, "ref", env.Detail.Ref)
		_ = q.Delete(ctx, m.ReceiptHandle)
		return
	}

	mt.Count("ReleaseRunsStarted")
	res, err := rr.Run(ctx, rc.Request{Owner: owner, Repository: name, ReleaseTag: tag})
	if err != nil {
		logger.Error("release run failed; leaving message for retry", "repo", repo, "release", tag, "error", err.Error())
		mt.Count("ReleaseRunsFailed")
		_ = n.Notify(ctx, notify.Event{Repo: repo, Status: notify.StatusFailed, Err: err.Error()})
		return
	}
	mt.Count("ReleaseRunsSucceeded")
	mt.CountN("AssetsGenerated", float64(res.Published))
	_ = n.Notify(ctx, notify.Event{Repo: repo, Status: notify.StatusPublished, Assets: res.Published})
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
