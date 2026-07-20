package platform

import (
	"context"
	"math"
	"sort"
	"sync"
)

// Vector capabilities.
const (
	CapEmbedStore Capability = "embed-store"
	CapSimilarity Capability = "similarity-search"
)

// Vector is an embedding with an id and optional metadata/payload.
type Vector struct {
	ID       string
	Values   []float32
	Metadata map[string]string
}

// Match is a similarity-search hit.
type Match struct {
	ID       string
	Score    float64 // cosine similarity, higher is closer
	Metadata map[string]string
}

// VectorStore is the abstraction for future vector databases (pgvector, Pinecone,
// Weaviate, Qdrant, Milvus, Chroma, OpenSearch). A new backend is a new
// implementation; callers use this interface only.
type VectorStore interface {
	Provider
	Upsert(ctx context.Context, vectors []Vector) error
	Query(ctx context.Context, embedding []float32, k int) ([]Match, error)
	Delete(ctx context.Context, ids []string) error
}

// memoryVectorStore is the reference vector store: an in-memory cosine-similarity
// index. It is genuinely functional (not a stub) so the abstraction is provable
// end-to-end offline; a production backend implements the same three methods.
type memoryVectorStore struct {
	id  string
	mu  sync.RWMutex
	vec map[string]Vector
}

func newMemoryVectorStore(id string) *memoryVectorStore {
	return &memoryVectorStore{id: id, vec: map[string]Vector{}}
}

func (m *memoryVectorStore) ID() string { return m.id }
func (m *memoryVectorStore) Kind() Kind { return KindVector }
func (m *memoryVectorStore) Capabilities() []Capability {
	return []Capability{CapEmbedStore, CapSimilarity}
}
func (m *memoryVectorStore) Health(context.Context) Health { return OK() }

func (m *memoryVectorStore) Upsert(_ context.Context, vectors []Vector) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range vectors {
		if v.ID == "" {
			return ErrUnsupported
		}
		m.vec[v.ID] = v
	}
	return nil
}

func (m *memoryVectorStore) Query(_ context.Context, embedding []float32, k int) ([]Match, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var matches []Match
	for _, v := range m.vec {
		matches = append(matches, Match{ID: v.ID, Score: cosine(embedding, v.Values), Metadata: v.Metadata})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	if k > 0 && len(matches) > k {
		matches = matches[:k]
	}
	return matches, nil
}

func (m *memoryVectorStore) Delete(_ context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		delete(m.vec, id)
	}
	return nil
}

// NewMemoryVectorStore builds the reference in-memory vector store.
func NewMemoryVectorStore(id string) VectorStore { return newMemoryVectorStore(id) }

// cosine returns the cosine similarity of two vectors (0 when either is empty or
// dimensions mismatch — a safe, honest default).
func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func registerVector(r *Registry) {
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "memory", Kind: KindVector, Name: "In-memory (reference)", Version: "1.0.0",
			Priority: 1, Description: "Cosine-similarity in-memory vector store",
			Capabilities: []Capability{CapEmbedStore, CapSimilarity},
		},
		Factory: func(ConfigSource) (Provider, error) { return newMemoryVectorStore("memory"), nil },
	})
}
