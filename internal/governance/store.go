package governance

import "sync"

// MemoryRepository is an in-memory Repository adapter. A SQLite or PostgreSQL
// adapter can implement the same interface without changing the engines.
type MemoryRepository struct {
	mu    sync.RWMutex
	items map[string]*WorkflowState
	order []string
}

// NewMemoryRepository creates an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{items: map[string]*WorkflowState{}}
}

// Save upserts the state by its content ID.
func (r *MemoryRepository) Save(state *WorkflowState) error {
	if state == nil || state.Content.ID == "" {
		return ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id := state.Content.ID
	if _, exists := r.items[id]; !exists {
		r.order = append(r.order, id)
	}
	r.items[id] = state
	return nil
}

// Get returns the state for a content ID.
func (r *MemoryRepository) Get(id string) (*WorkflowState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

// List returns all states in insertion order.
func (r *MemoryRepository) List() ([]*WorkflowState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*WorkflowState, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.items[id])
	}
	return out, nil
}
