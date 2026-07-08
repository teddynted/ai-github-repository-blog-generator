package approval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

type capturePublisher struct {
	calls []struct {
		repo   string
		assets []generation.Content
	}
}

func (c *capturePublisher) Publish(_ context.Context, repo string, assets []generation.Content) error {
	c.calls = append(c.calls, struct {
		repo   string
		assets []generation.Content
	}{repo, assets})
	return nil
}

func seedPending(t *testing.T, dir, repo, date string, kinds ...string) {
	t.Helper()
	d := filepath.Join(dir, filepath.FromSlash(repo), date)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, k := range kinds {
		if err := os.WriteFile(filepath.Join(d, k+".md"), []byte("# "+k), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListPending(t *testing.T) {
	dir := t.TempDir()
	seedPending(t, dir, "acme/widget", "2026-07-09", "blog", "readme-improvements")
	seedPending(t, dir, "acme/other", "2026-07-08", "blog")

	items, err := (&PendingStore{Dir: dir}).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	// Sorted by repo: acme/other before acme/widget.
	if items[0].Repo != "acme/other" || items[1].Repo != "acme/widget" {
		t.Errorf("order = %s, %s", items[0].Repo, items[1].Repo)
	}
	if len(items[1].Assets) != 2 {
		t.Errorf("widget should have 2 assets, got %d", len(items[1].Assets))
	}
}

func TestListEmptyWhenAbsent(t *testing.T) {
	items, err := (&PendingStore{Dir: filepath.Join(t.TempDir(), "nope")}).List(context.Background())
	if err != nil || items != nil {
		t.Errorf("expected empty, got %v err=%v", items, err)
	}
}

func TestApproveAllPublishesAndClears(t *testing.T) {
	dir := t.TempDir()
	seedPending(t, dir, "acme/widget", "2026-07-09", "blog")
	seedPending(t, dir, "acme/other", "2026-07-08", "blog", "documentation")
	store := &PendingStore{Dir: dir}
	pub := &capturePublisher{}

	n, err := store.ApproveAll(context.Background(), pub)
	if err != nil {
		t.Fatalf("ApproveAll: %v", err)
	}
	if n != 2 || len(pub.calls) != 2 {
		t.Errorf("approved %d, published %d", n, len(pub.calls))
	}
	// Queue is now empty.
	items, _ := store.List(context.Background())
	if len(items) != 0 {
		t.Errorf("pending should be cleared, got %d", len(items))
	}
}

func TestRejectRemovesWithoutPublishing(t *testing.T) {
	dir := t.TempDir()
	seedPending(t, dir, "acme/widget", "2026-07-09", "blog")
	store := &PendingStore{Dir: dir}
	items, _ := store.List(context.Background())

	if err := store.Reject(context.Background(), items[0]); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	remaining, _ := store.List(context.Background())
	if len(remaining) != 0 {
		t.Errorf("rejected item should be gone, got %d", len(remaining))
	}
}
