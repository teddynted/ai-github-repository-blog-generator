// Package awssqs adapts Amazon SQS to the lifecycle.Queue port, reporting the
// events queue depth (visible + in-flight) so the idle-shutdown use case knows
// whether there is outstanding work.
package awssqs

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// API is the subset of the SQS client this adapter uses.
type API interface {
	GetQueueAttributes(ctx context.Context, in *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// Message is a received SQS message (the fields a consumer needs).
type Message struct {
	Body          string
	ReceiptHandle string
}

// Client reads the depth of a single queue.
type Client struct {
	api      API
	queueURL string
}

// New builds a Client for the given queue URL.
func New(api API, queueURL string) *Client {
	return &Client{api: api, queueURL: queueURL}
}

// Depth returns the number of messages that are visible plus those in flight.
// A depth of zero means the queue is fully drained.
func (c *Client) Depth(ctx context.Context) (int64, error) {
	out, err := c.api.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(c.queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeNameApproximateNumberOfMessages,
			sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("get queue attributes: %w", err)
	}
	visible := attr(out.Attributes, string(sqstypes.QueueAttributeNameApproximateNumberOfMessages))
	inflight := attr(out.Attributes, string(sqstypes.QueueAttributeNameApproximateNumberOfMessagesNotVisible))
	return visible + inflight, nil
}

// Receive long-polls for up to maxMessages, waiting up to waitSeconds.
func (c *Client) Receive(ctx context.Context, maxMessages, waitSeconds int32) ([]Message, error) {
	out, err := c.api.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("receive message: %w", err)
	}
	msgs := make([]Message, 0, len(out.Messages))
	for _, m := range out.Messages {
		msgs = append(msgs, Message{Body: aws.ToString(m.Body), ReceiptHandle: aws.ToString(m.ReceiptHandle)})
	}
	return msgs, nil
}

// Delete removes a processed message so it is not redelivered.
func (c *Client) Delete(ctx context.Context, receiptHandle string) error {
	_, err := c.api.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	return nil
}

func attr(m map[string]string, key string) int64 {
	v, err := strconv.ParseInt(m[key], 10, 64)
	if err != nil {
		return 0
	}
	return v
}
