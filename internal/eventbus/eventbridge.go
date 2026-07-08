package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/webhook"
)

// DetailType is the EventBridge detail-type for a matched publishing request.
// The serverless stack's rule pattern keys off this value.
const DetailType = "blog.publish.requested"

// EBAPI is the subset of the EventBridge client this adapter uses.
type EBAPI interface {
	PutEvents(ctx context.Context, in *eventbridge.PutEventsInput, optFns ...func(*eventbridge.Options)) (*eventbridge.PutEventsOutput, error)
}

// EventBridgePublisher publishes matched events onto the custom bus. It
// satisfies webhook.Publisher.
type EventBridgePublisher struct {
	api      EBAPI
	busName  string
	source   string
	detailFn func(webhook.Event) (string, error)
}

// NewEventBridge builds a publisher for the given bus and source.
func NewEventBridge(api EBAPI, busName, source string) *EventBridgePublisher {
	return &EventBridgePublisher{api: api, busName: busName, source: source, detailFn: marshalDetail}
}

// Publish emits one blog.publish.requested event carrying the matched-event
// detail. A partial failure (FailedEntryCount > 0) is treated as an error so
// the delivery is retried.
func (p *EventBridgePublisher) Publish(ctx context.Context, ev webhook.Event) error {
	detail, err := p.detailFn(ev)
	if err != nil {
		return fmt.Errorf("marshal event detail: %w", err)
	}
	out, err := p.api.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []ebtypes.PutEventsRequestEntry{{
			EventBusName: aws.String(p.busName),
			Source:       aws.String(p.source),
			DetailType:   aws.String(DetailType),
			Detail:       aws.String(detail),
		}},
	})
	if err != nil {
		return fmt.Errorf("put events: %w", err)
	}
	if out.FailedEntryCount > 0 {
		return fmt.Errorf("eventbridge rejected %d entr(y/ies)", out.FailedEntryCount)
	}
	return nil
}

func marshalDetail(ev webhook.Event) (string, error) {
	b, err := json.Marshal(ev)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
