// Command webhook-handler is the AWS Lambda entry point for GitHub webhook
// deliveries. It stays intentionally lightweight: resolve metadata, verify the
// signature, evaluate the commit-message trigger, and (on a match) publish an
// event. It performs no cloning, analysis, or AI work.
package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awsec2"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/eventbus"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metadata"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metrics"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/secrets"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/webhook"
)

// windowGate reports whether the platform's compute host (located by its
// Project tag) is running, so the handler can distinguish a delivery that is
// processed in the current scheduled window from one deferred to the next. It
// is strictly read-only — the scheduler stack owns instance power.
type windowGate struct {
	ec2     *awsec2.Client
	project string
}

func (g windowGate) Running(ctx context.Context) (bool, error) {
	inst, err := g.ec2.FindInstance(ctx, g.project)
	if err != nil {
		return false, err
	}
	return inst.State == "running", nil
}

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion", "RepositoriesTable", "SecretsPrefix", "EventBusName", "EventSource"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	handler := &webhook.Handler{
		Repos:          metadata.New(dynamodb.NewFromConfig(awsCfg), a.Config.RepositoriesTable),
		Secrets:        secrets.New(secretsmanager.NewFromConfig(awsCfg), a.Config.SecretsPrefix),
		Publisher:      eventbus.NewEventBridge(eventbridge.NewFromConfig(awsCfg), a.Config.EventBusName, a.Config.EventSource),
		Metrics:        metrics.New(metrics.Namespace, os.Stdout),
		DefaultTrigger: a.Config.PublishTrigger,
		Logger:         a.Logger,
	}

	// With PROJECT_NAME set, the handler reports processed-now vs deferred based
	// on whether the compute host is up (the fixed weekday window). Absent, every
	// matched delivery is reported "accepted".
	if a.Config.ProjectName != "" {
		handler.Gate = windowGate{
			ec2:     awsec2.New(ec2.NewFromConfig(awsCfg)),
			project: a.Config.ProjectName,
		}
	}

	lambda.Start(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		body := []byte(req.Body)
		if req.IsBase64Encoded {
			if decoded, derr := base64.StdEncoding.DecodeString(req.Body); derr == nil {
				body = decoded
			}
		}
		status, respBody := handler.Handle(ctx, req.Headers, body)
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(respBody),
		}, nil
	})
}
