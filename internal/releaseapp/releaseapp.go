// Package releaseapp assembles the release-content pipeline (build Release Context
// → analyze → generate the full suite → publish to S3) from configuration, so the
// same wiring drives both the one-shot Fargate runner (cmd/content-runner) and any
// other caller. It is the single source of truth for how a release run is built,
// which keeps the serverless generation path from diverging from the generator
// packages the rest of the system already uses.
//
// It intentionally does NOT trigger the video state machine: in the serverless
// architecture the GenerateContent state machine starts blog-gen-video as a
// separate step after this pipeline writes its scripts to S3.
package releaseapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/airouter"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/anthropic"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/artifactstore"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/bedrockclaude"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/config"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/contentsuite"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/engineeringanalysis"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metadata"
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

// Runner runs one release-content generation for a repository tag.
type Runner struct {
	pipeline *releasepipeline.Pipeline
	// Chain describes the configured AI provider order, for logging.
	Chain []string
}

// Run builds and publishes the full content suite for one release.
func (r *Runner) Run(ctx context.Context, req rc.Request) (releasepipeline.Result, error) {
	return r.pipeline.Run(ctx, req)
}

// New assembles the release pipeline from config: it resolves credentials, builds
// the AI provider router (Bedrock primary → Anthropic fallback), the S3 (or local)
// publisher, the release source, and the content-suite orchestrator, and wires S3
// artifact idempotency when a bucket is configured. It mirrors the construction
// the EC2 worker uses for its release path, so cloud output is identical.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger, awsCfg aws.Config) (*Runner, error) {
	sec := secrets.New(secretsmanager.NewFromConfig(awsCfg), cfg.RepoSecretID)
	meta := metadata.New(dynamodb.NewFromConfig(awsCfg), cfg.RepositoriesTable)
	// Resolves each repository's registered PAT to read the repo over the GitHub API.
	tokenSource := &reposource.MetaTokenSource{Meta: meta, Secrets: sec}

	// Anthropic key: a Secrets Manager reference wins over a plaintext env value.
	anthropicKey := cfg.AnthropicAPIKey
	if cfg.AnthropicAPIKeySecret != "" {
		if k, err := sec.Value(ctx, cfg.AnthropicAPIKeySecret); err != nil {
			logger.Warn("could not resolve Anthropic API key secret", "error", err.Error())
		} else {
			anthropicKey = k
		}
	}

	// AI Provider Router: Bedrock (Claude) primary with Anthropic API fallback.
	var chain []airouter.Provider
	if cfg.BedrockModelID != "" {
		if bc, err := bedrockclaude.NewFromAWS(ctx, cfg.AWSRegion, bedrockclaude.Config{ModelID: cfg.BedrockModelID, System: bedrockclaude.WriterPersona}); err != nil {
			logger.Warn("bedrock claude init failed; primary provider unavailable", "error", err.Error())
		} else {
			chain = append(chain, airouter.Provider{Name: airouter.ProviderBedrock, Model: cfg.BedrockModelID, Client: bc})
			logger.Info("ai provider registered", "provider", airouter.ProviderBedrock, "model", cfg.BedrockModelID)
		}
	}
	if anthropicKey != "" {
		am := cfg.AnthropicModel
		if am == "" {
			am = anthropic.DefaultModel
		}
		if an, err := anthropic.New(anthropic.Config{APIKey: anthropicKey, Model: cfg.AnthropicModel, System: bedrockclaude.WriterPersona}); err != nil {
			logger.Warn("anthropic client init failed", "error", err.Error())
		} else {
			chain = append(chain, airouter.Provider{Name: airouter.ProviderAnthropic, Model: am, Client: an})
			logger.Info("ai provider registered", "provider", airouter.ProviderAnthropic, "model", am)
		}
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("ai: no provider configured — set BEDROCK_MODEL_ID (cloud) or an Anthropic API key")
	}
	router := airouter.New(chain, logger)
	logger.Info("ai provider router configured", "chain", router.Chain())
	routed := func(kind string) releasegen.Model { return router.ModelFor(kind) }
	analysisModel := router.ModelFor("analysis")
	provName, provModel := router.Primary()

	// Publish to S3 when configured, otherwise the local filesystem.
	var publisher releasepipeline.Publisher = &publish.FilePublisher{Dir: cfg.OutputDir, Logger: logger}
	if cfg.OutputS3Bucket != "" {
		publisher = publish.NewS3(s3.NewFromConfig(awsCfg), cfg.OutputS3Bucket, cfg.OutputS3Prefix, logger)
	}

	// Read repos over the GitHub API (not by cloning); GITHUB_TOKEN is the public
	// fallback, each repo's registered PAT is used for private repos.
	releaseSrc := releasesource.New(github.New(), os.Getenv("GITHUB_TOKEN"))
	releaseSrc.TokenFor = tokenSource.Token

	// Provenance for release metadata: routed artifacts report the router's primary
	// provider; deterministic artifacts (e.g. the SVG diagram) report "deterministic".
	routedKinds := make(map[string]bool, len(airouter.AllKinds))
	for _, k := range airouter.AllKinds {
		routedKinds[k] = true
	}
	provenance := func(kind string) (provider, model, promptVersion string) {
		if !routedKinds[kind] {
			return "deterministic", "", promptversion.For(kind)
		}
		return provName, provModel, promptversion.For(kind)
	}

	pipe := &releasepipeline.Pipeline{
		Builder:          &rc.Builder{Sources: releaseSrc, Logger: logger},
		Analyzer:         &engineeringanalysis.Analyzer{Model: analysisModel, Logger: logger},
		Generator:        &releasegen.Generator{Model: routed("blog"), Logger: logger},
		Suite:            &contentsuite.Orchestrator{Model: analysisModel, ModelFor: routed, Provenance: provenance, Logger: logger},
		GeneratorVersion: os.Getenv("WORKER_VERSION"),
		ExperimentID:     os.Getenv("EXPERIMENT_ID"),
		Reviewer:         review.Reviewer{},
		Publisher:        publisher,
		Logger:           logger,
	}

	// S3 artifact idempotency: reuse any artifact already in S3 instead of
	// regenerating it (no model tokens re-spent).
	if cfg.OutputS3Bucket != "" {
		s3Client := s3.NewFromConfig(awsCfg)
		bucket, prefix := cfg.OutputS3Bucket, cfg.OutputS3Prefix
		pipe.ArtifactStore = func(ctx context.Context, rctx *rc.ReleaseContext) contentsuite.ArtifactStore {
			return artifactstore.NewS3(ctx, s3Client, bucket, prefix,
				rctx.Repository.Owner, rctx.Repository.Name, rctx.Release.Tag, logger)
		}
	}

	return &Runner{pipeline: pipe, Chain: router.Chain()}, nil
}
