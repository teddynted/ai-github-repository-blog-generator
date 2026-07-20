package governance

import "errors"

// Sentinel errors let callers (and n8n / GitHub Actions) branch on failure kinds.
var (
	ErrNotFound           = errors.New("governance: workflow state not found")
	ErrInvalidTransition  = errors.New("governance: invalid status transition")
	ErrNotPendingApproval = errors.New("governance: content is not pending approval")
	ErrPermissionDenied   = errors.New("governance: reviewer role may not perform this action")
	ErrRevisionLimit      = errors.New("governance: revision limit exceeded")
	ErrNotApproved        = errors.New("governance: content is not approved")
	ErrNotReadyToPublish  = errors.New("governance: content failed publication-readiness checks")
	ErrDuplicateApproval  = errors.New("governance: reviewer has already recorded a decision for this revision")
	ErrEmptyContent       = errors.New("governance: content body is empty")
	ErrReviewFailed       = errors.New("governance: AI review failed")
	ErrQualityGateFailed  = errors.New("governance: quality gates not satisfied")
)
