// Package memory implements Repository Memory: a persistent, per-repository
// record of what has already been published, used to avoid regenerating
// duplicate content. Per the requirements it persists on the filesystem (the
// EBS volume on the instance), not a managed database, and never stores secrets.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// Entry records one published run.
type Entry struct {
	CommitSHA string   `json:"commit_sha"`
	Kinds     []string `json:"kinds"`
	At        string   `json:"at"`
}

// Memory is a repository's persisted record.
type Memory struct {
	LastProcessedCommitSHA string  `json:"last_processed_commit_sha"`
	Published              []Entry `json:"published"`
}

// Store persists Repository Memory as JSON under a base directory. A single
// worker processes runs sequentially, so no cross-process locking is needed.
type Store struct {
	Dir    string
	Now    func() time.Time
	Logger *slog.Logger
}

// New builds a Store rooted at dir.
func New(dir string) *Store { return &Store{Dir: dir} }

// Load returns a repository's memory (empty, no error, when none exists yet).
func (s *Store) Load(_ context.Context, repoFullName string) (Memory, error) {
	path, err := s.path(repoFullName)
	if err != nil {
		return Memory{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Memory{}, nil
	}
	if err != nil {
		return Memory{}, fmt.Errorf("read memory: %w", err)
	}
	var m Memory
	if err := json.Unmarshal(data, &m); err != nil {
		return Memory{}, fmt.Errorf("parse memory: %w", err)
	}
	return m, nil
}

// AlreadyPublished reports whether content has been published for commitSHA.
func (s *Store) AlreadyPublished(ctx context.Context, repoFullName, commitSHA string) (bool, error) {
	if commitSHA == "" {
		return false, nil
	}
	m, err := s.Load(ctx, repoFullName)
	if err != nil {
		return false, err
	}
	for _, e := range m.Published {
		if e.CommitSHA == commitSHA {
			return true, nil
		}
	}
	return false, nil
}

// RecordPublished appends (or refreshes) an entry for commitSHA and updates the
// last-processed marker.
func (s *Store) RecordPublished(ctx context.Context, repoFullName, commitSHA string, kinds []string) error {
	m, err := s.Load(ctx, repoFullName)
	if err != nil {
		return err
	}
	entry := Entry{CommitSHA: commitSHA, Kinds: kinds, At: s.now().UTC().Format(time.RFC3339)}

	replaced := false
	for i, e := range m.Published {
		if e.CommitSHA == commitSHA {
			m.Published[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		m.Published = append(m.Published, entry)
	}
	if commitSHA != "" {
		m.LastProcessedCommitSHA = commitSHA
	}

	path, err := s.path(repoFullName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write memory: %w", err)
	}
	if s.Logger != nil {
		s.Logger.Info("recorded repository memory",
			slog.String("repo", repoFullName), slog.String("commit_sha", commitSHA))
	}
	return nil
}

func (s *Store) path(repoFullName string) (string, error) {
	owner, name, ok := strings.Cut(repoFullName, "/")
	if !ok || owner == "" || name == "" || strings.Contains(repoFullName, "..") {
		return "", apperror.New(apperror.CodeInvalidInput, "invalid repository name")
	}
	return filepath.Join(s.Dir, owner, name, "memory.json"), nil
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
