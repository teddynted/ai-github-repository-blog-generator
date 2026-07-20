package platform

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/semver"
)

// Registry is the thread-safe provider registry: the single place plug-ins are
// discovered, registered, and selected. It is keyed generically by Kind, so new
// provider categories need no registry changes. Business services depend on the
// registry + domain interfaces only — never on a concrete provider.
type Registry struct {
	mu   sync.RWMutex
	regs map[Kind]map[string]Registration
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry {
	return &Registry{regs: map[Kind]map[string]Registration{}}
}

// Register adds a provider. It validates required fields and API-version
// compatibility, and rejects duplicates — enforcing that registration is
// deliberate and version-safe. This is the ONLY mutation point; adding a
// provider is a Register call, never an edit to existing code.
func (r *Registry) Register(reg Registration) error {
	d := reg.Descriptor
	if d.ID == "" || d.Kind == "" || reg.Factory == nil {
		return fmt.Errorf("%w: id, kind and factory are required", ErrInvalidRegistration)
	}
	if !compatible(d.apiVersion()) {
		return fmt.Errorf("%w: provider %q targets API %s, platform is %s", ErrIncompatible, d.ID, d.apiVersion(), APIVersion)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.regs[d.Kind] == nil {
		r.regs[d.Kind] = map[string]Registration{}
	}
	if _, exists := r.regs[d.Kind][d.ID]; exists {
		return fmt.Errorf("%w: %s/%s", ErrAlreadyRegistered, d.Kind, d.ID)
	}
	r.regs[d.Kind][d.ID] = reg
	return nil
}

// MustRegister registers or panics — for package init() auto-registration where
// a duplicate is a programming error.
func (r *Registry) MustRegister(reg Registration) {
	if err := r.Register(reg); err != nil {
		panic(err)
	}
}

// Lookup returns the registration for a (kind, id).
func (r *Registry) Lookup(kind Kind, id string) (Registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.regs[kind]
	if m == nil {
		return Registration{}, false
	}
	reg, ok := m[id]
	return reg, ok
}

// Descriptors returns the descriptors registered for a kind, highest priority
// first (ties broken by id for determinism).
func (r *Registry) Descriptors(kind Kind) []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Descriptor
	for _, reg := range r.regs[kind] {
		out = append(out, reg.Descriptor)
	}
	sortDescriptors(out)
	return out
}

// AllDescriptors returns every registered descriptor, grouped-stable by kind.
func (r *Registry) AllDescriptors() []Descriptor {
	var out []Descriptor
	for _, k := range r.Kinds() {
		out = append(out, r.Descriptors(k)...)
	}
	return out
}

// Kinds returns the registered kinds, sorted.
func (r *Registry) Kinds() []Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Kind
	for k, m := range r.regs {
		if len(m) > 0 {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Create instantiates a specific provider by (kind, id) with configuration. This
// is the factory entry point — construction is lazy, so credentials/SDKs are
// only touched when a provider is actually built.
func (r *Registry) Create(kind Kind, id string, cfg ConfigSource) (Provider, error) {
	reg, ok := r.Lookup(kind, id)
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrNotFound, kind, id)
	}
	return reg.Factory(cfg)
}

// Candidates returns the descriptors for a kind that satisfy the options,
// ordered by priority. It drives fallback: callers try candidates in order.
func (r *Registry) Candidates(kind Kind, opts ...SelectOption) []Descriptor {
	o := applyOptions(opts)
	var out []Descriptor
	for _, d := range r.Descriptors(kind) {
		if o.id != "" && d.ID != o.id {
			continue
		}
		if o.capability != "" && !d.HasCapability(o.capability) {
			continue
		}
		if o.managedOnly && !d.Managed {
			continue
		}
		if o.localOnly && d.Managed {
			continue
		}
		if d.Priority < o.minPriority {
			continue
		}
		out = append(out, d)
	}
	return out
}

// Select picks the best healthy provider for a kind: it filters by capability and
// options, orders by priority, then instantiates and health-checks candidates in
// order, returning the first healthy one — automatic priority selection with
// fallback. No caller ever hard-codes a provider.
func (r *Registry) Select(ctx context.Context, kind Kind, cfg ConfigSource, opts ...SelectOption) (Provider, error) {
	o := applyOptions(opts)
	candidates := r.Candidates(kind, opts...)
	if len(candidates) == 0 {
		return nil, &SelectionError{Kind: kind, Capability: o.capability, Err: ErrNotFound}
	}
	var tried []string
	var lastErr error = ErrNoHealthyProvider
	for _, d := range candidates {
		tried = append(tried, d.ID)
		p, err := r.Create(kind, d.ID, cfg)
		if err != nil {
			lastErr = err
			continue
		}
		if p.Health(ctx).Healthy() {
			return p, nil
		}
	}
	return nil, &SelectionError{Kind: kind, Capability: o.capability, Tried: tried, Err: lastErr}
}

// sortDescriptors orders by priority desc, then id asc (stable, deterministic).
func sortDescriptors(ds []Descriptor) {
	sort.SliceStable(ds, func(i, j int) bool {
		if ds[i].Priority != ds[j].Priority {
			return ds[i].Priority > ds[j].Priority
		}
		return ds[i].ID < ds[j].ID
	})
}

// compatible reports whether a provider's declared API major matches the platform.
func compatible(providerAPI string) bool {
	pv, err := semver.Parse(providerAPI)
	if err != nil {
		return false
	}
	cur, err := semver.Parse(APIVersion)
	if err != nil {
		return false
	}
	return pv.Major == cur.Major
}
