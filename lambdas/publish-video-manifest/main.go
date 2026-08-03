// Command publish-video-manifest is the final state of the video state machine.
// It collects the per-format render results from the Parallel state, writes a
// manifest.json (the rendered MP4 S3 URIs) into the video bucket, and publishes
// a summary to SNS for downstream publishing workflows (n8n).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// videoResult is one Parallel branch's output.
type videoResult struct {
	Format    string `json:"format"`
	Status    string `json:"status"` // rendered | failed
	OutputURI string `json:"outputUri"`
	Error     string `json:"error,omitempty"`
}

// input is the manifest state's payload.
type input struct {
	Owner   string        `json:"owner"`
	Name    string        `json:"name"`
	Tag     string        `json:"tag"`
	Results []videoResult `json:"results"`
}

// manifest is written to S3 and returned to the caller.
type manifest struct {
	Repo        string        `json:"repo"`
	Version     string        `json:"version"`
	GeneratedAt string        `json:"generatedAt"`
	ManifestURI string        `json:"manifestUri"`
	Videos      []videoResult `json:"videos"`
}

type s3API interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}
type snsAPI interface {
	Publish(context.Context, *sns.PublishInput, ...func(*sns.Options)) (*sns.PublishOutput, error)
}

func main() {
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	bucket := os.Getenv("VIDEO_BUCKET")
	topic := os.Getenv("SNS_TOPIC_ARN")
	s3c := s3.NewFromConfig(cfg)
	snsc := sns.NewFromConfig(cfg)
	lambda.Start(func(ctx context.Context, in input) (manifest, error) {
		return publish(ctx, s3c, snsc, bucket, topic, in, time.Now().UTC())
	})
}

// buildManifest assembles the manifest from the render results.
func buildManifest(in input, bucket string, now time.Time) manifest {
	key := manifestKey(in.Owner, in.Name, in.Tag)
	return manifest{
		Repo:        in.Owner + "/" + in.Name,
		Version:     in.Tag,
		GeneratedAt: now.Format(time.RFC3339),
		ManifestURI: "s3://" + bucket + "/" + key,
		Videos:      in.Results,
	}
}

// publish writes the manifest to S3 and notifies SNS. A missing SNS topic (or
// bucket) is tolerated so the state machine still returns the manifest.
func publish(ctx context.Context, s3c s3API, snsc snsAPI, bucket, topic string, in input, now time.Time) (manifest, error) {
	m := buildManifest(in, bucket, now)
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m, fmt.Errorf("marshal manifest: %w", err)
	}
	if bucket != "" {
		if _, err := s3c.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(manifestKey(in.Owner, in.Name, in.Tag)),
			Body:        bytes.NewReader(body),
			ContentType: aws.String("application/json"),
		}); err != nil {
			return m, fmt.Errorf("put manifest: %w", err)
		}
	}
	if topic != "" {
		rendered := 0
		for _, v := range in.Results {
			if v.Status == "rendered" {
				rendered++
			}
		}
		subject := fmt.Sprintf("Videos ready: %s %s (%d/%d)", m.Repo, m.Version, rendered, len(in.Results))
		if _, err := snsc.Publish(ctx, &sns.PublishInput{
			TopicArn: aws.String(topic),
			Subject:  aws.String(subject),
			Message:  aws.String(string(body)),
		}); err != nil {
			return m, fmt.Errorf("sns publish: %w", err)
		}
	}
	return m, nil
}
