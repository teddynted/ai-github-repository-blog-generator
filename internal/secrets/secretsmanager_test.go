package secrets

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

const sharedID = "blog-gen/github/repositories"

// fakeAPI is an in-memory Secrets Manager: one string value per secret id.
type fakeAPI struct {
	values map[string]string
	puts   int
}

func newFake() *fakeAPI { return &fakeAPI{values: map[string]string{sharedID: "{}"}} }

func (f *fakeAPI) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	v, ok := f.values[*in.SecretId]
	if !ok {
		return nil, &smtypes.ResourceNotFoundException{}
	}
	return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(v)}, nil
}

func (f *fakeAPI) PutSecretValue(_ context.Context, in *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	f.puts++
	f.values[*in.SecretId] = *in.SecretString
	return &secretsmanager.PutSecretValueOutput{}, nil
}

func TestPutInsertUpdateAndDuplicate(t *testing.T) {
	f := newFake()
	s := New(f, sharedID)
	ctx := context.Background()

	// Insert acme/widget.
	existed, err := s.PutRepoCredentials(ctx, "acme/widget", "pat1", "ws1", false)
	if err != nil || existed {
		t.Fatalf("insert: existed=%v err=%v", existed, err)
	}
	// Duplicate without update → conflict, nothing written.
	putsBefore := f.puts
	_, err = s.PutRepoCredentials(ctx, "acme/widget", "pat2", "ws2", false)
	if apperror.CodeOf(err) != apperror.CodeConflict {
		t.Fatalf("duplicate should conflict, got %v", err)
	}
	if f.puts != putsBefore {
		t.Errorf("duplicate must not write")
	}
	// Update with allowUpdate → overwrites.
	existed, err = s.PutRepoCredentials(ctx, "acme/widget", "pat2", "ws2", true)
	if err != nil || !existed {
		t.Fatalf("update: existed=%v err=%v", existed, err)
	}

	// A second repo coexists (no clobber of the first).
	if _, err := s.PutRepoCredentials(ctx, "acme/gadget", "patG", "wsG", false); err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if pat, _ := s.PAT(ctx, "acme/widget"); pat != "pat2" {
		t.Errorf("widget pat = %q, want pat2", pat)
	}
	if ws, _ := s.WebhookSecret(ctx, "acme/gadget"); ws != "wsG" {
		t.Errorf("gadget webhook secret = %q, want wsG", ws)
	}
}

func TestGetAndLookupMissing(t *testing.T) {
	f := newFake()
	s := New(f, sharedID)
	ctx := context.Background()
	if _, ok, err := s.Get(ctx, "no/repo"); err != nil || ok {
		t.Errorf("missing key: ok=%v err=%v", ok, err)
	}
	if _, err := s.WebhookSecret(ctx, "no/repo"); apperror.CodeOf(err) != apperror.CodeNotFound {
		t.Errorf("missing webhook secret should be not_found, got %v", err)
	}
	if _, err := s.PAT(ctx, "no/repo"); apperror.CodeOf(err) != apperror.CodeNotFound {
		t.Errorf("missing pat should be not_found, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	f := newFake()
	s := New(f, sharedID)
	ctx := context.Background()
	_, _ = s.PutRepoCredentials(ctx, "acme/widget", "p", "w", false)
	_, _ = s.PutRepoCredentials(ctx, "acme/gadget", "p", "w", false)

	existed, err := s.DeleteRepoCredentials(ctx, "acme/widget")
	if err != nil || !existed {
		t.Fatalf("delete existing: existed=%v err=%v", existed, err)
	}
	if _, ok, _ := s.Get(ctx, "acme/widget"); ok {
		t.Error("widget should be gone")
	}
	if _, ok, _ := s.Get(ctx, "acme/gadget"); !ok {
		t.Error("gadget must remain")
	}
	// Deleting an absent key is a no-op success with no extra write.
	puts := f.puts
	existed, err = s.DeleteRepoCredentials(ctx, "acme/widget")
	if err != nil || existed {
		t.Errorf("delete absent: existed=%v err=%v", existed, err)
	}
	if f.puts != puts {
		t.Error("deleting absent key must not write")
	}
}

func TestValueResolvesArbitrarySecret(t *testing.T) {
	f := newFake()
	f.values["blog-gen/notifications/smtp-password"] = "s3cr3t"
	s := New(f, sharedID)
	got, err := s.Value(context.Background(), "blog-gen/notifications/smtp-password")
	if err != nil || got != "s3cr3t" {
		t.Fatalf("Value = %q err %v, want s3cr3t", got, err)
	}
	if _, err := s.Value(context.Background(), "missing"); err == nil {
		t.Error("expected error for a missing secret")
	}
}
