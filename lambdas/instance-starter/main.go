// Command instance-starter is the AWS Lambda invoked by EventBridge when a
// matched publishing event is emitted. It ensures the platform's EC2 Spot
// instance (located by its Project tag) is running. Invoking the n8n workflow
// once the host is healthy is added in Milestone 6.
package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awsec2"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/lifecycle"
)

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion", "ProjectName"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	starter := &lifecycle.Starter{
		EC2:     awsec2.New(ec2.NewFromConfig(awsCfg)),
		Project: a.Config.ProjectName,
		Logger:  a.Logger,
	}

	lambda.Start(func(ctx context.Context, event json.RawMessage) error {
		// The event identifies the repository; for the MVP the starter only
		// needs to ensure the tagged instance is running. The n8n invocation
		// (using the event) is added in Milestone 6.
		started, err := starter.EnsureRunning(ctx)
		if err != nil {
			a.Logger.Error("ensure running failed", "error", err.Error())
			return err
		}
		a.Logger.Info("instance ensured running", "start_issued", started)
		return nil
	})
}
