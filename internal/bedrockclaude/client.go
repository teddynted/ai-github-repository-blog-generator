// Package bedrockclaude is the Stage 3 technical-writer model: Anthropic Claude
// served through Amazon Bedrock. It implements the same minimal generate port
// (Generate(ctx, prompt) -> string) that every content generator already
// depends on, so Claude drops in exactly where the local Ollama model was —
// with no change to the generators themselves.
//
// Bedrock is chosen over the direct Anthropic API deliberately: authentication
// is IAM (the instance role), so there is NO API key to store, rotate, or leak;
// traffic stays inside AWS; and it fits the platform's secure-by-default,
// no-hardcoded-credentials posture. The model id, token budget, temperature and
// system prompt are all configurable; credentials come from the default AWS
// chain (instance role in production, profile/env in dev).
package bedrockclaude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// DefaultModelID is a widely-available Bedrock Claude model. Override with the
// model your account has access to (see NewFromAWS / Config.ModelID) — Bedrock
// model ids and inference-profile ids are account- and region-specific.
const DefaultModelID = "anthropic.claude-3-5-sonnet-20240620-v1:0"

// WriterPersona is the default system prompt for the technical-writer stage. It
// steers Claude to write as the engineer who built the platform — a narrative,
// decision-driven voice — and away from the generic, release-notes register
// that the redesign exists to eliminate.
const WriterPersona = "You are the lead software engineer who designed and built this platform, writing a technical blog post for other senior engineers. " +
	"Write a first-person engineering narrative that explains the problem, the decisions you made and why, the trade-offs you weighed, and what you learned — grounded strictly in the provided engineering analysis and release context. " +
	"Never invent facts. Avoid generic AI openings, marketing language, and long bullet lists; prefer prose, concrete detail, and a strong point of view. Do not write release notes."

// anthropicVersion is the Bedrock-pinned Anthropic API version for the
// Messages API request body.
const anthropicVersion = "bedrock-2023-05-31"

// invoker is the minimal Bedrock Runtime surface the client needs, so the
// client is unit-testable with a fake (the real *bedrockruntime.Client
// satisfies it).
type invoker interface {
	InvokeModel(ctx context.Context, in *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

// Config parameterises the client. Zero values are filled with sensible
// defaults by New.
type Config struct {
	ModelID     string
	MaxTokens   int
	Temperature float64
	// System is the default system prompt applied to every request (the writer
	// persona). Callers may leave it empty and bake the persona into the prompt.
	System string
}

// Client generates text with Claude on Bedrock.
type Client struct {
	api         invoker
	modelID     string
	maxTokens   int
	temperature float64
	system      string
}

// New builds a Client from an already-configured Bedrock Runtime API. Use this
// when you manage AWS config yourself or in tests (pass a fake invoker).
func New(api invoker, cfg Config) *Client {
	c := &Client{
		api:         api,
		modelID:     cfg.ModelID,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		system:      cfg.System,
	}
	if c.modelID == "" {
		c.modelID = DefaultModelID
	}
	if c.maxTokens <= 0 {
		// 8192 leaves ample headroom for a full ~2,500-word article plus its
		// Conclusion; 4096 truncated long articles mid-section (the model never
		// reached the Conclusion, failing validation). max_tokens is an upper
		// bound — only tokens actually generated are billed.
		c.maxTokens = 8192
	}
	return c
}

// NewFromAWS loads the default AWS config (instance role in production) and
// builds a Bedrock-backed Client. No credentials are read from code.
func NewFromAWS(ctx context.Context, region string, cfg Config) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("bedrockclaude: load AWS config: %w", err)
	}
	return New(bedrockruntime.NewFromConfig(awsCfg), cfg), nil
}

// requestBody is the Anthropic Messages API payload for Bedrock.
type requestBody struct {
	AnthropicVersion string    `json:"anthropic_version"`
	MaxTokens        int       `json:"max_tokens"`
	Temperature      float64   `json:"temperature,omitempty"`
	System           string    `json:"system,omitempty"`
	Messages         []message `json:"messages"`
}

type message struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// responseBody is the subset of the Bedrock Claude response we consume.
type responseBody struct {
	Content    []contentPart `json:"content"`
	StopReason string        `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Generate sends a single-turn prompt to Claude and returns the text. It
// implements the generate port shared by every content generator.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	if c == nil || c.api == nil {
		return "", errors.New("bedrockclaude: client not initialised")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("bedrockclaude: empty prompt")
	}

	body, err := json.Marshal(requestBody{
		AnthropicVersion: anthropicVersion,
		MaxTokens:        c.maxTokens,
		Temperature:      c.temperature,
		System:           c.system,
		Messages: []message{{
			Role:    "user",
			Content: []contentPart{{Type: "text", Text: prompt}},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("bedrockclaude: marshal request: %w", err)
	}

	out, err := c.api.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(c.modelID),
		Body:        body,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("bedrockclaude: invoke %s: %w", c.modelID, err)
	}

	var resp responseBody
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return "", fmt.Errorf("bedrockclaude: decode response: %w", err)
	}
	text := joinText(resp.Content)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("bedrockclaude: empty completion (stop_reason=%q)", resp.StopReason)
	}
	return text, nil
}

func joinText(parts []contentPart) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}
