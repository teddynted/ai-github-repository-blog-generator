package retry

import (
	"context"
	"errors"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// fastConfig avoids real sleeping (zero delay => immediate).
var fastConfig = Config{MaxAttempts: 3, BaseDelay: 0}

func TestDoSucceedsFirstTry(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastConfig, func() error { calls++; return nil })
	if err != nil || calls != 1 {
		t.Errorf("calls=%d err=%v", calls, err)
	}
}

func TestDoRetriesTransientThenSucceeds(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastConfig, func() error {
		calls++
		if calls < 3 {
			return apperror.New(apperror.CodeUpstream, "transient")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Errorf("calls=%d err=%v", calls, err)
	}
}

func TestDoStopsAtMaxAttempts(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastConfig, func() error {
		calls++
		return apperror.New(apperror.CodeUpstream, "always transient")
	})
	if err == nil || calls != 3 {
		t.Errorf("expected exhaustion: calls=%d err=%v", calls, err)
	}
}

func TestDoFailsFastOnNonRetryable(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastConfig, func() error {
		calls++
		return apperror.New(apperror.CodeUnauthorized, "bad token")
	})
	if err == nil || calls != 1 {
		t.Errorf("non-retryable should not retry: calls=%d err=%v", calls, err)
	}
}

func TestDoRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	calls := 0
	err := Do(ctx, Config{MaxAttempts: 5, BaseDelay: 1}, func() error {
		calls++
		return apperror.New(apperror.CodeUpstream, "transient")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v (calls=%d)", err, calls)
	}
}

func TestIsRetryable(t *testing.T) {
	cases := map[error]bool{
		apperror.New(apperror.CodeUpstream, ""):     true,
		apperror.New(apperror.CodeUnavailable, ""):  true,
		apperror.New(apperror.CodeUnauthorized, ""): false,
		apperror.New(apperror.CodeInvalidInput, ""): false,
		apperror.New(apperror.CodeNotFound, ""):     false,
		errors.New("plain"):                         false,
	}
	for err, want := range cases {
		if got := IsRetryable(err); got != want {
			t.Errorf("IsRetryable(%v) = %v, want %v", err, got, want)
		}
	}
}
