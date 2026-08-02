package main

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type fakeS3 struct {
	key, body string
	puts      int
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.puts++
	f.key = aws.ToString(in.Key)
	b, _ := io.ReadAll(in.Body)
	f.body = string(b)
	return &s3.PutObjectOutput{}, nil
}

type fakeSNS struct {
	subject string
	pubs    int
}

func (f *fakeSNS) Publish(_ context.Context, in *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	f.pubs++
	f.subject = aws.ToString(in.Subject)
	return &sns.PublishOutput{}, nil
}

func sample() input {
	return input{
		Owner: "acme", Name: "widget", Tag: "v1.2.0",
		Results: []videoResult{
			{Format: "youtube", Status: "rendered", OutputURI: "s3://vb/acme/widget/v1.2.0/youtube/final-youtube.mp4"},
			{Format: "youtube-shorts", Status: "rendered", OutputURI: "s3://vb/.../final-youtube-shorts.mp4"},
			{Format: "tiktok", Status: "failed", OutputURI: "s3://vb/.../final-tiktok.mp4", Error: "States.TaskFailed"},
		},
	}
}

func TestPublishWritesManifestAndNotifies(t *testing.T) {
	s3c, snsc := &fakeS3{}, &fakeSNS{}
	m, err := publish(context.Background(), s3c, snsc, "vb", "arn:sns:topic", sample(), time.Unix(1700000000, 0).UTC())
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if s3c.puts != 1 || s3c.key != "acme/widget/v1.2.0/videos/manifest.json" {
		t.Errorf("manifest key = %q (puts=%d)", s3c.key, s3c.puts)
	}
	if snsc.pubs != 1 || !strings.Contains(snsc.subject, "2/3") {
		t.Errorf("sns subject = %q (pubs=%d)", snsc.subject, snsc.pubs)
	}
	if m.Repo != "acme/widget" || m.Version != "v1.2.0" || len(m.Videos) != 3 {
		t.Errorf("manifest = %+v", m)
	}
	// The written body is the manifest JSON.
	var got manifest
	if err := json.Unmarshal([]byte(s3c.body), &got); err != nil || got.ManifestURI != "s3://vb/acme/widget/v1.2.0/videos/manifest.json" {
		t.Errorf("written manifest: err=%v uri=%q", err, got.ManifestURI)
	}
}

func TestPublishToleratesNoTopicOrBucket(t *testing.T) {
	s3c, snsc := &fakeS3{}, &fakeSNS{}
	if _, err := publish(context.Background(), s3c, snsc, "", "", sample(), time.Now()); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if s3c.puts != 0 || snsc.pubs != 0 {
		t.Errorf("expected no calls with blank bucket/topic: puts=%d pubs=%d", s3c.puts, snsc.pubs)
	}
}
