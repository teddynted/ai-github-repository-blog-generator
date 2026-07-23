package mcp

import (
	"fmt"
	"sort"
	"sync"
)

// Factory builds a live Server. Plug-ins register a Factory (not an instance) so
// construction is lazy and each Client can hold its own server instances.
type Factory func() (Server, error)

type registration struct {
	descriptor ServerDescriptor
	factory    Factory
}

// Registry is a plug-in registry of MCP servers, following the database/sql
// driver pattern: servers self-register from package init() and are instantiated
// on demand. It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	entries   map[string]registration
	instances map[string]Server
}

// NewRegistry returns an empty registry (tests build isolated ones; production
// uses DefaultRegistry via the package-level helpers).
func NewRegistry() *Registry {
	return &Registry{entries: map[string]registration{}, instances: map[string]Server{}}
}

// Register adds a server plug-in. It errors if the id is already taken so a
// mis-wired build fails loudly rather than shadowing a server.
func (r *Registry) Register(d ServerDescriptor, f Factory) error {
	if d.ID == "" {
		return fmt.Errorf("%w: empty server id", ErrInvalidArgument)
	}
	if f == nil {
		return fmt.Errorf("%w: nil factory for %q", ErrInvalidArgument, d.ID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[d.ID]; ok {
		return fmt.Errorf("%w: %q", ErrAlreadyRegistered, d.ID)
	}
	r.entries[d.ID] = registration{descriptor: d, factory: f}
	return nil
}

// MustRegister is Register that panics on error, for package init() blocks.
func (r *Registry) MustRegister(d ServerDescriptor, f Factory) {
	if err := r.Register(d, f); err != nil {
		panic(err)
	}
}

// Descriptor returns the descriptor for an id.
func (r *Registry) Descriptor(id string) (ServerDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	return e.descriptor, ok
}

// Descriptors returns every registered descriptor, sorted by id (deterministic).
func (r *Registry) Descriptors() []ServerDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ServerDescriptor, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e.descriptor)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Instance returns the live Server for an id, building and caching it on first
// use. Returns ErrServerNotFound if the id is not registered.
func (r *Registry) Instance(id string) (Server, error) {
	r.mu.RLock()
	if s, ok := r.instances[id]; ok {
		r.mu.RUnlock()
		return s, nil
	}
	e, ok := r.entries[id]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrServerNotFound, id)
	}
	s, err := e.factory()
	if err != nil {
		return nil, fmt.Errorf("mcp: build server %q: %w", id, err)
	}
	r.mu.Lock()
	// Another goroutine may have won the race; reuse its instance.
	if existing, ok := r.instances[id]; ok {
		r.mu.Unlock()
		return existing, nil
	}
	r.instances[id] = s
	r.mu.Unlock()
	return s, nil
}

// DefaultRegistry is the process-wide registry that built-in servers register
// into from their package init() functions.
var DefaultRegistry = NewRegistry()

// Register adds a plug-in to the DefaultRegistry.
func Register(d ServerDescriptor, f Factory) error { return DefaultRegistry.Register(d, f) }

// MustRegister adds a plug-in to the DefaultRegistry, panicking on error.
func MustRegister(d ServerDescriptor, f Factory) { DefaultRegistry.MustRegister(d, f) }
