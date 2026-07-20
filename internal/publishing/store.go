package publishing

import "sync"

// MemoryRepository is an in-memory Repository. SQLite/PostgreSQL adapters
// implement the same interface without changing the engine.
type MemoryRepository struct {
	mu    sync.RWMutex
	items map[string]*Publication
	order []string
}

// NewMemoryRepository creates an empty repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{items: map[string]*Publication{}}
}

func (r *MemoryRepository) Save(p *Publication) error {
	if p == nil || p.ID == "" {
		return ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[p.ID]; !ok {
		r.order = append(r.order, p.ID)
	}
	r.items[p.ID] = p
	return nil
}

func (r *MemoryRepository) Get(id string) (*Publication, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (r *MemoryRepository) List() ([]*Publication, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Publication, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.items[id])
	}
	return out, nil
}

func (r *MemoryRepository) ByStatus(s PublicationStatus) ([]*Publication, error) {
	all, _ := r.List()
	var out []*Publication
	for _, p := range all {
		if p.Status == s {
			out = append(out, p)
		}
	}
	return out, nil
}
