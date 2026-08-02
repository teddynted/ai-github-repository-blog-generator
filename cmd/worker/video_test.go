package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasepipeline"
)

type fakeRelease struct {
	err  error
	runs int
}

func (f *fakeRelease) Run(context.Context, rc.Request) (releasepipeline.Result, error) {
	f.runs++
	return releasepipeline.Result{Published: 3}, f.err
}

type fakeSFN struct {
	in   *sfn.StartExecutionInput
	err  error
	call int
}

func (f *fakeSFN) StartExecution(_ context.Context, in *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	f.call++
	f.in = in
	return &sfn.StartExecutionOutput{}, f.err
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestVideoTriggerStartsOnSuccess(t *testing.T) {
	inner, sf := &fakeRelease{}, &fakeSFN{}
	v := &videoTriggeringRunner{inner: inner, sfn: sf, arn: "arn:sm:video", bucket: "content-b", prefix: "generated-content", logger: discard()}

	res, err := v.Run(context.Background(), rc.Request{Owner: "acme", Repository: "widget", ReleaseTag: "v1.2.0"})
	if err != nil || res.Published != 3 {
		t.Fatalf("run: err=%v res=%+v", err, res)
	}
	if sf.call != 1 || aws.ToString(sf.in.StateMachineArn) != "arn:sm:video" {
		t.Fatalf("StartExecution not called correctly: call=%d", sf.call)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(aws.ToString(sf.in.Input)), &got); err != nil {
		t.Fatalf("input json: %v", err)
	}
	if got["owner"] != "acme" || got["name"] != "widget" || got["tag"] != "v1.2.0" || got["bucket"] != "content-b" || got["prefix"] != "generated-content" {
		t.Errorf("input = %+v", got)
	}
}

func TestVideoTriggerSkippedOnReleaseFailure(t *testing.T) {
	inner, sf := &fakeRelease{err: errors.New("boom")}, &fakeSFN{}
	v := &videoTriggeringRunner{inner: inner, sfn: sf, arn: "arn", bucket: "b", logger: discard()}
	if _, err := v.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1"}); err == nil {
		t.Fatal("expected the inner error to propagate")
	}
	if sf.call != 0 {
		t.Error("video must not start when the release run failed")
	}
}

func TestVideoTriggerFailureIsNonFatal(t *testing.T) {
	inner, sf := &fakeRelease{}, &fakeSFN{err: errors.New("throttled")}
	v := &videoTriggeringRunner{inner: inner, sfn: sf, arn: "arn", bucket: "b", logger: discard()}
	// A StartExecution error must NOT fail the (already-published) release run.
	if res, err := v.Run(context.Background(), rc.Request{Owner: "a", Repository: "b", ReleaseTag: "v1"}); err != nil || res.Published != 3 {
		t.Fatalf("video failure should be swallowed: err=%v res=%+v", err, res)
	}
}
