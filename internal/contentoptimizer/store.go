package contentoptimizer

import (
	"sort"
	"sync"
)

// MemoryRepository is a thread-safe, append-only, in-memory Repository. Every
// write appends; historical optimization data is never overwritten. Prompt
// versions are stored per template in version order, so rollback is a read of an
// earlier version. A production SQLite/Postgres adapter implements the same
// Repository interface (see docs/content-optimizer-schema.sql).
type MemoryRepository struct {
	mu         sync.RWMutex
	reports    []OptimizationReport
	reportSeen map[string]bool
	recs       []Recommendation
	patterns   []Pattern
	trends     []TrendReport
	prompts    map[string][]PromptVersion // templateID -> versions (ascending)
	approvals  []Approval
}

// NewMemoryRepository constructs an empty repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		reportSeen: map[string]bool{},
		prompts:    map[string][]PromptVersion{},
	}
}

func (r *MemoryRepository) SaveReport(rep OptimizationReport) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reportSeen[rep.RunID] {
		return ErrDuplicateReport
	}
	r.reportSeen[rep.RunID] = true
	r.reports = append(r.reports, rep)
	return nil
}

func (r *MemoryRepository) Reports() []OptimizationReport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]OptimizationReport(nil), r.reports...)
}

func (r *MemoryRepository) LatestReport() (OptimizationReport, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.reports) == 0 {
		return OptimizationReport{}, false
	}
	return r.reports[len(r.reports)-1], true
}

func (r *MemoryRepository) SaveRecommendations(runID string, recs []Recommendation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs = append(r.recs, recs...)
	return nil
}

func (r *MemoryRepository) Recommendations() []Recommendation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Recommendation(nil), r.recs...)
}

func (r *MemoryRepository) SavePattern(p Pattern) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patterns = append(r.patterns, p)
	return nil
}

func (r *MemoryRepository) Patterns() []Pattern {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Pattern(nil), r.patterns...)
}

func (r *MemoryRepository) SaveTrend(runID string, t TrendReport) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trends = append(r.trends, t)
	return nil
}

func (r *MemoryRepository) Trends() []TrendReport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]TrendReport(nil), r.trends...)
}

// SavePromptVersion appends a version. It rejects a duplicate (templateID,
// version) so history is immutable; a new proposal must use the next version.
func (r *MemoryRepository) SavePromptVersion(v PromptVersion) error {
	if v.TemplateID == "" || v.Version <= 0 {
		return ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.prompts[v.TemplateID] {
		if existing.Version == v.Version {
			return ErrDuplicateReport
		}
	}
	list := append(r.prompts[v.TemplateID], v)
	sort.SliceStable(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	r.prompts[v.TemplateID] = list
	return nil
}

func (r *MemoryRepository) PromptVersions(templateID string) []PromptVersion {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]PromptVersion(nil), r.prompts[templateID]...)
}

func (r *MemoryRepository) LatestPromptVersion(templateID string) (PromptVersion, bool) {
	vs := r.PromptVersions(templateID)
	if len(vs) == 0 {
		return PromptVersion{}, false
	}
	return vs[len(vs)-1], true
}

func (r *MemoryRepository) PromptTemplates() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.prompts))
	for id := range r.prompts {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (r *MemoryRepository) SaveApproval(a Approval) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.approvals = append(r.approvals, a)
	return nil
}

func (r *MemoryRepository) Approvals() []Approval {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Approval(nil), r.approvals...)
}
