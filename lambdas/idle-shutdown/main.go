// Command idle-shutdown is the AWS Lambda invoked on a schedule by the
// EventBridge idle timer. It stops the platform's EC2 Spot instance once it has
// been up beyond the idle timeout and the events queue is fully drained,
// keeping compute cost proportional to actual work.
package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/app"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awsec2"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/awssqs"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/lifecycle"
)

func main() {
	a, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	if err := a.Config.Require("AWSRegion", "ProjectName", "QueueURL"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(a.Config.AWSRegion))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	shutdowner := &lifecycle.Shutdowner{
		EC2:         awsec2.New(ec2.NewFromConfig(awsCfg)),
		Queue:       awssqs.New(sqs.NewFromConfig(awsCfg), a.Config.QueueURL),
		Project:     a.Config.ProjectName,
		IdleTimeout: time.Duration(a.Config.IdleTimeoutMinutes) * time.Minute,
		Logger:      a.Logger,
	}

	lambda.Start(func(ctx context.Context, _ json.RawMessage) error {
		stopped, err := shutdowner.StopIfIdle(ctx)
		if err != nil {
			a.Logger.Error("idle check failed", "error", err.Error())
			return err
		}
		a.Logger.Info("idle check complete", "stop_issued", stopped)
		return nil
	})
}
