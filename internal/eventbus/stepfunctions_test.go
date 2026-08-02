package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/intake"
)

type fakeSFN struct {
	in  *sfn.StartExecutionInput
	err error
}

func (f *fakeSFN) StartExecution(_ context.Context, in *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	f.in = in
	if f.err != nil {
		return nil, f.err
	}
	return &sfn.StartExecutionOutput{ExecutionArn: aws.String("arn:execution")}, nil
}

func TestStepFunctionsPublishStartsExecutionWithDetailEnvelope(t *testing.T) {
	f := &fakeSFN{}
	p := NewStepFunctions(f, "arn:aws:states:us-east-1:1:stateMachine:blog-gen")
	if err := p.Publish(context.Background(), intake.Event{RepoFullName: "acme/widget", Source: "manual", ReleaseTag: "v1.2.3"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if aws.ToString(f.in.StateMachineArn) != "arn:aws:states:us-east-1:1:stateMachine:blog-gen" {
		t.Errorf("state machine arn = %s", aws.ToString(f.in.StateMachineArn))
	}
	// The execution input must be the exact SQS body the worker unmarshals:
	// {"detail": {...}} with the event's snake_case fields.
	var env struct {
		Detail struct {
			RepoFullName string `json:"repo_full_name"`
			ReleaseTag   string `json:"release_tag"`
		} `json:"detail"`
	}
	if err := json.Unmarshal([]byte(aws.ToString(f.in.Input)), &env); err != nil {
		t.Fatalf("input not valid JSON: %v", err)
	}
	if env.Detail.RepoFullName != "acme/widget" || env.Detail.ReleaseTag != "v1.2.3" {
		t.Errorf("detail = %+v", env.Detail)
	}
}

func TestStepFunctionsPublishPropagatesError(t *testing.T) {
	p := NewStepFunctions(&fakeSFN{err: errors.New("boom")}, "arn")
	if err := p.Publish(context.Background(), intake.Event{RepoFullName: "a/b"}); err == nil {
		t.Error("expected error from StartExecution")
	}
}
