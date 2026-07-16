// Package secrets adapts AWS Secrets Manager to the platform's SecretStore
// port. Each repository's GitHub PAT and webhook secret are stored together in
// a single JSON secret at "<prefix>/<owner>/<name>"; only that reference is
// ever returned to callers. One secret per repo (rather than one per field)
// halves the Secrets Manager entry count and cost.
package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// repoCredentials is the JSON shape stored in a repository's secret.
type repoCredentials struct {
	PAT           string `json:"pat"`
	WebhookSecret string `json:"webhook_secret"`
}

// API is the subset of the Secrets Manager client this adapter uses. The real
// *secretsmanager.Client satisfies it; tests supply a fake.
type API interface {
	CreateSecret(ctx context.Context, in *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	PutSecretValue(ctx context.Context, in *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// Store persists per-repository credentials in Secrets Manager.
type Store struct {
	api    API
	prefix string
}

// New builds a Store. prefix is the Secrets Manager path root (e.g.
// "blog-gen/repos").
func New(api API, prefix string) *Store {
	return &Store{api: api, prefix: prefix}
}

// PutRepoCredentials stores the PAT and webhook secret for a repository as one
// JSON secret and returns its reference ("<prefix>/<owner>/<name>").
func (s *Store) PutRepoCredentials(ctx context.Context, owner, name, pat, webhookSecret string) (string, error) {
	ref := fmt.Sprintf("%s/%s/%s", s.prefix, owner, name)
	value, err := json.Marshal(repoCredentials{PAT: pat, WebhookSecret: webhookSecret})
	if err != nil {
		return "", fmt.Errorf("marshal credentials: %w", err)
	}
	if err := s.upsert(ctx, ref, string(value)); err != nil {
		return "", fmt.Errorf("store credentials: %w", err)
	}
	return ref, nil
}

// WebhookSecret retrieves a repository's webhook signing secret, given the
// reference returned by PutRepoCredentials.
func (s *Store) WebhookSecret(ctx context.Context, ref string) (string, error) {
	c, err := s.repoCredentials(ctx, ref)
	if err != nil {
		return "", err
	}
	return c.WebhookSecret, nil
}

// PAT retrieves a repository's GitHub Personal Access Token, given the
// reference. Callers must treat the result as secret and never log it.
func (s *Store) PAT(ctx context.Context, ref string) (string, error) {
	c, err := s.repoCredentials(ctx, ref)
	if err != nil {
		return "", err
	}
	return c.PAT, nil
}

// repoCredentials fetches and parses the repository's JSON credential secret.
func (s *Store) repoCredentials(ctx context.Context, ref string) (repoCredentials, error) {
	raw, err := s.get(ctx, ref, "credentials")
	if err != nil {
		return repoCredentials{}, err
	}
	var c repoCredentials
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return repoCredentials{}, fmt.Errorf("parse credentials: %w", err)
	}
	return c, nil
}

// Value resolves an arbitrary secret by its id or ARN (not prefix-scoped). Used
// for shared secrets such as the SMTP password. Treat the result as sensitive.
func (s *Store) Value(ctx context.Context, secretID string) (string, error) {
	return s.get(ctx, secretID, "value")
}

func (s *Store) get(ctx context.Context, name, what string) (string, error) {
	out, err := s.api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	})
	if err != nil {
		return "", fmt.Errorf("get %s: %w", what, err)
	}
	return aws.ToString(out.SecretString), nil
}

// upsert creates the secret, falling back to a new version if it already
// exists (supports re-registration / token rotation).
func (s *Store) upsert(ctx context.Context, name, value string) error {
	_, err := s.api.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretString: aws.String(value),
	})
	if err == nil {
		return nil
	}
	var exists *smtypes.ResourceExistsException
	if errors.As(err, &exists) {
		_, perr := s.api.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
			SecretId:     aws.String(name),
			SecretString: aws.String(value),
		})
		return perr
	}
	return err
}
