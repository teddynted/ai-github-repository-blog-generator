// Command registration is the AWS Lambda entry point for repository
// onboarding. It is a thin adapter: bootstrap the app, construct the AWS/GitHub
// collaborators, and hand API Gateway requests to the registration handler.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/metadata"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/registration"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/secrets"
)

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion", "RepositoriesTable", "RepoSecretID", "WebhookURL"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	handler := &registration.Handler{
		Logger: a.Logger,
		Service: &registration.Service{
			GitHub:         github.New(),
			Secrets:        secrets.New(secretsmanager.NewFromConfig(awsCfg), a.Config.RepoSecretID),
			Metadata:       metadata.New(dynamodb.NewFromConfig(awsCfg), a.Config.RepositoriesTable),
			WebhookURL:     a.Config.WebhookURL,
			DefaultTrigger: a.Config.PublishTrigger,
		},
	}

	lambda.Start(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		body := []byte(req.Body)
		var status int
		var respBody []byte
		switch req.HTTPMethod {
		case http.MethodDelete:
			status, respBody = handler.Delete(ctx, body)
		default: // POST (register / update)
			status, respBody = handler.Register(ctx, body)
		}
		return events.APIGatewayProxyResponse{
			StatusCode: status,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(respBody),
		}, nil
	})
}
