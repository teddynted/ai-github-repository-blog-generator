// Package config loads and validates runtime configuration from the
// environment. Every Lambda in the platform shares this loader so that
// configuration is sourced and validated consistently.
//
// Configuration is intentionally environment-driven (12-factor): the
// CloudFormation templates inject these values into each Lambda, and local
// development supplies them via a .env file (see .env.example).
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Default values applied when an optional variable is unset.
const (
	DefaultPublishTrigger      = "blog:"
	DefaultSecretsPrefix       = "blog-gen/repos"
	DefaultOllamaModel         = "qwen2.5:7b"
	DefaultOllamaBaseURL       = "http://localhost:11434"
	DefaultOutputDir           = "/data/generated-content"
	DefaultIdleTimeoutMinutes  = 15
	DefaultLogLevel            = "info"
	DefaultRequireHumanApprove = false
)

// Config holds the runtime configuration shared across the platform's
// Lambdas. Not every field is required by every function; callers validate
// the subset they depend on via Require.
type Config struct {
	// AWSRegion is the deployment region (AWS_REGION).
	AWSRegion string
	// ProjectName is the resource prefix / tag value used to locate the EC2
	// instance and name resources (PROJECT_NAME).
	ProjectName string
	// EventSource is the EventBridge "source" the webhook handler publishes
	// under; the matched-event rule keys off it (EVENT_SOURCE).
	EventSource string
	// PublishTrigger is the default commit-message trigger prefix
	// (PUBLISH_TRIGGER). A repository may override it via its stored
	// trigger pattern; this is the platform default.
	PublishTrigger string
	// RepositoriesTable is the DynamoDB metadata table (REPOSITORIES_TABLE).
	RepositoriesTable string
	// SecretsPrefix is the Secrets Manager path prefix for per-repo secrets
	// (SECRETS_PREFIX).
	SecretsPrefix string
	// EventBusName is the EventBridge bus for matched events (EVENT_BUS_NAME).
	EventBusName string
	// QueueURL is the SQS events queue URL (QUEUE_URL).
	QueueURL string
	// InstanceID is the EC2 Spot instance managed by the platform (INSTANCE_ID).
	InstanceID string
	// N8NWebhookURL is the n8n workflow entry point invoked once the instance
	// is healthy (N8N_WEBHOOK_URL).
	N8NWebhookURL string
	// WebhookURL is this platform's public webhook endpoint, used by the
	// registration Lambda when creating the GitHub webhook (WEBHOOK_URL).
	WebhookURL string
	// IdleTimeoutMinutes is the inactivity window before auto-shutdown
	// (IDLE_TIMEOUT_MINUTES).
	IdleTimeoutMinutes int
	// OllamaModel is the local model served by Ollama (OLLAMA_MODEL).
	OllamaModel string
	// OllamaBaseURL is the local Ollama endpoint (OLLAMA_BASE_URL).
	OllamaBaseURL string
	// OutputDir is where generated content is written (OUTPUT_DIR).
	OutputDir string
	// RequireHumanApproval gates publishing behind a manual approval
	// (REQUIRE_HUMAN_APPROVAL).
	RequireHumanApproval bool
	// LogLevel is the structured-logging level (LOG_LEVEL): debug|info|warn|error.
	LogLevel string
}

// Getenv is the environment lookup abstraction. Injecting it keeps the
// loader unit-testable without touching process state.
type Getenv func(key string) string

// Load reads configuration from the given lookup, applying documented
// defaults for optional values. It returns an error only when a supplied
// value is malformed (e.g. a non-numeric duration); presence of required
// values is enforced separately via Require, because each Lambda needs a
// different subset.
func Load(getenv Getenv) (Config, error) {
	cfg := Config{
		AWSRegion:            getenv("AWS_REGION"),
		ProjectName:          getenv("PROJECT_NAME"),
		EventSource:          getenv("EVENT_SOURCE"),
		PublishTrigger:       firstNonEmpty(getenv("PUBLISH_TRIGGER"), DefaultPublishTrigger),
		RepositoriesTable:    getenv("REPOSITORIES_TABLE"),
		SecretsPrefix:        firstNonEmpty(getenv("SECRETS_PREFIX"), DefaultSecretsPrefix),
		EventBusName:         getenv("EVENT_BUS_NAME"),
		QueueURL:             getenv("QUEUE_URL"),
		InstanceID:           getenv("INSTANCE_ID"),
		N8NWebhookURL:        getenv("N8N_WEBHOOK_URL"),
		WebhookURL:           getenv("WEBHOOK_URL"),
		OllamaModel:          firstNonEmpty(getenv("OLLAMA_MODEL"), DefaultOllamaModel),
		OllamaBaseURL:        firstNonEmpty(getenv("OLLAMA_BASE_URL"), DefaultOllamaBaseURL),
		OutputDir:            firstNonEmpty(getenv("OUTPUT_DIR"), DefaultOutputDir),
		LogLevel:             firstNonEmpty(getenv("LOG_LEVEL"), DefaultLogLevel),
		IdleTimeoutMinutes:   DefaultIdleTimeoutMinutes,
		RequireHumanApproval: DefaultRequireHumanApprove,
	}

	if raw := getenv("IDLE_TIMEOUT_MINUTES"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("config: IDLE_TIMEOUT_MINUTES %q is not an integer: %w", raw, err)
		}
		if v <= 0 {
			return Config{}, fmt.Errorf("config: IDLE_TIMEOUT_MINUTES must be positive, got %d", v)
		}
		cfg.IdleTimeoutMinutes = v
	}

	if raw := getenv("REQUIRE_HUMAN_APPROVAL"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("config: REQUIRE_HUMAN_APPROVAL %q is not a boolean: %w", raw, err)
		}
		cfg.RequireHumanApproval = v
	}

	return cfg, nil
}

// Require validates that the named fields are non-empty, returning a single
// aggregated error listing every missing field. Field names use the struct
// field identifiers (e.g. "RepositoriesTable"); unknown names are reported so
// typos surface in tests rather than silently passing.
func (c Config) Require(fields ...string) error {
	var missing []string
	for _, f := range fields {
		v, ok := c.field(f)
		if !ok {
			return fmt.Errorf("config: unknown required field %q", f)
		}
		if strings.TrimSpace(v) == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("config: missing required values: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c Config) field(name string) (string, bool) {
	switch name {
	case "AWSRegion":
		return c.AWSRegion, true
	case "ProjectName":
		return c.ProjectName, true
	case "EventSource":
		return c.EventSource, true
	case "PublishTrigger":
		return c.PublishTrigger, true
	case "RepositoriesTable":
		return c.RepositoriesTable, true
	case "SecretsPrefix":
		return c.SecretsPrefix, true
	case "EventBusName":
		return c.EventBusName, true
	case "QueueURL":
		return c.QueueURL, true
	case "InstanceID":
		return c.InstanceID, true
	case "N8NWebhookURL":
		return c.N8NWebhookURL, true
	case "WebhookURL":
		return c.WebhookURL, true
	case "OllamaModel":
		return c.OllamaModel, true
	case "LogLevel":
		return c.LogLevel, true
	default:
		return "", false
	}
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
