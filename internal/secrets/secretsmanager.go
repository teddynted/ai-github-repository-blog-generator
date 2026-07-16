// Package secrets adapts AWS Secrets Manager to the platform's credential
// storage. All registered repositories' credentials live in ONE shared secret
// whose value is a JSON object keyed by "<owner>/<name>":
//
//	{ "octocat/widget": {"pat": "…", "webhook_secret": "…"}, … }
//
// Registration writes entries; the webhook handler and worker read them by key.
// Writes use read-modify-write with retry; the registration Lambda additionally
// runs with reserved concurrency 1, so there is a single writer and updates are
// not lost.
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// Credentials is a single repository's stored credentials.
type Credentials struct {
	PAT           string `json:"pat"`
	WebhookSecret string `json:"webhook_secret"`
}

// API is the subset of the Secrets Manager client this adapter uses. The real
// *secretsmanager.Client satisfies it; tests supply a fake.
type API interface {
	GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	PutSecretValue(ctx context.Context, in *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
}

// Store reads and writes the shared repository-credentials secret.
type Store struct {
	api      API
	secretID string // name or ARN of the shared secret
	maxRetry int
	backoff  time.Duration
}

// New builds a Store for the given shared secret (name or ARN).
func New(api API, secretID string) *Store {
	return &Store{api: api, secretID: secretID, maxRetry: 5, backoff: 100 * time.Millisecond}
}

// WebhookSecret returns a repository's webhook signing secret by key
// ("<owner>/<name>"). A missing repository is a not-found error.
func (s *Store) WebhookSecret(ctx context.Context, key string) (string, error) {
	c, ok, err := s.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", apperror.New(apperror.CodeNotFound, "repository credentials not found")
	}
	return c.WebhookSecret, nil
}

// PAT returns a repository's GitHub Personal Access Token by key. Callers must
// treat the result as secret and never log it.
func (s *Store) PAT(ctx context.Context, key string) (string, error) {
	c, ok, err := s.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", apperror.New(apperror.CodeNotFound, "repository credentials not found")
	}
	return c.PAT, nil
}

// Get returns a repository's credentials and whether the key was present.
func (s *Store) Get(ctx context.Context, key string) (Credentials, bool, error) {
	m, err := s.read(ctx)
	if err != nil {
		return Credentials{}, false, err
	}
	c, ok := m[key]
	return c, ok, nil
}

// PutRepoCredentials inserts or updates a repository's entry in the shared
// secret and reports whether it already existed. When allowUpdate is false and
// the key already exists it returns a conflict error (duplicate registration)
// and writes nothing. The read-modify-write is retried on transient failures.
func (s *Store) PutRepoCredentials(ctx context.Context, key, pat, webhookSecret string, allowUpdate bool) (existed bool, err error) {
	return s.mutate(ctx, func(m map[string]Credentials) (bool, error) {
		_, existed := m[key]
		if existed && !allowUpdate {
			return false, apperror.New(apperror.CodeConflict, "repository is already registered")
		}
		m[key] = Credentials{PAT: pat, WebhookSecret: webhookSecret}
		return existed, nil
	})
}

// DeleteRepoCredentials removes a repository's entry and reports whether it was
// present. Removing a missing key is a no-op success.
func (s *Store) DeleteRepoCredentials(ctx context.Context, key string) (existed bool, err error) {
	return s.mutate(ctx, func(m map[string]Credentials) (bool, error) {
		if _, ok := m[key]; !ok {
			return false, errSkipWrite
		}
		delete(m, key)
		return true, nil
	})
}

// Value resolves an arbitrary secret by id/ARN (e.g. the SMTP password). Treat
// the result as sensitive.
func (s *Store) Value(ctx context.Context, secretID string) (string, error) {
	out, err := s.api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(secretID)})
	if err != nil {
		return "", fmt.Errorf("get secret %s: %w", secretID, err)
	}
	return aws.ToString(out.SecretString), nil
}

// errSkipWrite lets a mutate callback report "nothing to change" so no write is
// issued (e.g. deleting an absent key), returning success.
var errSkipWrite = fmt.Errorf("no change")

// mutate applies fn to the current credential map and persists the result,
// retrying the whole read-modify-write on transient errors. fn returns a value
// surfaced to the caller (e.g. whether the key existed) and an error; a fn
// error other than errSkipWrite aborts without writing (e.g. a duplicate).
func (s *Store) mutate(ctx context.Context, fn func(map[string]Credentials) (bool, error)) (bool, error) {
	var lastErr error
	for attempt := 0; attempt < s.maxRetry; attempt++ {
		if attempt > 0 {
			time.Sleep(s.backoff * time.Duration(attempt))
		}
		m, err := s.read(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		result, ferr := fn(m)
		if ferr == errSkipWrite {
			return result, nil
		}
		if ferr != nil {
			return result, ferr // e.g. conflict — do not retry
		}
		if err := s.write(ctx, m); err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}
	return false, fmt.Errorf("update shared secret after %d attempts: %w", s.maxRetry, lastErr)
}

// read fetches and decodes the shared secret's JSON map. An empty/blank value
// (freshly provisioned secret) decodes to an empty map.
func (s *Store) read(ctx context.Context) (map[string]Credentials, error) {
	out, err := s.api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(s.secretID)})
	if err != nil {
		return nil, fmt.Errorf("get shared secret: %w", err)
	}
	m := map[string]Credentials{}
	raw := strings.TrimSpace(aws.ToString(out.SecretString))
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			return nil, fmt.Errorf("parse shared secret: %w", err)
		}
	}
	return m, nil
}

func (s *Store) write(ctx context.Context, m map[string]Credentials) error {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal shared secret: %w", err)
	}
	if _, err := s.api.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(s.secretID),
		SecretString: aws.String(string(b)),
	}); err != nil {
		return fmt.Errorf("put shared secret: %w", err)
	}
	return nil
}
