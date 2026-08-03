// Command content-runner is the one-shot Fargate task that generates the full
// release content suite for a single repository release. The GenerateContent state
// machine runs it (env FORMAT-free: OWNER, REPO, RELEASE_TAG), it builds the
// Release Context, generates every artifact, and publishes them to S3 — then the
// state machine starts the video render and publishes the manifest as later steps.
//
// This replaces the EC2 worker's release path with an on-demand serverless run:
// no SQS, no instance wake-gate, no long-running box. It reuses the exact same
// pipeline wiring as the worker via internal/releaseapp, so the output is identical.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releaseapp"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	// The release run reads repos over the GitHub API and publishes to S3, so it
	// needs the region, the repositories table + shared secret (for per-repo PATs),
	// and an output bucket. OutputS3Bucket is not in Require's field allowlist, so
	// check it directly (as the worker does).
	if err := a.Config.Require("AWSRegion", "RepositoriesTable", "RepoSecretID"); err != nil {
		log.Fatalf("config: %v", err)
	}
	if a.Config.OutputS3Bucket == "" {
		log.Fatalf("config: OUTPUT_S3_BUCKET is required")
	}

	owner := os.Getenv("OWNER")
	repo := os.Getenv("REPO")
	tag := os.Getenv("RELEASE_TAG")
	if owner == "" || repo == "" || tag == "" {
		log.Fatalf("OWNER, REPO and RELEASE_TAG are required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	runner, err := releaseapp.New(ctx, a.Config, a.Logger, awsCfg)
	if err != nil {
		log.Fatalf("build release pipeline: %v", err)
	}
	a.Logger.Info("content-runner started", "owner", owner, "repo", repo, "tag", tag, "ai_chain", runner.Chain)

	res, err := runner.Run(ctx, rc.Request{Owner: owner, Repository: repo, ReleaseTag: tag})
	if err != nil {
		log.Fatalf("release run failed: owner=%s repo=%s tag=%s: %v", owner, repo, tag, err)
	}
	a.Logger.Info("content-runner finished", "owner", owner, "repo", repo, "tag", tag, "published", res.Published)
}
