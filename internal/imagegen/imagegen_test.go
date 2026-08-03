package imagegen

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// fakeInvoker returns queued responses/errors in order, recording call count.
type fakeInvoker struct {
	calls   int
	replies []reply
}

type reply struct {
	body string
	err  error
}

func (f *fakeInvoker) InvokeModel(_ context.Context, _ *bedrockruntime.InvokeModelInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
	i := f.calls
	f.calls++
	if i >= len(f.replies) {
		return nil, errors.New("unexpected extra call")
	}
	r := f.replies[i]
	if r.err != nil {
		return nil, r.err
	}
	return &bedrockruntime.InvokeModelOutput{Body: []byte(r.body)}, nil
}

func testClient(f *fakeInvoker) *Client {
	return &Client{api: f, modelID: DefaultModelID, sleep: func(time.Duration) {}}
}

func okBody(png []byte) string {
	return `{"images":["` + base64.StdEncoding.EncodeToString(png) + `"]}`
}

func TestGenerateSuccess(t *testing.T) {
	want := []byte("\x89PNG-bytes")
	c := testClient(&fakeInvoker{replies: []reply{{body: okBody(want)}}})
	got, err := c.Generate(context.Background(), Spec{Prompt: "a diagram", Width: 1280, Height: 720})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("png = %q, want %q", got, want)
	}
}

func TestGenerateRetriesThrottleThenSucceeds(t *testing.T) {
	want := []byte("img")
	f := &fakeInvoker{replies: []reply{
		{err: errors.New("ServiceUnavailableException: Too many connections")},
		{err: errors.New("ThrottlingException: Too many requests")},
		{body: okBody(want)},
	}}
	c := testClient(f)
	got, err := c.Generate(context.Background(), Spec{Prompt: "x", Width: 720, Height: 1280})
	if err != nil {
		t.Fatalf("Generate should recover after throttling: %v", err)
	}
	if string(got) != string(want) || f.calls != 3 {
		t.Errorf("calls=%d png=%q, want 3 calls and %q", f.calls, got, want)
	}
}

func TestGenerateDoesNotRetryNonThrottle(t *testing.T) {
	f := &fakeInvoker{replies: []reply{{err: errors.New("AccessDeniedException: nope")}}}
	c := testClient(f)
	if _, err := c.Generate(context.Background(), Spec{Prompt: "x", Width: 512, Height: 512}); err == nil {
		t.Fatal("expected an error for a non-throttle failure")
	}
	if f.calls != 1 {
		t.Errorf("a non-throttle error must not retry, got %d calls", f.calls)
	}
}

func TestGenerateGivesUpAfterMaxRetries(t *testing.T) {
	var replies []reply
	for i := 0; i <= maxRetries; i++ {
		replies = append(replies, reply{err: errors.New("Too many connections")})
	}
	f := &fakeInvoker{replies: replies}
	c := testClient(f)
	_, err := c.Generate(context.Background(), Spec{Prompt: "x", Width: 512, Height: 512})
	if err == nil || !strings.Contains(err.Error(), "throttled after") {
		t.Fatalf("expected exhausted-retry error, got %v", err)
	}
	if f.calls != maxRetries+1 {
		t.Errorf("want %d attempts, got %d", maxRetries+1, f.calls)
	}
}

func TestGenerateRejectsEmptyPrompt(t *testing.T) {
	c := testClient(&fakeInvoker{})
	if _, err := c.Generate(context.Background(), Spec{Prompt: "  ", Width: 512, Height: 512}); err == nil {
		t.Error("empty prompt should error before invoking the model")
	}
}

func TestGenerateSurfacesModelError(t *testing.T) {
	c := testClient(&fakeInvoker{replies: []reply{{body: `{"error":"content filtered"}`}}})
	if _, err := c.Generate(context.Background(), Spec{Prompt: "x", Width: 512, Height: 512}); err == nil || !strings.Contains(err.Error(), "content filtered") {
		t.Errorf("model error should surface, got %v", err)
	}
}

func TestGenerateEmptyImagesIsError(t *testing.T) {
	c := testClient(&fakeInvoker{replies: []reply{{body: `{"images":[]}`}}})
	if _, err := c.Generate(context.Background(), Spec{Prompt: "x", Width: 512, Height: 512}); err == nil {
		t.Error("no images returned should error")
	}
}
