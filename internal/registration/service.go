// Package registration implements the MVP repository onboarding use case:
// validate repository access with a GitHub PAT, create the push webhook, store
// the PAT securely, and persist repository metadata. It depends only on small
// ports (interfaces) so it is fully unit-testable and free of AWS/GitHub SDKs.
package registration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
)

// GitHub is the port for the GitHub operations onboarding needs. The concrete
// *github.Client satisfies it structurally.
type GitHub interface {
	GetRepository(ctx context.Context, owner, name, pat string) (github.RepoInfo, error)
	CreateWebhook(ctx context.Context, owner, name, pat string, cfg github.WebhookConfig) (int64, error)
}

// SecretStore persists a repository's credentials (PAT + webhook secret) and
// returns an opaque reference (never the secret values).
type SecretStore interface {
	PutRepoCredentials(ctx context.Context, owner, name, pat, webhookSecret string) (secretRef string, err error)
}

// MetadataStore persists repository metadata.
type MetadataStore interface {
	Put(ctx context.Context, r repo.Repository) error
}

// Input is the registration request payload.
type Input struct {
	RepositoryURL string `json:"repository_url"`
	PAT           string `json:"pat"`
}

// Output is returned on successful registration (no secret material).
type Output struct {
	RepoFullName   string `json:"repo_full_name"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	DefaultBranch  string `json:"default_branch"`
	WebhookID      int64  `json:"webhook_id"`
	TriggerPattern string `json:"trigger_pattern"`
	Status         string `json:"status"`
}

// Service orchestrates onboarding. Collaborators are injected; the two funcs
// are overridable in tests for deterministic behaviour.
type Service struct {
	GitHub         GitHub
	Secrets        SecretStore
	Metadata       MetadataStore
	WebhookURL     string
	DefaultTrigger string
	Now            func() time.Time
	NewSecret      func() (string, error)
}

// Register runs the onboarding flow. Steps are ordered so that a failure never
// leaves a half-registered repository visible in the metadata store: the
// webhook and secret are created before metadata is written.
func (s *Service) Register(ctx context.Context, in Input) (Output, error) {
	if in.RepositoryURL == "" {
		return Output{}, apperror.New(apperror.CodeInvalidInput, "repository_url is required")
	}
	if in.PAT == "" {
		return Output{}, apperror.New(apperror.CodeInvalidInput, "pat is required")
	}

	owner, name, err := repo.ParseRepositoryURL(in.RepositoryURL)
	if err != nil {
		return Output{}, apperror.Wrap(err, apperror.CodeInvalidInput, "invalid repository_url")
	}

	// Validate access + token read permission.
	info, err := s.GitHub.GetRepository(ctx, owner, name, in.PAT)
	if err != nil {
		return Output{}, err
	}

	// Create the webhook (also validates webhook permission).
	webhookSecret, err := s.newSecret()
	if err != nil {
		return Output{}, apperror.Wrap(err, apperror.CodeInternal, "generate webhook secret")
	}
	hookID, err := s.GitHub.CreateWebhook(ctx, owner, name, in.PAT, github.WebhookConfig{
		URL:    s.WebhookURL,
		Secret: webhookSecret,
		Events: []string{"push"},
	})
	if err != nil {
		return Output{}, err
	}

	// Store credentials in Secrets Manager; keep only the reference.
	secretRef, err := s.Secrets.PutRepoCredentials(ctx, owner, name, in.PAT, webhookSecret)
	if err != nil {
		return Output{}, apperror.Wrap(err, apperror.CodeInternal, "store credentials")
	}

	trigger := s.DefaultTrigger
	if trigger == "" {
		trigger = "blog:"
	}
	r := repo.Repository{
		RepoFullName:   repo.FullName(owner, name),
		RepositoryID:   info.ID,
		Owner:          owner,
		Name:           name,
		URL:            in.RepositoryURL,
		DefaultBranch:  info.DefaultBranch,
		WebhookID:      hookID,
		TriggerPattern: trigger,
		Enabled:        true,
		SecretRef:      secretRef,
		RegisteredAt:   s.now().UTC().Format(time.RFC3339),
	}
	if err := s.Metadata.Put(ctx, r); err != nil {
		return Output{}, apperror.Wrap(err, apperror.CodeInternal, "store metadata")
	}

	return Output{
		RepoFullName:   r.RepoFullName,
		Owner:          owner,
		Name:           name,
		DefaultBranch:  info.DefaultBranch,
		WebhookID:      hookID,
		TriggerPattern: trigger,
		Status:         "registered",
	}, nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) newSecret() (string, error) {
	if s.NewSecret != nil {
		return s.NewSecret()
	}
	return randomHex(32)
}

// randomHex returns n cryptographically-random bytes hex-encoded.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
