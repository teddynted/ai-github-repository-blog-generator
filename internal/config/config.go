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
	"time"
)

// Default values applied when an optional variable is unset.
const (
	DefaultPublishTrigger      = "blog:"
	DefaultSecretsPrefix       = "blog-gen/repos"
	DefaultOutputDir           = "/data/generated-content"
	DefaultWorkDir             = "/data/work"
	DefaultMemoryDir           = "/data/memory"
	DefaultPendingDir          = "/data/pending"
	DefaultLogLevel            = "info"
	DefaultRequireHumanApprove = false
	// Turbo SMTP defaults for email notifications. Port 587 uses STARTTLS.
	DefaultSMTPHost = "pro.turbo-smtp.com"
	DefaultSMTPPort = 587
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
	// (SECRETS_PREFIX). Retained for compatibility; superseded by RepoSecretID.
	SecretsPrefix string
	// RepoSecretID is the name/ARN of the single shared secret holding every
	// repository's credentials, keyed by "<owner>/<name>" (REPO_SECRET_ID).
	RepoSecretID string
	// EventBusName is the EventBridge bus for matched events (EVENT_BUS_NAME).
	EventBusName string
	// StateMachineArn is the Step Functions orchestration state machine the
	// manual trigger starts on POST /process (STATE_MACHINE_ARN).
	StateMachineArn string
	// VideoStateMachineArn is the Step Functions video state machine the worker
	// starts after a successful release run to render social videos
	// (VIDEO_STATE_MACHINE_ARN). Blank disables video rendering.
	VideoStateMachineArn string
	// QueueURL is the SQS events queue URL (QUEUE_URL).
	QueueURL string
	// InstanceID is the On-Demand EC2 instance the scheduler powers on/off
	// (INSTANCE_ID).
	InstanceID string
	// N8NWebhookURL is the n8n workflow entry point invoked once the instance
	// is healthy (N8N_WEBHOOK_URL).
	N8NWebhookURL string
	// WebhookURL is this platform's public webhook endpoint, used by the
	// registration Lambda when creating the GitHub webhook (WEBHOOK_URL).
	WebhookURL string
	// BedrockModelID selects the Amazon Bedrock Claude model — the AI Provider
	// Router's primary provider in the cloud (BEDROCK_MODEL_ID, e.g. the
	// inference-profile id "us.anthropic.claude-opus-4-8"). Auth is IAM via the
	// instance role in AWSRegion — no API key. When Bedrock cannot fulfil a request
	// because of quota/throttling, the router falls back to the Anthropic API.
	BedrockModelID string
	// AnthropicAPIKey is an Anthropic API key (ANTHROPIC_API_KEY) — the router's
	// fallback provider in the cloud and the only provider locally. Prefer
	// AnthropicAPIKeySecret in production so the key never sits in plaintext env.
	AnthropicAPIKey string
	// AnthropicAPIKeySecret is a Secrets Manager id/ARN whose value is the
	// Anthropic API key (ANTHROPIC_API_KEY_SECRET). Resolved at startup; takes
	// precedence over AnthropicAPIKey so the credential never lives in the
	// instance's env file.
	AnthropicAPIKeySecret string
	// AnthropicModel is the Anthropic API model id for the writer (ANTHROPIC_MODEL,
	// e.g. "claude-opus-4-8"). Blank uses the client default.
	AnthropicModel string
	// OutputDir is where generated content is written locally (OUTPUT_DIR).
	OutputDir string
	// OutputS3Bucket, when set, publishes generated content to S3 instead of the
	// local filesystem (OUTPUT_S3_BUCKET).
	OutputS3Bucket string
	// OutputS3Prefix is the S3 key prefix (OUTPUT_S3_PREFIX).
	OutputS3Prefix string
	// WorkDir is the parent directory for repository clones (WORK_DIR).
	WorkDir string
	// MemoryDir is the base directory for Repository Memory (MEMORY_DIR).
	MemoryDir string
	// PendingDir is where content awaiting human approval is stashed (PENDING_DIR).
	PendingDir string
	// NotifyWebhookURL is an optional Slack/webhook URL for notifications
	// (NOTIFY_WEBHOOK_URL). When blank, notifications are logged only.
	NotifyWebhookURL string
	// NotifyEmailFrom / NotifyEmailTo enable email notifications when both are
	// set (NOTIFY_EMAIL_FROM, NOTIFY_EMAIL_TO — comma-separated recipients).
	// Delivery is over SMTP (Turbo SMTP), configured by the SMTP* fields below.
	NotifyEmailFrom string
	NotifyEmailTo   string
	// SMTP* configure the SMTP relay used for email notifications (Turbo SMTP by
	// default). SMTPUsername + SMTPPassword are required to actually send; the
	// password is a credential (SMTP_PASSWORD) — keep it out of source control.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	// SMTPPasswordSecret is an optional AWS Secrets Manager id/ARN whose value is
	// the SMTP password (SMTP_PASSWORD_SECRET). When set, it is resolved at
	// startup and takes precedence over SMTP_PASSWORD, so the credential never
	// lives in plaintext env/config on the instance.
	SMTPPasswordSecret string
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
		AWSRegion:             getenv("AWS_REGION"),
		ProjectName:           getenv("PROJECT_NAME"),
		EventSource:           getenv("EVENT_SOURCE"),
		PublishTrigger:        firstNonEmpty(getenv("PUBLISH_TRIGGER"), DefaultPublishTrigger),
		RepositoriesTable:     getenv("REPOSITORIES_TABLE"),
		SecretsPrefix:         firstNonEmpty(getenv("SECRETS_PREFIX"), DefaultSecretsPrefix),
		RepoSecretID:          getenv("REPO_SECRET_ID"),
		EventBusName:          getenv("EVENT_BUS_NAME"),
		StateMachineArn:       getenv("STATE_MACHINE_ARN"),
		VideoStateMachineArn:  getenv("VIDEO_STATE_MACHINE_ARN"),
		QueueURL:              getenv("QUEUE_URL"),
		InstanceID:            getenv("INSTANCE_ID"),
		N8NWebhookURL:         getenv("N8N_WEBHOOK_URL"),
		WebhookURL:            getenv("WEBHOOK_URL"),
		BedrockModelID:        getenv("BEDROCK_MODEL_ID"),
		AnthropicAPIKey:       getenv("ANTHROPIC_API_KEY"),
		AnthropicAPIKeySecret: getenv("ANTHROPIC_API_KEY_SECRET"),
		AnthropicModel:        getenv("ANTHROPIC_MODEL"),
		OutputDir:             firstNonEmpty(getenv("OUTPUT_DIR"), DefaultOutputDir),
		OutputS3Bucket:        getenv("OUTPUT_S3_BUCKET"),
		OutputS3Prefix:        firstNonEmpty(getenv("OUTPUT_S3_PREFIX"), "generated-content"),
		WorkDir:               firstNonEmpty(getenv("WORK_DIR"), DefaultWorkDir),
		MemoryDir:             firstNonEmpty(getenv("MEMORY_DIR"), DefaultMemoryDir),
		PendingDir:            firstNonEmpty(getenv("PENDING_DIR"), DefaultPendingDir),
		NotifyWebhookURL:      getenv("NOTIFY_WEBHOOK_URL"),
		NotifyEmailFrom:       getenv("NOTIFY_EMAIL_FROM"),
		NotifyEmailTo:         getenv("NOTIFY_EMAIL_TO"),
		SMTPHost:              firstNonEmpty(getenv("SMTP_HOST"), DefaultSMTPHost),
		SMTPPort:              DefaultSMTPPort,
		SMTPUsername:          getenv("SMTP_USERNAME"),
		SMTPPassword:          getenv("SMTP_PASSWORD"),
		SMTPPasswordSecret:    getenv("SMTP_PASSWORD_SECRET"),
		LogLevel:              firstNonEmpty(getenv("LOG_LEVEL"), DefaultLogLevel),
		RequireHumanApproval:  DefaultRequireHumanApprove,
	}

	if raw := getenv("SMTP_PORT"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("config: SMTP_PORT %q is not an integer: %w", raw, err)
		}
		if v <= 0 || v > 65535 {
			return Config{}, fmt.Errorf("config: SMTP_PORT must be 1-65535, got %d", v)
		}
		cfg.SMTPPort = v
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
	case "RepoSecretID":
		return c.RepoSecretID, true
	case "EventBusName":
		return c.EventBusName, true
	case "StateMachineArn":
		return c.StateMachineArn, true
	case "VideoStateMachineArn":
		return c.VideoStateMachineArn, true
	case "QueueURL":
		return c.QueueURL, true
	case "InstanceID":
		return c.InstanceID, true
	case "N8NWebhookURL":
		return c.N8NWebhookURL, true
	case "WebhookURL":
		return c.WebhookURL, true
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

// durationOrDefault parses a Go duration string (e.g. "20m", "1h30m") and
// returns fallback when it is empty, unparseable, or non-positive.
func durationOrDefault(v string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil && d > 0 {
		return d
	}
	return fallback
}
