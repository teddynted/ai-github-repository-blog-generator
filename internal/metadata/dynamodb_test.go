package metadata

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
)

type fakeDDB struct {
	table string
	item  map[string]ddbtypes.AttributeValue
}

func (f *fakeDDB) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.table = *in.TableName
	f.item = in.Item
	return &dynamodb.PutItemOutput{}, nil
}

func TestPutMarshalsRepository(t *testing.T) {
	f := &fakeDDB{}
	s := New(f, "blog-gen-repositories")

	r := repo.Repository{
		RepoFullName:   "acme/widget",
		RepositoryID:   7,
		Owner:          "acme",
		Name:           "widget",
		DefaultBranch:  "main",
		WebhookID:      555,
		TriggerPattern: "blog:",
		Enabled:        true,
		SecretRef:      "blog-gen/repos/acme/widget",
	}
	if err := s.Put(context.Background(), r); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if f.table != "blog-gen-repositories" {
		t.Errorf("table = %q", f.table)
	}
	pk, ok := f.item["repo_full_name"].(*ddbtypes.AttributeValueMemberS)
	if !ok || pk.Value != "acme/widget" {
		t.Errorf("partition key not marshaled: %#v", f.item["repo_full_name"])
	}
	if _, ok := f.item["secret_ref"].(*ddbtypes.AttributeValueMemberS); !ok {
		t.Errorf("secret_ref missing from item: %#v", f.item)
	}
	// The PAT must never appear in the persisted item.
	if _, present := f.item["pat"]; present {
		t.Error("item must not contain a pat attribute")
	}
}
