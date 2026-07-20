package socialintel

import (
	"sort"
	"sync"
)

// MemoryRepository is an in-memory, append-only Repository. Snapshots are
// immutable: a save for an existing (platform, date) key is rejected. A SQLite
// adapter (see docs/social-intel-schema.sql) implements the same interface.
type MemoryRepository struct {
	mu       sync.RWMutex
	accounts map[string]AccountSnapshot   // key: platform|date
	content  map[string][]ContentSnapshot // key: platform|date
	byItem   map[string][]ContentSnapshot // key: platform|contentId
	accKeys  map[Platform][]Date
}

// NewMemoryRepository creates an empty repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		accounts: map[string]AccountSnapshot{},
		content:  map[string][]ContentSnapshot{},
		byItem:   map[string][]ContentSnapshot{},
		accKeys:  map[Platform][]Date{},
	}
}

func acctKey(p Platform, d Date) string { return string(p) + "|" + d }

func (r *MemoryRepository) SaveAccountSnapshot(s AccountSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := acctKey(s.Platform, s.Date)
	if _, ok := r.accounts[k]; ok {
		return ErrDuplicateSnapshot // immutable — never overwrite
	}
	r.accounts[k] = s
	r.accKeys[s.Platform] = append(r.accKeys[s.Platform], s.Date)
	return nil
}

func (r *MemoryRepository) SaveContentSnapshot(s ContentSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ck := acctKey(s.Platform, s.Date)
	// Reject a duplicate for the same content on the same day.
	for _, ex := range r.content[ck] {
		if ex.ContentID == s.ContentID {
			return ErrDuplicateSnapshot
		}
	}
	r.content[ck] = append(r.content[ck], s)
	ik := string(s.Platform) + "|" + s.ContentID
	r.byItem[ik] = append(r.byItem[ik], s)
	return nil
}

func (r *MemoryRepository) AccountSnapshotExists(p Platform, d Date) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.accounts[acctKey(p, d)]
	return ok
}

func (r *MemoryRepository) AccountSeries(p Platform) []AccountSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dates := append([]Date{}, r.accKeys[p]...)
	sort.Strings(dates)
	out := make([]AccountSnapshot, 0, len(dates))
	for _, d := range dates {
		out = append(out, r.accounts[acctKey(p, d)])
	}
	return out
}

func (r *MemoryRepository) ContentSnapshots(p Platform, d Date) []ContentSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ContentSnapshot{}, r.content[acctKey(p, d)]...)
}

func (r *MemoryRepository) ContentHistory(p Platform, contentID string) []ContentSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := append([]ContentSnapshot{}, r.byItem[string(p)+"|"+contentID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].Date < items[j].Date })
	return items
}

func (r *MemoryRepository) Platforms() []Platform {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Platform
	for p := range r.accKeys {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
