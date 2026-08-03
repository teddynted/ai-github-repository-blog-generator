package releaseapp_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/config"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releaseapp"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// With neither Bedrock nor an Anthropic key configured, there is no AI provider
// and New must fail fast rather than build an unusable pipeline.
func TestNewRequiresAnAIProvider(t *testing.T) {
	cfg := config.Config{AWSRegion: "us-east-1", OutputS3Bucket: "bucket"}
	if _, err := releaseapp.New(context.Background(), cfg, quietLogger(), aws.Config{}); err == nil {
		t.Fatal("expected an error when neither Bedrock nor Anthropic is configured")
	}
}

// A configured Anthropic key is enough to build the pipeline; the router chain
// should report the Anthropic leg (defaulting the model when unset). No network
// or AWS calls happen during construction.
func TestNewBuildsPipelineWithAnthropicKey(t *testing.T) {
	cfg := config.Config{
		AWSRegion:       "us-east-1",
		OutputS3Bucket:  "bucket",
		AnthropicAPIKey: "test-key",
	}
	r, err := releaseapp.New(context.Background(), cfg, quietLogger(), aws.Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if r == nil {
		t.Fatal("expected a non-nil runner")
	}
	if len(r.Chain) != 1 || r.Chain[0] != "anthropic:claude-sonnet-5" {
		t.Errorf("chain = %v, want [anthropic:claude-sonnet-5]", r.Chain)
	}
}
