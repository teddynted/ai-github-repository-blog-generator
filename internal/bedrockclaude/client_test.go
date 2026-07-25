package bedrockclaude

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type fakeInvoker struct {
	gotBody  []byte
	gotModel string
	out      string
	err      error
}

func (f *fakeInvoker) InvokeModel(_ context.Context, in *bedrockruntime.InvokeModelInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
	f.gotBody = in.Body
	if in.ModelId != nil {
		f.gotModel = *in.ModelId
	}
	if f.err != nil {
		return nil, f.err
	}
	resp := responseBody{Content: []contentPart{{Type: "text", Text: f.out}}, StopReason: "end_turn"}
	b, _ := json.Marshal(resp)
	return &bedrockruntime.InvokeModelOutput{Body: b}, nil
}

func TestGenerateBuildsAnthropicBodyAndReturnsText(t *testing.T) {
	fi := &fakeInvoker{out: "# A great blog\n\nBody."}
	c := New(fi, Config{System: "You are the lead engineer.", Temperature: 0.6})

	got, err := c.Generate(context.Background(), "Write about v0.3.0")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(got, "# A great blog") {
		t.Errorf("text = %q", got)
	}
	if c.modelID != DefaultModelID || fi.gotModel != DefaultModelID {
		t.Errorf("model id = %q / %q", c.modelID, fi.gotModel)
	}

	var req requestBody
	if err := json.Unmarshal(fi.gotBody, &req); err != nil {
		t.Fatalf("request body not valid JSON: %v", err)
	}
	if req.AnthropicVersion != anthropicVersion {
		t.Errorf("anthropic_version = %q", req.AnthropicVersion)
	}
	if req.MaxTokens != 4096 {
		t.Errorf("default max_tokens = %d, want 4096", req.MaxTokens)
	}
	if req.System != "You are the lead engineer." {
		t.Errorf("system = %q", req.System)
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" ||
		req.Messages[0].Content[0].Text != "Write about v0.3.0" {
		t.Errorf("messages = %+v", req.Messages)
	}
}

func TestGenerateEmptyPrompt(t *testing.T) {
	c := New(&fakeInvoker{}, Config{})
	if _, err := c.Generate(context.Background(), "  "); err == nil {
		t.Fatal("expected error on empty prompt")
	}
}

func TestGenerateInvokeError(t *testing.T) {
	c := New(&fakeInvoker{err: errors.New("access denied")}, Config{})
	_, err := c.Generate(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "invoke") {
		t.Fatalf("expected invoke error, got %v", err)
	}
}

func TestGenerateEmptyCompletion(t *testing.T) {
	c := New(&fakeInvoker{out: "   "}, Config{})
	if _, err := c.Generate(context.Background(), "hi"); err == nil {
		t.Fatal("expected error on empty completion")
	}
}

func TestNilClient(t *testing.T) {
	var c *Client
	if _, err := c.Generate(context.Background(), "hi"); err == nil {
		t.Fatal("expected error on nil client")
	}
}

func TestConfigDefaults(t *testing.T) {
	c := New(&fakeInvoker{out: "x"}, Config{ModelID: "custom.model:0", MaxTokens: 0})
	if c.modelID != "custom.model:0" {
		t.Errorf("model id = %q", c.modelID)
	}
	if c.maxTokens != 4096 {
		t.Errorf("maxTokens default = %d", c.maxTokens)
	}
}
