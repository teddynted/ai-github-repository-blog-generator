package awssqs

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/lifecycle"
)

type fakeAPI struct {
	attrs         map[string]string
	in            *sqs.GetQueueAttributesInput
	messages      []sqstypes.Message
	deletedHandle string
	recvIn        *sqs.ReceiveMessageInput
}

func (f *fakeAPI) GetQueueAttributes(_ context.Context, in *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	f.in = in
	return &sqs.GetQueueAttributesOutput{Attributes: f.attrs}, nil
}

func (f *fakeAPI) ReceiveMessage(_ context.Context, in *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	f.recvIn = in
	return &sqs.ReceiveMessageOutput{Messages: f.messages}, nil
}

func (f *fakeAPI) DeleteMessage(_ context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.deletedHandle = *in.ReceiptHandle
	return &sqs.DeleteMessageOutput{}, nil
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

func TestReceiveMapsMessages(t *testing.T) {
	f := &fakeAPI{messages: []sqstypes.Message{
		{Body: aws.String(`{"detail":{}}`), ReceiptHandle: aws.String("rh-1")},
	}}
	msgs, err := New(f, "u").Receive(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ReceiptHandle != "rh-1" || msgs[0].Body != `{"detail":{}}` {
		t.Errorf("messages = %+v", msgs)
	}
	if f.recvIn.MaxNumberOfMessages != 10 || f.recvIn.WaitTimeSeconds != 20 {
		t.Errorf("receive params = max:%d wait:%d", f.recvIn.MaxNumberOfMessages, f.recvIn.WaitTimeSeconds)
	}
}

func TestDelete(t *testing.T) {
	f := &fakeAPI{}
	if err := New(f, "u").Delete(context.Background(), "rh-9"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if f.deletedHandle != "rh-9" {
		t.Errorf("deleted handle = %q", f.deletedHandle)
	}
}

// Guard: *Client satisfies the lifecycle.Queue port.
var _ lifecycle.Queue = (*Client)(nil)
