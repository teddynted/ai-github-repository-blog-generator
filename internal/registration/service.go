// Package registration implements repository onboarding: validate repository
// access with a GitHub PAT, create (or update) the push/release webhook, store
// the credentials in the shared repositories secret, and persist repository
// metadata. It depends only on small ports (interfaces) so it is fully
// unit-testable and free of AWS/GitHub SDKs.
package registration

import (
	"context"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/github"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/trigger"
)

// GitHub is the port for the GitHub operations onboarding needs.
type GitHub interface {
	GetRepository(ctx context.Context, owner, name, pat string) (github.RepoInfo, error)
	CreateWebhook(ctx context.Context, owner, name, pat string, cfg github.WebhookConfig) (int64, error)
	UpdateWebhook(ctx context.Context, owner, name, pat string, hookID int64, cfg github.WebhookConfig) error
	DeleteWebhook(ctx context.Context, owner, name, pat string, hookID int64) error
}

// CredentialStore persists repository credentials in the shared secret, keyed
// by "<owner>/<name>". PutRepoCredentials reports whether the key already
// existed and rejects a duplicate when allowUpdate is false.
type CredentialStore interface {
	PutRepoCredentials(ctx context.Context, key, pat, webhookSecret string, allowUpdate bool) (existed bool, err error)
	DeleteRepoCredentials(ctx context.Context, key string) (existed bool, err error)
	PAT(ctx context.Context, key string) (string, error)
}

// MetadataStore persists repository metadata.
type MetadataStore interface {
	Put(ctx context.Context, r repo.Repository) error
	Get(ctx context.Context, fullName string) (repo.Repository, bool, error)
	Delete(ctx context.Context, fullName string) error
}

// Input is the /repositories registration payload.
type Input struct {
	Owner         string `json:"owner"`
	Repository    string `json:"repository"`
	PAT           string `json:"pat"`
	WebhookSecret string `json:"webhook_secret"`
	// TriggerPattern optionally overrides the default publishing trigger.
	TriggerPattern string `json:"trigger_pattern,omitempty"`
	// Update must be true to overwrite an already-registered repository;
	// otherwise a duplicate is rejected.
	Update bool `json:"update,omitempty"`
}

// DeleteInput identifies a repository to deregister.
type DeleteInput struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
}

// Outcome reports what a Register call did.
type Outcome struct {
	RepoFullName string
	Updated      bool // false = newly registered, true = credentials updated
}

// Service orchestrates onboarding against the shared repositories secret.
type Service struct {
	GitHub         GitHub
	Secrets        CredentialStore
	Metadata       MetadataStore
	WebhookURL     string
	DefaultTrigger string
	Now            func() time.Time
}

// Register onboards a repository or updates an existing one. It validates
// access, creates/updates the webhook with the supplied secret, upserts the
// credentials into the shared secret, and writes metadata. A duplicate (already
// registered, Update not set) is rejected.
func (s *Service) Register(ctx context.Context, in Input) (Outcome, error) {
	if err := validate(in); err != nil {
		return Outcome{}, err
	}
	owner, name := in.Owner, in.Repository
	key := repo.FullName(owner, name)

	existing, alreadyRegistered, err := s.Metadata.Get(ctx, key)
	if err != nil {
		return Outcome{}, apperror.Wrap(err, apperror.CodeInternal, "metadata lookup failed")
	}
	if alreadyRegistered && !in.Update {
		return Outcome{}, apperror.New(apperror.CodeConflict, "Repository is already registered.")
	}

	// Validate access + token read permission.
	info, err := s.GitHub.GetRepository(ctx, owner, name, in.PAT)
	if err != nil {
		return Outcome{}, err
	}

	// GitHub webhook ingress has been removed from the platform. Only manage a
	// GitHub webhook when a delivery URL is configured; otherwise onboarding is
	// purely credential + metadata registration for the manual /process trigger.
	hookID := existing.WebhookID
	if s.WebhookURL != "" {
		cfg := github.WebhookConfig{URL: s.WebhookURL, Secret: in.WebhookSecret, Events: []string{"push", "release"}}
		if alreadyRegistered {
			// Keep GitHub signing with the (possibly rotated) secret.
			if err := s.GitHub.UpdateWebhook(ctx, owner, name, in.PAT, hookID, cfg); err != nil {
				return Outcome{}, err
			}
		} else {
			if hookID, err = s.GitHub.CreateWebhook(ctx, owner, name, in.PAT, cfg); err != nil {
				return Outcome{}, err
			}
		}
	}

	// Upsert into the shared secret (idempotent; concurrency handled by the store).
	if _, err := s.Secrets.PutRepoCredentials(ctx, key, in.PAT, in.WebhookSecret, in.Update); err != nil {
		return Outcome{}, err
	}

	triggerPattern := firstNonEmpty(in.TriggerPattern, existing.TriggerPattern, s.DefaultTrigger, trigger.DefaultPattern)
	r := repo.Repository{
		RepoFullName:   key,
		RepositoryID:   info.ID,
		Owner:          owner,
		Name:           name,
		URL:            "https://github.com/" + key,
		DefaultBranch:  info.DefaultBranch,
		WebhookID:      hookID,
		TriggerPattern: triggerPattern,
		Enabled:        true,
		SecretRef:      key, // the shared-secret key; readers look up by it
		RegisteredAt:   s.now().UTC().Format(time.RFC3339),
	}
	if err := s.Metadata.Put(ctx, r); err != nil {
		return Outcome{}, apperror.Wrap(err, apperror.CodeInternal, "store metadata")
	}
	return Outcome{RepoFullName: key, Updated: alreadyRegistered}, nil
}

// Delete deregisters a repository: remove the GitHub webhook, its entry in the
// shared secret, and its metadata. Missing pieces are tolerated so deletion is
// idempotent, but an unregistered repository is reported as not found.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (string, error) {
	if in.Owner == "" || in.Repository == "" {
		return "", apperror.New(apperror.CodeInvalidInput, "owner and repository are required")
	}
	key := repo.FullName(in.Owner, in.Repository)

	r, ok, err := s.Metadata.Get(ctx, key)
	if err != nil {
		return "", apperror.Wrap(err, apperror.CodeInternal, "metadata lookup failed")
	}
	if !ok {
		return "", apperror.New(apperror.CodeNotFound, "repository is not registered")
	}

	// Best-effort webhook removal (needs the PAT from the shared secret).
	if r.WebhookID != 0 {
		if pat, perr := s.Secrets.PAT(ctx, key); perr == nil {
			_ = s.GitHub.DeleteWebhook(ctx, in.Owner, in.Repository, pat, r.WebhookID)
		}
	}
	if _, err := s.Secrets.DeleteRepoCredentials(ctx, key); err != nil {
		return "", err
	}
	if err := s.Metadata.Delete(ctx, key); err != nil {
		return "", apperror.Wrap(err, apperror.CodeInternal, "delete metadata")
	}
	return key, nil
}

// validate enforces the required fields and an optional trigger pattern.
func validate(in Input) error {
	var missing []string
	if in.Owner == "" {
		missing = append(missing, "owner")
	}
	if in.Repository == "" {
		missing = append(missing, "repository")
	}
	if in.PAT == "" {
		missing = append(missing, "pat")
	}
	if in.WebhookSecret == "" {
		missing = append(missing, "webhook_secret")
	}
	if len(missing) > 0 {
		return apperror.New(apperror.CodeInvalidInput, "missing required field(s): "+joinComma(missing))
	}
	if in.TriggerPattern != "" {
		if err := trigger.Validate(in.TriggerPattern); err != nil {
			return apperror.Wrap(err, apperror.CodeInvalidInput, "invalid trigger_pattern")
		}
	}
	return nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func joinComma(v []string) string {
	out := ""
	for i, s := range v {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
