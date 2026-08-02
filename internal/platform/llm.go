package platform

import (
	"context"
	"fmt"
	"strings"
)

// Capabilities commonly advertised by LLM providers.
const (
	CapTextGeneration Capability = "text-generation"
	CapStreaming      Capability = "streaming"
	CapChat           Capability = "chat"
	CapJSONMode       Capability = "json-mode"
	CapToolUse        Capability = "tool-use"
)

// LLMRequest is a provider-neutral generation request. New knobs are added as
// fields with zero-value defaults, preserving backward compatibility.
type LLMRequest struct {
	Prompt      string
	System      string
	MaxTokens   int
	Temperature float64
	Stop        []string
}

// LLMResponse is a provider-neutral generation response.
type LLMResponse struct {
	Text         string
	FinishReason string
	InputTokens  int
	OutputTokens int
	Model        string
}

// LLMProvider is the abstraction every managed (Bedrock, OpenAI, Anthropic,
// Gemini, Azure OpenAI, Cohere, Mistral, Together, Groq) and local (
// llama.cpp, vLLM, LM Studio) model implements. Adding a provider is a new file
// implementing this interface + a Register call — no business-logic change.
type LLMProvider interface {
	Provider
	Generate(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

// StreamingLLM is the optional streaming extension. A provider that supports
// token streaming implements it in addition to LLMProvider; callers feature-test
// with a type assertion, so non-streaming providers are unaffected.
type StreamingLLM interface {
	LLMProvider
	GenerateStream(ctx context.Context, req LLMRequest, onToken func(string) error) (LLMResponse, error)
}

// echoLLM is the reference LLM provider: deterministic, dependency-free, and
// offline. It demonstrates the contract (and doubles as a test/dev double). A
// production provider (Bedrock/OpenAI) replaces only the Generate body.
type echoLLM struct {
	id   string
	name string
}

func (e *echoLLM) ID() string { return e.id }
func (e *echoLLM) Kind() Kind { return KindLLM }
func (e *echoLLM) Capabilities() []Capability {
	return []Capability{CapTextGeneration, CapChat, CapStreaming}
}
func (e *echoLLM) Health(context.Context) Health { return OK() }

func (e *echoLLM) Generate(_ context.Context, req LLMRequest) (LLMResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return LLMResponse{}, fmt.Errorf("%s: empty prompt", e.id)
	}
	text := fmt.Sprintf("[%s] %s", e.name, req.Prompt)
	return LLMResponse{
		Text:         text,
		FinishReason: "stop",
		InputTokens:  len(strings.Fields(req.Prompt)),
		OutputTokens: len(strings.Fields(text)),
		Model:        e.id,
	}, nil
}

// GenerateStream streams the response word-by-word, demonstrating the optional
// StreamingLLM extension.
func (e *echoLLM) GenerateStream(ctx context.Context, req LLMRequest, onToken func(string) error) (LLMResponse, error) {
	resp, err := e.Generate(ctx, req)
	if err != nil {
		return resp, err
	}
	for _, tok := range strings.Fields(resp.Text) {
		if err := onToken(tok + " "); err != nil {
			return resp, err
		}
	}
	return resp, nil
}

// NewEchoLLM builds the reference LLM provider (exported for demos/tests).
func NewEchoLLM(id, name string) LLMProvider {
	return &echoLLM{id: id, name: name}
}

func registerLLM(r *Registry) {
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "echo", Kind: KindLLM, Name: "Echo (reference)", Version: "1.0.0",
			Priority: 1, Managed: false, Description: "Deterministic reference LLM",
			Capabilities: []Capability{CapTextGeneration, CapChat, CapStreaming},
		},
		Factory: func(cfg ConfigSource) (Provider, error) {
			name := GetDefault(cfg, "ECHO_LLM_NAME", "echo")
			return &echoLLM{id: "echo", name: name}, nil
		},
	})
}
