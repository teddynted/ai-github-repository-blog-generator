package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/intake"
)

type fakeEB struct {
	in     *eventbridge.PutEventsInput
	failed int32
	err    error
}

func (f *fakeEB) PutEvents(_ context.Context, in *eventbridge.PutEventsInput, _ ...func(*eventbridge.Options)) (*eventbridge.PutEventsOutput, error) {
	f.in = in
	if f.err != nil {
		return nil, f.err
	}
	return &eventbridge.PutEventsOutput{FailedEntryCount: f.failed}, nil
}

func sampleEvent() intake.Event {
	return intake.Event{
		RepoFullName: "acme/widget", Owner: "acme", Name: "widget",
		Ref: "refs/heads/main", CommitSHA: "abc", CommitMessage: "blog: x", TriggerPattern: "blog:",
	}
}

func TestPublishSendsCorrectEntry(t *testing.T) {
	f := &fakeEB{}
	p := NewEventBridge(f, "blog-gen-bus", "blog-gen.webhook")

	if err := p.Publish(context.Background(), sampleEvent()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(f.in.Entries) != 1 {
		t.Fatalf("entries = %d", len(f.in.Entries))
	}
	e := f.in.Entries[0]
	if *e.EventBusName != "blog-gen-bus" || *e.Source != "blog-gen.webhook" || *e.DetailType != DetailType {
		t.Errorf("entry = bus:%s source:%s type:%s", *e.EventBusName, *e.Source, *e.DetailType)
	}
	var got intake.Event
	if err := json.Unmarshal([]byte(*e.Detail), &got); err != nil {
		t.Fatalf("detail not valid json: %v", err)
	}
	if got.RepoFullName != "acme/widget" || got.CommitSHA != "abc" {
		t.Errorf("detail = %+v", got)
	}
}

func TestPublishFailsOnFailedEntries(t *testing.T) {
	p := NewEventBridge(&fakeEB{failed: 1}, "bus", "src")
	if err := p.Publish(context.Background(), sampleEvent()); err == nil {
		t.Error("expected error when FailedEntryCount > 0")
	}
}

func TestPublishPropagatesAPIError(t *testing.T) {
	p := NewEventBridge(&fakeEB{err: errors.New("throttled")}, "bus", "src")
	if err := p.Publish(context.Background(), sampleEvent()); err == nil {
		t.Error("expected error from PutEvents")
	}
}

// Guard: EventBridgePublisher satisfies the intake.Publisher port.
var _ intake.Publisher = (*EventBridgePublisher)(nil)
