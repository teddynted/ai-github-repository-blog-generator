// Package metadata adapts Amazon DynamoDB to the platform's MetadataStore
// port, persisting repository metadata (never the PAT — only its reference).
package metadata

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
)

// API is the subset of the DynamoDB client this adapter uses. The real
// *dynamodb.Client satisfies it; tests supply a fake.
type API interface {
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
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
