package governance

// WorkflowStatus is a state in the review-and-approval lifecycle.
type WorkflowStatus string

const (
	StatusDraft           WorkflowStatus = "Draft"
	StatusGenerated       WorkflowStatus = "Generated"
	StatusAIReviewed      WorkflowStatus = "AI Reviewed"
	StatusNeedsRevision   WorkflowStatus = "Needs Revision"
	StatusUpdated         WorkflowStatus = "Updated"
	StatusPendingApproval WorkflowStatus = "Pending Approval"
	StatusApproved        WorkflowStatus = "Approved"
	StatusPublished       WorkflowStatus = "Published"
	StatusArchived        WorkflowStatus = "Archived"
	StatusRejected        WorkflowStatus = "Rejected"
)

// transitions defines the legal state machine. A transition not listed here is
// rejected by the workflow engine, so the lifecycle can never skip a gate.
var transitions = map[WorkflowStatus][]WorkflowStatus{
	StatusDraft:           {StatusGenerated},
	StatusGenerated:       {StatusAIReviewed},
	StatusAIReviewed:      {StatusNeedsRevision, StatusPendingApproval, StatusRejected},
	StatusNeedsRevision:   {StatusUpdated, StatusArchived},
	StatusUpdated:         {StatusAIReviewed},
	StatusPendingApproval: {StatusApproved, StatusNeedsRevision, StatusRejected},
	StatusApproved:        {StatusPublished, StatusNeedsRevision},
	StatusPublished:       {StatusArchived},
	StatusRejected:        {StatusNeedsRevision, StatusArchived},
	StatusArchived:        {},
}

// canTransition reports whether from → to is a legal transition.
func canTransition(from, to WorkflowStatus) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}
