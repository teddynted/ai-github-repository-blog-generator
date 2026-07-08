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

func attr(m map[string]string, key string) int64 {
	v, err := strconv.ParseInt(m[key], 10, 64)
	if err != nil {
		return 0
	}
	return v
}
