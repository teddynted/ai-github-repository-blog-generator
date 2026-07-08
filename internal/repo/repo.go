// Package repo holds the repository domain model shared across the platform
// and the logic for parsing GitHub repository URLs. It has no infrastructure
// dependencies.
package repo

import (
	"fmt"
	"net/url"
	"strings"
)

// Repository is the metadata persisted for each registered repository. The
// GitHub PAT is never stored here — only SecretRef, a reference to the
// Secrets Manager entry that holds the credentials.
type Repository struct {
	RepoFullName           string `dynamodbav:"repo_full_name" json:"repo_full_name"`
	RepositoryID           int64  `dynamodbav:"repository_id" json:"repository_id"`
	Owner                  string `dynamodbav:"owner" json:"owner"`
	Name                   string `dynamodbav:"name" json:"name"`
	URL                    string `dynamodbav:"url" json:"url"`
	DefaultBranch          string `dynamodbav:"default_branch" json:"default_branch"`
	WebhookID              int64  `dynamodbav:"webhook_id" json:"webhook_id"`
	TriggerPattern         string `dynamodbav:"trigger_pattern" json:"trigger_pattern"`
	Enabled                bool   `dynamodbav:"enabled" json:"enabled"`
	SecretRef              string `dynamodbav:"secret_ref" json:"secret_ref"`
	LastProcessedCommitSHA string `dynamodbav:"last_processed_commit_sha" json:"last_processed_commit_sha"`
	RegisteredAt           string `dynamodbav:"registered_at" json:"registered_at"`
}

// FullName builds the canonical "owner/name" identifier.
func FullName(owner, name string) string { return owner + "/" + name }

// ParseRepositoryURL extracts the owner and repository name from a GitHub
// repository URL. It accepts HTTPS URLs (https://github.com/owner/name[.git])
// and scp-like SSH URLs (git@github.com:owner/name[.git]).
func ParseRepositoryURL(raw string) (owner, name string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("repository url is empty")
	}

	// scp-like SSH form: git@github.com:owner/name(.git)
	if strings.HasPrefix(raw, "git@") {
		idx := strings.Index(raw, ":")
		if idx < 0 {
			return "", "", fmt.Errorf("invalid ssh repository url: %q", raw)
		}
		return splitOwnerName(raw[idx+1:])
	}

	u, perr := url.Parse(raw)
	if perr != nil {
		return "", "", fmt.Errorf("invalid repository url: %w", perr)
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "github.com") {
		return "", "", fmt.Errorf("only github.com repositories are supported, got %q", u.Host)
	}
	return splitOwnerName(u.Path)
}

func splitOwnerName(path string) (string, string, error) {
	path = strings.TrimPrefix(strings.TrimSpace(path), "/")
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repository path must be owner/name, got %q", path)
	}
	return parts[0], parts[1], nil
}
