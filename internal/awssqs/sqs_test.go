package awssqs

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/lifecycle"
)

type fakeAPI struct {
	attrs map[string]string
	in    *sqs.GetQueueAttributesInput
}

func (f *fakeAPI) GetQueueAttributes(_ context.Context, in *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	f.in = in
	return &sqs.GetQueueAttributesOutput{Attributes: f.attrs}, nil
}

func TestDepthSumsVisibleAndInflight(t *testing.T) {
	f := &fakeAPI{attrs: map[string]string{
		string(sqstypes.QueueAttributeNameApproximateNumberOfMessages):           "4",
		string(sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible): "2",
	}}
	d, err := New(f, "https://sqs/queue").Depth(context.Background())
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if d != 6 {
		t.Errorf("Depth = %d, want 6", d)
	}
	if *f.in.QueueUrl != "https://sqs/queue" {
		t.Errorf("queue url = %q", *f.in.QueueUrl)
	}
}

func TestDepthEmptyQueue(t *testing.T) {
	f := &fakeAPI{attrs: map[string]string{}}
	d, err := New(f, "u").Depth(context.Background())
	if err != nil || d != 0 {
		t.Errorf("empty queue: d=%d err=%v", d, err)
	}
}

// Guard: *Client satisfies the lifecycle.Queue port.
var _ lifecycle.Queue = (*Client)(nil)
