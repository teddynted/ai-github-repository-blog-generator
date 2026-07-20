// Package governance is the content governance layer (Milestone 14). It is the
// review-and-approval workflow that every AI-generated asset must pass before it
// is published: deterministic validation, AI-assisted quality review grounded in
// the Release Context, quality scoring, a revision loop, human approval gates,
// publication-readiness checks, and a full audit trail.
//
// It follows Clean Architecture: the domain (this file + status.go) is pure; the
// engines (validation, grounding, scoring, review, approval, revision,
// readiness) depend only on the domain; and the AI reviewer and storage are
// behind ports (ports.go) so Amazon Bedrock, Ollama, in-memory, or SQL back ends
// are swappable via dependency inversion. Validation and scoring are
// deterministic and authoritative; the LLM only adds qualitative review notes,
// grounded in the Release Context — it never fabricates missing information, and
// nothing is ever approved on hallucinated content.
package governance

import "time"

// SchemaVersion is the governance document version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// ContentType identifies the kind of generated asset under review.
type ContentType string

const (
	TypeBlog                ContentType = "Technical Blog"
	TypeStoryboard          ContentType = "Storyboard"
	TypeVoiceOver           ContentType = "Voice-over Script"
	TypeYouTubeScript       ContentType = "YouTube Video Script"
	TypeYouTubeShorts       ContentType = "YouTube Shorts"
	TypeTikTok              ContentType = "TikTok Shorts"
	TypeLinkedIn            ContentType = "LinkedIn Post"
	TypeXThread             ContentType = "X Thread"
	TypeArchitectureDiagram ContentType = "AWS Architecture Diagram"
	TypeSEO                 ContentType = "SEO Metadata"
	TypeThumbnailPrompt     ContentType = "Thumbnail Prompt"
	TypeVisualAsset         ContentType = "Visual Asset"
)

// Grounding is the source of truth every claim in the content must trace back
// to. Terms are lowercased grounded tokens drawn from the Release Context, git,
// CHANGELOG, docs, architecture, and CloudFormation.
type Grounding struct {
	Repository string   `json:"repository"`
	Release    string   `json:"release"`
	Terms      []string `json:"terms,omitempty"`
}

// Content is the asset submitted to the workflow.
type Content struct {
	ID        string            `json:"id"`
	Type      ContentType       `json:"type"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Grounding Grounding         `json:"grounding"`
	Author    string            `json:"author,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

// Severity classifies a validation issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// ValidationIssue is one deterministic validation finding.
type ValidationIssue struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Field    string   `json:"field,omitempty"`
}

// ValidationReport is the deterministic validation result.
type ValidationReport struct {
	Issues   []ValidationIssue `json:"issues,omitempty"`
	Errors   int               `json:"errors"`
	Warnings int               `json:"warnings"`
	Passed   bool              `json:"passed"` // no error-severity issues
}

// Scores are the quality dimensions (0–100).
type Scores struct {
	TechnicalAccuracy int `json:"technicalAccuracy"`
	Readability       int `json:"readability"`
	SEO               int `json:"seo"`
	Architecture      int `json:"architecture"`
	Completeness      int `json:"completeness"`
	Grammar           int `json:"grammar"`
	Consistency       int `json:"consistency"`
	EducationalValue  int `json:"educationalValue"`
	Overall           int `json:"overall"`
}

// Decision is the outcome of a quality review.
type Decision string

const (
	DecisionApprove       Decision = "Approve"
	DecisionNeedsRevision Decision = "Needs Revision"
	DecisionReject        Decision = "Reject"
)

// GroundingResult is the outcome of grounding verification.
type GroundingResult struct {
	Verified         bool     `json:"verified"`
	CheckedClaims    int      `json:"checkedClaims"`
	GroundedClaims   int      `json:"groundedClaims"`
	UnverifiedClaims []string `json:"unverifiedClaims,omitempty"`
}

// QualityReport is the combined deterministic + AI review of a piece of content.
type QualityReport struct {
	Reviewer    string           `json:"reviewer"` // "deterministic" | model id | human name
	Scores      Scores           `json:"scores"`
	Decision    Decision         `json:"decision"`
	Confidence  int              `json:"confidence"` // 0–100
	Summary     string           `json:"summary"`
	Strengths   []string         `json:"strengths,omitempty"`
	Weaknesses  []string         `json:"weaknesses,omitempty"`
	Suggestions []string         `json:"suggestions,omitempty"`
	Validation  ValidationReport `json:"validation"`
	Grounding   GroundingResult  `json:"grounding"`
	CreatedAt   time.Time        `json:"createdAt"`
}

// Role is an approval role with configurable permissions.
type Role string

const (
	RoleAuthor            Role = "Author"
	RoleReviewer          Role = "Reviewer"
	RoleTechnicalReviewer Role = "Technical Reviewer"
	RolePublisher         Role = "Publisher"
	RoleAdministrator     Role = "Administrator"
)

// Reviewer is a person (or system) acting in the workflow.
type Reviewer struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
}

// ApprovalDecision is a human (or automated) approval action.
type ApprovalDecision struct {
	Reviewer  Reviewer  `json:"reviewer"`
	Approved  bool      `json:"approved"`
	Comment   string    `json:"comment,omitempty"`
	DecidedAt time.Time `json:"decidedAt"`
}

// Revision records one revision cycle of the content body.
type Revision struct {
	Number           int       `json:"number"`
	PreviousBody     string    `json:"-"` // kept for diffing; omitted from JSON to keep it small
	NewBody          string    `json:"-"`
	Feedback         []string  `json:"feedback,omitempty"`
	ReviewerComments []string  `json:"reviewerComments,omitempty"`
	ChangedChars     int       `json:"changedChars"`
	CreatedAt        time.Time `json:"createdAt"`
}

// AuditEntry is one immutable record in the audit trail.
type AuditEntry struct {
	Timestamp  time.Time      `json:"timestamp"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	FromStatus WorkflowStatus `json:"fromStatus,omitempty"`
	ToStatus   WorkflowStatus `json:"toStatus,omitempty"`
	Detail     string         `json:"detail,omitempty"`
}

// ReadinessCheck is one publication-readiness gate.
type ReadinessCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// PublicationReadiness is the aggregate readiness result.
type PublicationReadiness struct {
	Ready  bool             `json:"ready"`
	Checks []ReadinessCheck `json:"checks"`
}

// WorkflowState is the aggregate root: the content plus its full lifecycle.
type WorkflowState struct {
	Content      Content               `json:"content"`
	Status       WorkflowStatus        `json:"status"`
	Reviews      []QualityReport       `json:"reviews,omitempty"`
	Approvals    []ApprovalDecision    `json:"approvals,omitempty"`
	Revisions    []Revision            `json:"revisions,omitempty"`
	Audit        []AuditEntry          `json:"audit,omitempty"`
	LatestReport *QualityReport        `json:"latestReport,omitempty"`
	Readiness    *PublicationReadiness `json:"readiness,omitempty"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
}

// approvedRoles returns the set of roles that have an approving decision.
func (s *WorkflowState) approvedRoles() map[Role]bool {
	out := map[Role]bool{}
	for _, a := range s.Approvals {
		if a.Approved {
			out[a.Reviewer.Role] = true
		}
	}
	return out
}
