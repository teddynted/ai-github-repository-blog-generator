// Command webhook-handler is the AWS Lambda entry point for GitHub webhook
// deliveries. It stays intentionally lightweight: resolve metadata, verify the
// signature, evaluate the commit-message trigger, and (on a match) publish an
// event. It performs no cloning, analysis, or AI work.
package main

import (
	"context"
	"encoding/base64"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/eventbus"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metadata"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/secrets"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/webhook"
)

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion", "RepositoriesTable", "SecretsPrefix"); err != nil {
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
		Publisher:      &eventbus.LogPublisher{Logger: a.Logger}, // EventBridge in Milestone 5
		DefaultTrigger: a.Config.PublishTrigger,
		Logger:         a.Logger,
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
