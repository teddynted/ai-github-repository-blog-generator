// Package imagegen renders a text prompt into a raster image using Amazon Nova
// Canvas on Amazon Bedrock. It exists so the video renderer can give each scene a
// generated background instead of a plain colour card, and it is deliberately
// small: one Generate(ctx, Spec) -> PNG bytes call, IAM-authenticated through the
// default AWS chain (the task role in production), with bounded retry/backoff for
// Nova Canvas's frequent throttling and no fallback of its own — callers decide
// what to show when generation is unavailable.
package imagegen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// DefaultModelID is the Bedrock Nova Canvas text-to-image model.
const DefaultModelID = "amazon.nova-canvas-v1:0"

// maxRetries bounds the backoff loop; Nova Canvas throttles ("Too many
// connections"/"Too many requests") aggressively, so a few spaced retries turn a
// transient failure into a success without stalling a render for long.
const maxRetries = 4

// invoker is the minimal Bedrock Runtime surface the client needs, so the client
// is unit-testable with a fake (the real *bedrockruntime.Client satisfies it).
type invoker interface {
	InvokeModel(ctx context.Context, in *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

// Client generates images with Nova Canvas on Bedrock.
type Client struct {
	api     invoker
	modelID string
	// sleep is the backoff delay function; overridable in tests so retries don't
	// wall-clock. Defaults to time.Sleep.
	sleep func(time.Duration)
}

// New builds a client from the default AWS config chain (task role in
// production, profile/env in dev).
func New(ctx context.Context) (*Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("imagegen: aws config: %w", err)
	}
	return &Client{api: bedrockruntime.NewFromConfig(cfg), modelID: DefaultModelID, sleep: time.Sleep}, nil
}

// Spec is one image request. Width and Height must satisfy Nova Canvas's
// constraints (each a multiple of 16, 320–4096, total <= 4,194,304 px); callers
// should pass a supported size and scale the result to the final frame.
type Spec struct {
	Prompt         string
	NegativePrompt string
	Width          int
	Height         int
	Seed           int
}

// novaRequest is the Nova Canvas TEXT_IMAGE request body.
type novaRequest struct {
	TaskType          string         `json:"taskType"`
	TextToImageParams textToImage    `json:"textToImageParams"`
	ImageGenConfig    imageGenConfig `json:"imageGenerationConfig"`
}

type textToImage struct {
	Text         string `json:"text"`
	NegativeText string `json:"negativeText,omitempty"`
}

type imageGenConfig struct {
	NumberOfImages int     `json:"numberOfImages"`
	Height         int     `json:"height"`
	Width          int     `json:"width"`
	CfgScale       float64 `json:"cfgScale"`
	Seed           int     `json:"seed"`
	Quality        string  `json:"quality"`
}

// novaResponse is the Nova Canvas response: base64-encoded PNGs, or an error
// list when the model rejects the request.
type novaResponse struct {
	Images []string `json:"images"`
	Error  string   `json:"error,omitempty"`
}

// Generate renders spec into PNG bytes. It retries on Bedrock throttling with
// linear backoff; any other error (or an exhausted retry budget) is returned so
// the caller can fall back. Nova Canvas's negativeText rejects an empty string,
// so a blank NegativePrompt is omitted.
func (c *Client) Generate(ctx context.Context, spec Spec) ([]byte, error) {
	if strings.TrimSpace(spec.Prompt) == "" {
		return nil, errors.New("imagegen: empty prompt")
	}
	body, err := json.Marshal(novaRequest{
		TaskType: "TEXT_IMAGE",
		TextToImageParams: textToImage{
			Text:         truncate(spec.Prompt, 1024),
			NegativeText: truncate(strings.TrimSpace(spec.NegativePrompt), 1024),
		},
		ImageGenConfig: imageGenConfig{
			NumberOfImages: 1,
			Height:         spec.Height,
			Width:          spec.Width,
			CfgScale:       8.0,
			Seed:           spec.Seed,
			Quality:        "standard",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("imagegen: marshal: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Linear backoff (1s, 2s, 3s, …) — Nova Canvas throttling clears quickly.
			c.sleep(time.Duration(attempt) * time.Second)
		}
		out, err := c.api.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(c.modelID),
			ContentType: aws.String("application/json"),
			Accept:      aws.String("application/json"),
			Body:        body,
		})
		if err != nil {
			lastErr = err
			if isThrottle(err) {
				continue
			}
			return nil, fmt.Errorf("imagegen: invoke: %w", err)
		}
		var resp novaResponse
		if err := json.Unmarshal(out.Body, &resp); err != nil {
			return nil, fmt.Errorf("imagegen: decode: %w", err)
		}
		if resp.Error != "" {
			return nil, fmt.Errorf("imagegen: model error: %s", resp.Error)
		}
		if len(resp.Images) == 0 {
			return nil, errors.New("imagegen: no image returned")
		}
		png, err := base64.StdEncoding.DecodeString(resp.Images[0])
		if err != nil {
			return nil, fmt.Errorf("imagegen: decode png: %w", err)
		}
		return png, nil
	}
	return nil, fmt.Errorf("imagegen: throttled after %d attempts: %w", maxRetries, lastErr)
}

// isThrottle reports whether err is a Bedrock capacity/rate error worth
// retrying: the modeled Throttling/ServiceUnavailable exceptions, or their
// message text when only a generic error surfaces.
func isThrottle(err error) bool {
	var thr *types.ThrottlingException
	var svc *types.ServiceUnavailableException
	if errors.As(err, &thr) || errors.As(err, &svc) {
		return true
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "too many") || strings.Contains(m, "throttl") || strings.Contains(m, "serviceunavailable")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
