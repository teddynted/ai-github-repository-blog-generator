// Package metadata adapts Amazon DynamoDB to the platform's MetadataStore
// port, persisting repository metadata (never the PAT — only its reference).
package metadata

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
)

// API is the subset of the DynamoDB client this adapter uses. The real
// *dynamodb.Client satisfies it; tests supply a fake.
type API interface {
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

// Store persists repository metadata in a DynamoDB table.
type Store struct {
	api   API
	table string
}

// New builds a Store for the given table.
func New(api API, table string) *Store {
	return &Store{api: api, table: table}
}

// Put writes (or overwrites) a repository's metadata item.
func (s *Store) Put(ctx context.Context, r repo.Repository) error {
	item, err := attributevalue.MarshalMap(r)
	if err != nil {
		return fmt.Errorf("marshal repository: %w", err)
	}
	_, err = s.api.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamodb put item: %w", err)
	}
	return nil
}

// Get fetches a repository by its "owner/name" key. The bool is false (with a
// nil error) when no such repository is registered.
func (s *Store) Get(ctx context.Context, fullName string) (repo.Repository, bool, error) {
	out, err := s.api.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]ddbtypes.AttributeValue{
			"repo_full_name": &ddbtypes.AttributeValueMemberS{Value: fullName},
		},
	})
	if err != nil {
		return repo.Repository{}, false, fmt.Errorf("dynamodb get item: %w", err)
	}
	if out.Item == nil {
		return repo.Repository{}, false, nil
	}
	var r repo.Repository
	if err := attributevalue.UnmarshalMap(out.Item, &r); err != nil {
		return repo.Repository{}, false, fmt.Errorf("unmarshal repository: %w", err)
	}
	return r, true, nil
}
