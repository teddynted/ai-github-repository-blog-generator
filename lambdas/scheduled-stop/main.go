// Command scheduled-stop is the AWS Lambda invoked by the EventBridge Scheduler
// "stop" schedule (default 08:00 in the configured timezone). It stops the EC2
// instance named by INSTANCE_ID, and is idempotent: if the instance is already
// stopped the invocation succeeds without action.
package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awsec2"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/power"
)

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion", "InstanceID"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	// Adaptive retries + extra attempts so transient EC2 API errors and
	// throttling (RequestLimitExceeded) are absorbed by the SDK's retry
	// middleware rather than failing the schedule.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(a.Config.AWSRegion),
		awsconfig.WithRetryMode(aws.RetryModeAdaptive),
		awsconfig.WithRetryMaxAttempts(5),
	)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	sw := &power.Switch{
		Instances:  awsec2.New(ec2.NewFromConfig(awsCfg)),
		InstanceID: a.Config.InstanceID,
		Logger:     a.Logger,
	}

	lambda.Start(func(ctx context.Context, _ json.RawMessage) error {
		stopped, err := sw.EnsureStopped(ctx)
		if err != nil {
			a.Logger.Error("scheduled stop failed", "error", err.Error())
			return err
		}
		a.Logger.Info("scheduled stop complete", "stop_issued", stopped)
		return nil
	})
}
