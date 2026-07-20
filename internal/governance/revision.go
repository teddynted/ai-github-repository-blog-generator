package governance

import "time"

// RevisionManager records revision cycles, preserving history and reviewer
// comments so revisions can be compared and audited.
type RevisionManager struct {
	Now Clock
}

// Record creates a revision from an old→new body, capturing the feedback that
// prompted it and any preserved reviewer comments.
func (m RevisionManager) Record(number int, oldBody, newBody string, feedback, comments []string) Revision {
	now := time.Now
	if m.Now != nil {
		now = m.Now
	}
	return Revision{
		Number:           number,
		PreviousBody:     oldBody,
		NewBody:          newBody,
		Feedback:         dedupe(feedback),
		ReviewerComments: dedupe(comments),
		ChangedChars:     charDelta(oldBody, newBody),
		CreatedAt:        now(),
	}
}

// charDelta is a simple change magnitude (absolute length difference plus a
// coarse content-change signal) for the audit trail.
func charDelta(oldBody, newBody string) int {
	d := len(newBody) - len(oldBody)
	if d < 0 {
		d = -d
	}
	if collapse(oldBody) != collapse(newBody) && d == 0 {
		return 1 // same length but changed
	}
	return d
}

// feedbackFrom builds revision feedback from a quality report (the actionable
// suggestions plus any validation errors and ungrounded claims).
func feedbackFrom(rep *QualityReport) []string {
	if rep == nil {
		return nil
	}
	var out []string
	out = append(out, rep.Suggestions...)
	if !rep.Grounding.Verified {
		out = append(out, "Ground or remove the flagged unverified claims before resubmitting.")
	}
	return topStrings(dedupe(out), 12)
}
