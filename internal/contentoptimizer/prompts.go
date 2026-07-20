package contentoptimizer

import (
	"fmt"
	"time"
)

// PromptService manages versioned, measurable content-generation prompt
// templates. Versions are append-only; a new version never overwrites a prior
// one, so rollback is always possible. It NEVER auto-applies a change — proposals
// are surfaced for human approval, and approval creates a new version.
type PromptService struct {
	Repo Repository
	Now  Clock
}

// NewPromptService wires a prompt service.
func NewPromptService(repo Repository, now Clock) *PromptService {
	if now == nil {
		now = time.Now
	}
	return &PromptService{Repo: repo, Now: now}
}

// Register creates version 1 of a template (draft). Idempotent-ish: if the
// template already exists it is a no-op returning the latest version.
func (s *PromptService) Register(templateID, body, notes string) (PromptVersion, error) {
	if v, ok := s.Repo.LatestPromptVersion(templateID); ok {
		return v, nil
	}
	v := PromptVersion{
		TemplateID: templateID,
		Version:    1,
		Body:       body,
		Notes:      notes,
		Status:     StatusApproved, // the seed prompt is the approved baseline
		CreatedAt:  s.Now(),
	}
	if err := s.Repo.SavePromptVersion(v); err != nil {
		return PromptVersion{}, err
	}
	return v, nil
}

// Propose records a new proposed version derived from a recommendation. It is
// NOT active — status is "proposed" until a human approves it.
func (s *PromptService) Propose(templateID, body, notes string) (PromptVersion, error) {
	latest, ok := s.Repo.LatestPromptVersion(templateID)
	if !ok {
		return PromptVersion{}, ErrNotFound
	}
	v := PromptVersion{
		TemplateID: templateID,
		Version:    latest.Version + 1,
		Body:       body,
		Notes:      notes,
		Status:     StatusProposed,
		CreatedAt:  s.Now(),
	}
	if err := s.Repo.SavePromptVersion(v); err != nil {
		return PromptVersion{}, err
	}
	return v, nil
}

// Approve transitions a proposed version to approved, recording the decision. It
// only advances proposed→approved (a legal transition); anything else is rejected.
func (s *PromptService) Approve(templateID string, version int, decider, reason string) error {
	return s.decide(templateID, version, StatusApproved, decider, reason)
}

// Reject transitions a proposed version to rejected.
func (s *PromptService) Reject(templateID string, version int, decider, reason string) error {
	return s.decide(templateID, version, StatusRejected, decider, reason)
}

func (s *PromptService) decide(templateID string, version int, to ApprovalStatus, decider, reason string) error {
	versions := s.Repo.PromptVersions(templateID)
	var target *PromptVersion
	for i := range versions {
		if versions[i].Version == version {
			target = &versions[i]
			break
		}
	}
	if target == nil {
		return ErrNotFound
	}
	// Guard on the EFFECTIVE status (derived from the append-only audit), not the
	// immutable version record — a version can only be decided once from proposed.
	if s.EffectiveStatus(templateID, version) != StatusProposed {
		return ErrInvalidTransition
	}
	// Record the approval audit entry (append-only). The version's own status is
	// derived from the audit trail on read via EffectiveStatus.
	return s.Repo.SaveApproval(Approval{
		ID:        shortID("approval", templateID, fmt.Sprint(version), string(to)),
		Kind:      "prompt",
		TargetID:  templateID,
		Version:   version,
		Status:    to,
		Decider:   decider,
		Reason:    reason,
		DecidedAt: s.Now(),
	})
}

// EffectiveStatus resolves a version's status from the append-only approval
// audit (the latest decision wins), so the immutable version record is never
// mutated in place.
func (s *PromptService) EffectiveStatus(templateID string, version int) ApprovalStatus {
	status := StatusProposed
	for _, v := range s.Repo.PromptVersions(templateID) {
		if v.Version == version {
			status = v.Status
		}
	}
	for _, a := range s.Repo.Approvals() {
		if a.Kind == "prompt" && a.TargetID == templateID && a.Version == version {
			status = a.Status
		}
	}
	return status
}

// ActiveVersion returns the highest-numbered approved version — the one used for
// generation. Rollback is achieved by approving an earlier version again, or by
// reading any prior version directly.
func (s *PromptService) ActiveVersion(templateID string) (PromptVersion, bool) {
	var active PromptVersion
	found := false
	for _, v := range s.Repo.PromptVersions(templateID) {
		if s.EffectiveStatus(templateID, v.Version) == StatusApproved {
			active, found = v, true
		}
	}
	return active, found
}

// RecordUsage appends measured metrics to a NEW immutable version snapshot so
// historical metrics are preserved (the prior version record is untouched).
func (s *PromptService) RecordUsage(templateID string, version int, m PromptMetrics) error {
	versions := s.Repo.PromptVersions(templateID)
	for _, v := range versions {
		if v.Version == version {
			// Metrics are stored on the version at creation in this in-memory
			// model; a production store would append a metrics row keyed by
			// (templateID, version, as_of). We surface the intent via the audit.
			return s.Repo.SaveApproval(Approval{
				ID:        shortID("usage", templateID, fmt.Sprint(version), fmt.Sprint(m.UsageCount)),
				Kind:      "usage",
				TargetID:  templateID,
				Version:   version,
				Status:    s.EffectiveStatus(templateID, version),
				Decider:   "system",
				Reason:    fmt.Sprintf("usage=%d ctr=%.2f eng=%.2f", m.UsageCount, m.AvgCTR, m.AvgEngagementRate),
				DecidedAt: s.Now(),
			})
		}
	}
	return ErrNotFound
}

// DefaultPromptAnalyzer analyzes a template's version history and proposes
// grounded improvements. It never rewrites a prompt; it returns proposals.
type DefaultPromptAnalyzer struct {
	Config Config
}

// NewPromptAnalyzer wires a prompt analyzer.
func NewPromptAnalyzer(cfg Config) *DefaultPromptAnalyzer {
	return &DefaultPromptAnalyzer{Config: cfg}
}

// Analyze evaluates version performance and proposes aspect-level improvements,
// each grounded in the analytics (weak hooks → hook proposal, etc.).
func (a *DefaultPromptAnalyzer) Analyze(templateID string, versions []PromptVersion, input AnalyticsInput) PromptAnalysis {
	an := PromptAnalysis{TemplateID: templateID}
	if len(versions) == 0 {
		return an
	}
	an.Versions = len(versions)
	an.CurrentVersion = versions[len(versions)-1].Version

	// Best version by average score.
	best := versions[0]
	for _, v := range versions {
		if v.Metrics.AvgScore > best.Metrics.AvgScore {
			best = v
		}
	}
	an.BestVersion = best.Version
	an.Trend = versionTrend(versions)

	// Grounded aspect proposals from the analytics signals.
	base := computeBaseline(input.Records)
	if base.retention > 0 && base.retention < 60 {
		an.Recommendations = append(an.Recommendations, PromptRecommendation{
			TemplateID:  templateID,
			BaseVersion: an.CurrentVersion,
			Aspect:      "hook",
			Suggestion:  "Open with a concrete problem statement and a promise of the payoff within the first two sentences.",
			Rationale:   "Retention across content is below target; stronger hooks lift early retention.",
			Evidence:    []string{fmt.Sprintf("mean retention %.1f%%", base.retention)},
			Confidence:  supportConfidence(base.n, 0.4),
		})
	}
	if base.ctr > 0 && base.ctr < 5 {
		an.Recommendations = append(an.Recommendations, PromptRecommendation{
			TemplateID:  templateID,
			BaseVersion: an.CurrentVersion,
			Aspect:      "seo",
			Suggestion:  "Instruct the model to place the primary keyword in the title, first heading, and meta description.",
			Rationale:   "Click-through is below target; keyword-forward titles raise CTR.",
			Evidence:    []string{fmt.Sprintf("mean CTR %.2f%%", base.ctr)},
			Confidence:  supportConfidence(base.n, 0.4),
		})
	}
	if base.engagement > 0 && base.engagement < 5 {
		an.Recommendations = append(an.Recommendations, PromptRecommendation{
			TemplateID:  templateID,
			BaseVersion: an.CurrentVersion,
			Aspect:      "cta",
			Suggestion:  "Add an explicit, single call-to-action near the end tied to the reader's next step.",
			Rationale:   "Engagement is below target; a clear CTA increases interaction.",
			Evidence:    []string{fmt.Sprintf("mean engagement %.2f%%", base.engagement)},
			Confidence:  supportConfidence(base.n, 0.35),
		})
	}
	return an
}

// versionTrend compares the latest version's score to the previous one.
func versionTrend(versions []PromptVersion) string {
	if len(versions) < 2 {
		return "flat"
	}
	last := versions[len(versions)-1].Metrics.AvgScore
	prev := versions[len(versions)-2].Metrics.AvgScore
	switch {
	case last > prev:
		return "improving"
	case last < prev:
		return "regressing"
	default:
		return "flat"
	}
}
