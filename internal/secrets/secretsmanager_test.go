package secrets

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type fakeAPI struct {
	created     map[string]string
	putValues   map[string]string
	existing    map[string]bool
	createCalls int
}

func newFake() *fakeAPI {
	return &fakeAPI{created: map[string]string{}, putValues: map[string]string{}, existing: map[string]bool{}}
}

func (f *fakeAPI) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	name := *in.SecretId
	if v, ok := f.created[name]; ok {
		return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(v)}, nil
	}
	return nil, &smtypes.ResourceNotFoundException{}
}

func (f *fakeAPI) CreateSecret(_ context.Context, in *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	f.createCalls++
	name := *in.Name
	if f.existing[name] {
		return nil, &smtypes.ResourceExistsException{}
	}
	f.created[name] = *in.SecretString
	return &secretsmanager.CreateSecretOutput{}, nil
}

func (f *fakeAPI) PutSecretValue(_ context.Context, in *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	f.putValues[*in.SecretId] = *in.SecretString
	return &secretsmanager.PutSecretValueOutput{}, nil
}

func TestPutRepoCredentialsCreates(t *testing.T) {
	f := newFake()
	s := New(f, "blog-gen/repos")

	ref, err := s.PutRepoCredentials(context.Background(), "acme", "widget", "github_pat_x", "whsec")
	if err != nil {
		t.Fatalf("PutRepoCredentials: %v", err)
	}
	if ref != "blog-gen/repos/acme/widget" {
		t.Errorf("ref = %q", ref)
	}
	if f.created["blog-gen/repos/acme/widget/pat"] != "github_pat_x" {
		t.Errorf("pat secret not created: %v", f.created)
	}
	if f.created["blog-gen/repos/acme/widget/webhook-secret"] != "whsec" {
		t.Errorf("webhook secret not created: %v", f.created)
	}
}

func TestPutRepoCredentialsFallsBackToPutValue(t *testing.T) {
	f := newFake()
	f.existing["blog-gen/repos/acme/widget/pat"] = true // simulate re-registration
	s := New(f, "blog-gen/repos")

	if _, err := s.PutRepoCredentials(context.Background(), "acme", "widget", "newpat", "whsec"); err != nil {
		t.Fatalf("PutRepoCredentials: %v", err)
	}
	if f.putValues["blog-gen/repos/acme/widget/pat"] != "newpat" {
		t.Errorf("expected PutSecretValue for existing pat, got %v", f.putValues)
	}
}

func TestWebhookSecret(t *testing.T) {
	f := newFake()
	s := New(f, "blog-gen/repos")
	if _, err := s.PutRepoCredentials(context.Background(), "acme", "widget", "pat", "whsec"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := s.WebhookSecret(context.Background(), "blog-gen/repos/acme/widget")
	if err != nil {
		t.Fatalf("WebhookSecret: %v", err)
	}
	if got != "whsec" {
		t.Errorf("WebhookSecret = %q, want whsec", got)
	}
}

func TestValueResolvesArbitrarySecret(t *testing.T) {
	f := newFake()
	f.created["blog-gen/notifications/smtp-password"] = "s3cr3t"
	s := New(f, "blog-gen/repos")

	got, err := s.Value(context.Background(), "blog-gen/notifications/smtp-password")
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("Value = %q, want s3cr3t", got)
	}
	if _, err := s.Value(context.Background(), "missing"); err == nil {
		t.Error("expected error for a missing secret")
	}
}
