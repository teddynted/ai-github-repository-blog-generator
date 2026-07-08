package memory

import (
	"context"
	"testing"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

func newStore(t *testing.T) *Store {
	return &Store{Dir: t.TempDir(), Now: func() time.Time { return time.Unix(1700000000, 0) }}
}

func TestLoadEmptyWhenAbsent(t *testing.T) {
	m, err := newStore(t).Load(context.Background(), "acme/widget")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Published) != 0 || m.LastProcessedCommitSHA != "" {
		t.Errorf("expected empty memory, got %+v", m)
	}
}

func TestRecordThenAlreadyPublished(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	already, _ := s.AlreadyPublished(ctx, "acme/widget", "sha1")
	if already {
		t.Fatal("should not be published before recording")
	}

	if err := s.RecordPublished(ctx, "acme/widget", "sha1", []string{"blog", "readme-improvements"}); err != nil {
		t.Fatalf("RecordPublished: %v", err)
	}

	already, err := s.AlreadyPublished(ctx, "acme/widget", "sha1")
	if err != nil || !already {
		t.Fatalf("expected published after recording: %v", err)
	}

	m, _ := s.Load(ctx, "acme/widget")
	if m.LastProcessedCommitSHA != "sha1" || len(m.Published) != 1 || len(m.Published[0].Kinds) != 2 {
		t.Errorf("persisted memory = %+v", m)
	}
}

func TestRecordIsIdempotentPerCommit(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.RecordPublished(ctx, "acme/widget", "sha1", []string{"blog"})
	_ = s.RecordPublished(ctx, "acme/widget", "sha1", []string{"blog", "docs"})

	m, _ := s.Load(ctx, "acme/widget")
	if len(m.Published) != 1 {
		t.Fatalf("re-recording the same commit should not duplicate: %+v", m.Published)
	}
	if len(m.Published[0].Kinds) != 2 {
		t.Errorf("entry should be refreshed: %+v", m.Published[0])
	}
}

func TestSeparateReposIsolated(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.RecordPublished(ctx, "acme/widget", "sha1", nil)
	already, _ := s.AlreadyPublished(ctx, "acme/other", "sha1")
	if already {
		t.Error("memory must be per-repository")
	}
}

func TestInvalidRepoName(t *testing.T) {
	s := newStore(t)
	if _, err := s.Load(context.Background(), "../etc"); apperror.CodeOf(err) != apperror.CodeInvalidInput {
		t.Errorf("code = %s, want invalid_input", apperror.CodeOf(err))
	}
}
