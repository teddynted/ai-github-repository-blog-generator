// Package secrets adapts AWS Secrets Manager to the platform's SecretStore
// port. Each repository's GitHub PAT and webhook secret are stored under a
// stable prefix; only the base reference is ever returned to callers.
package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

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

// PutRepoCredentials stores the PAT and webhook secret for a repository and
// returns the base reference ("<prefix>/<owner>/<name>"). The individual
// secrets live at "<ref>/pat" and "<ref>/webhook-secret".
func (s *Store) PutRepoCredentials(ctx context.Context, owner, name, pat, webhookSecret string) (string, error) {
	base := fmt.Sprintf("%s/%s/%s", s.prefix, owner, name)
	if err := s.upsert(ctx, base+"/pat", pat); err != nil {
		return "", fmt.Errorf("store pat: %w", err)
	}
	if err := s.upsert(ctx, base+"/webhook-secret", webhookSecret); err != nil {
		return "", fmt.Errorf("store webhook secret: %w", err)
	}
	return base, nil
}

// WebhookSecret retrieves a repository's webhook signing secret, given the
// base reference returned by PutRepoCredentials.
func (s *Store) WebhookSecret(ctx context.Context, ref string) (string, error) {
	name := ref + "/webhook-secret"
	out, err := s.api.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	})
	if err != nil {
		return "", fmt.Errorf("get webhook secret: %w", err)
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
