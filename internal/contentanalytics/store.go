package contentanalytics

import (
	"sort"
	"sync"
)

// MemoryRepository is a thread-safe, in-memory Repository. Publications are
// upsertable current records; metrics snapshots are IMMUTABLE and append-only —
// a duplicate (publicationID, period, date) is rejected, never overwritten. A
// production SQLite/Postgres adapter implements the same Repository interface
// (see docs/content-analytics-schema.sql).
type MemoryRepository struct {
	mu    sync.RWMutex
	pubs  map[string]Publication       // publicationID -> current record
	snaps map[string][]MetricsSnapshot // "publicationID|period" -> snapshots (oldest first)
	seen  map[string]bool              // "publicationID|period|date" -> immutability guard
}

// NewMemoryRepository constructs an empty repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		pubs:  map[string]Publication{},
		snaps: map[string][]MetricsSnapshot{},
		seen:  map[string]bool{},
	}
}

func (r *MemoryRepository) SavePublication(p Publication) error {
	if p.ID == "" {
		return errMissing("publication id is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pubs[p.ID] = p
	return nil
}

func (r *MemoryRepository) Publications() []Publication {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Publication, 0, len(r.pubs))
	for _, p := range r.pubs {
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *MemoryRepository) PublicationsByPlatform(pl Platform) []Publication {
	var out []Publication
	for _, p := range r.Publications() {
		if p.Platform == pl {
			out = append(out, p)
		}
	}
	return out
}

func snapKey(pubID string, period Period) string { return pubID + "|" + string(period) }
func seenKey(pubID string, period Period, date Date) string {
	return pubID + "|" + string(period) + "|" + date
}

func (r *MemoryRepository) SaveSnapshot(s MetricsSnapshot) error {
	if s.PublicationID == "" {
		return errMissing("snapshot publication id is empty")
	}
	if s.Period == "" {
		s.Period = PeriodDaily
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	sk := seenKey(s.PublicationID, s.Period, s.Date)
	if r.seen[sk] {
		return ErrDuplicateSnapshot
	}
	r.seen[sk] = true
	k := snapKey(s.PublicationID, s.Period)
	list := append(r.snaps[k], s)
	sort.SliceStable(list, func(i, j int) bool { return list[i].Date < list[j].Date })
	r.snaps[k] = list
	return nil
}

func (r *MemoryRepository) SnapshotExists(publicationID string, period Period, date Date) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.seen[seenKey(publicationID, period, date)]
}

func (r *MemoryRepository) Snapshots(publicationID string, period Period) []MetricsSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	src := r.snaps[snapKey(publicationID, period)]
	out := make([]MetricsSnapshot, len(src))
	copy(out, src)
	return out
}

func (r *MemoryRepository) SnapshotsOn(period Period, date Date) []MetricsSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []MetricsSnapshot
	for k, list := range r.snaps {
		if !hasSuffixPeriod(k, period) {
			continue
		}
		for _, s := range list {
			if s.Date == date {
				out = append(out, s)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PublicationID < out[j].PublicationID })
	return out
}

func (r *MemoryRepository) LatestSnapshot(publicationID string, period Period) (MetricsSnapshot, bool) {
	list := r.Snapshots(publicationID, period)
	if len(list) == 0 {
		return MetricsSnapshot{}, false
	}
	return list[len(list)-1], true
}

func (r *MemoryRepository) Platforms() []Platform {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set := map[Platform]bool{}
	for _, p := range r.pubs {
		set[p.Platform] = true
	}
	out := make([]Platform, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// hasSuffixPeriod reports whether a snapshot key belongs to a period.
func hasSuffixPeriod(key string, period Period) bool {
	suffix := "|" + string(period)
	return len(key) >= len(suffix) && key[len(key)-len(suffix):] == suffix
}
