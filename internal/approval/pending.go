package approval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

// Publisher delivers approved content to the live destination.
// *publish.FilePublisher and *publish.S3Publisher satisfy it.
type Publisher interface {
	Publish(ctx context.Context, repoFullName string, assets []generation.Content) error
}

// Pending is one repository's held content package (a dated directory of
// <kind>.md files under PendingDir/<owner>/<name>/<date>).
type Pending struct {
	Repo   string // "owner/name"
	Date   string // YYYY-MM-DD
	Dir    string // absolute path
	Assets []generation.Content
}

// PendingStore is the review queue backed by the pending directory: the
// approvals "dashboard". Approving publishes the content to the live
// destination and clears it from the queue.
type PendingStore struct {
	Dir string
}

// List returns every held content package, sorted by repo then date.
func (s *PendingStore) List(_ context.Context) ([]Pending, error) {
	owners, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read pending dir: %w", err)
	}

	var out []Pending
	for _, owner := range owners {
		if !owner.IsDir() {
			continue
		}
		names, _ := os.ReadDir(filepath.Join(s.Dir, owner.Name()))
		for _, name := range names {
			if !name.IsDir() {
				continue
			}
			dates, _ := os.ReadDir(filepath.Join(s.Dir, owner.Name(), name.Name()))
			for _, date := range dates {
				if !date.IsDir() {
					continue
				}
				dir := filepath.Join(s.Dir, owner.Name(), name.Name(), date.Name())
				assets, err := readAssets(dir)
				if err != nil {
					return nil, err
				}
				if len(assets) == 0 {
					continue
				}
				out = append(out, Pending{
					Repo:   owner.Name() + "/" + name.Name(),
					Date:   date.Name(),
					Dir:    dir,
					Assets: assets,
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].Date < out[j].Date
	})
	return out, nil
}

// Approve publishes a pending package to dst and removes it from the queue.
func (s *PendingStore) Approve(ctx context.Context, p Pending, dst Publisher) error {
	if err := dst.Publish(ctx, p.Repo, p.Assets); err != nil {
		return fmt.Errorf("publish %s: %w", p.Repo, err)
	}
	if err := os.RemoveAll(p.Dir); err != nil {
		return fmt.Errorf("clear pending %s: %w", p.Dir, err)
	}
	return nil
}

// Reject removes a pending package without publishing.
func (s *PendingStore) Reject(_ context.Context, p Pending) error {
	return os.RemoveAll(p.Dir)
}

// ApproveAll publishes every pending package (auto-approve) and returns the
// number approved.
func (s *PendingStore) ApproveAll(ctx context.Context, dst Publisher) (int, error) {
	items, err := s.List(ctx)
	if err != nil {
		return 0, err
	}
	for i, p := range items {
		if err := s.Approve(ctx, p, dst); err != nil {
			return i, err
		}
	}
	return len(items), nil
}

func readAssets(dir string) ([]generation.Content, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var assets []generation.Content
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		kind := generation.Kind(strings.TrimSuffix(e.Name(), ".md"))
		assets = append(assets, generation.Content{Kind: kind, Markdown: string(content)})
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Kind < assets[j].Kind })
	return assets, nil
}
