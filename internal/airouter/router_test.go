package airouter

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeModel struct {
	out string
	err error
}

func (f fakeModel) Generate(_ context.Context, _ string) (string, error) { return f.out, f.err }

func prov(name, model string, m Model) Provider { return Provider{Name: name, Model: model, Client: m} }

func TestPrimaryServesWhenBedrockSucceeds(t *testing.T) {
	r := New([]Provider{
		prov(ProviderBedrock, "us.anthropic.claude-opus-4-8", fakeModel{out: "bedrock-out"}),
		prov(ProviderAnthropic, "claude-opus-4-8", fakeModel{out: "anthropic-out"}),
	}, nil)
	got, err := r.ModelFor("blog").Generate(context.Background(), "p")
	if err != nil || got != "bedrock-out" {
		t.Fatalf("expected primary (bedrock), got %q err=%v", got, err)
	}
	if n, m := r.Primary(); n != ProviderBedrock || m != "us.anthropic.claude-opus-4-8" {
		t.Errorf("primary = %s/%s", n, m)
	}
}

func TestFallsBackToAnthropicOnBedrockQuota(t *testing.T) {
	r := New([]Provider{
		prov(ProviderBedrock, "us.anthropic.claude-opus-4-8", fakeModel{err: errors.New("ThrottlingException: rate exceeded")}),
		prov(ProviderAnthropic, "claude-opus-4-8", fakeModel{out: "anthropic-out"}),
	}, nil)
	got, err := r.ModelFor("blog").Generate(context.Background(), "p")
	if err != nil || got != "anthropic-out" {
		t.Fatalf("expected fallback to anthropic, got %q err=%v", got, err)
	}
}

func TestQuotaClassification(t *testing.T) {
	for _, q := range []string{"ServiceQuotaExceededException", "ThrottlingException", "Too Many Requests (429)", "insufficient capacity"} {
		if fallbackReason(errors.New(q)) != "quota" {
			t.Errorf("%q should classify as quota", q)
		}
	}
	if fallbackReason(errors.New("invalid request: bad prompt")) != "error" {
		t.Error("non-quota error should classify as 'error'")
	}
}

func TestAllProvidersFailReturnsCombinedError(t *testing.T) {
	r := New([]Provider{
		prov(ProviderBedrock, "b", fakeModel{err: errors.New("throttled")}),
		prov(ProviderAnthropic, "a", fakeModel{err: errors.New("boom")}),
	}, nil)
	_, err := r.ModelFor("blog").Generate(context.Background(), "p")
	if err == nil || !strings.Contains(err.Error(), "all providers failed") {
		t.Fatalf("expected combined failure, got %v", err)
	}
}

func TestAnthropicOnlyChain(t *testing.T) {
	r := New([]Provider{prov(ProviderAnthropic, "claude-opus-4-8", fakeModel{out: "a"})}, nil)
	got, err := r.ModelFor("blog").Generate(context.Background(), "p")
	if err != nil || got != "a" {
		t.Fatalf("anthropic-only: got %q err=%v", got, err)
	}
	if c := r.Chain(); len(c) != 1 || c[0] != "anthropic:claude-opus-4-8" {
		t.Errorf("chain = %v", c)
	}
}
