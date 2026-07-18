package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3API is the subset of the S3 client the store uses.
type s3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// s3Store persists each Release Context as a JSON object, keyed by date and
// context id, so downstream milestones can read it back.
type s3Store struct {
	client s3API
	bucket string
}

// Save writes the context JSON to s3://<bucket>/release-contexts/<date>/<id>.json
// and returns the s3:// location.
func (s *s3Store) Save(ctx context.Context, id string, data []byte) (string, error) {
	key := fmt.Sprintf("release-contexts/%s/%s.json", time.Now().UTC().Format("2006/01/02"), id)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("s3://%s/%s", s.bucket, key), nil
}
