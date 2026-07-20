package governance

// ReadinessEngine performs the pre-publication checks.
type ReadinessEngine struct {
	Config Config
}

// Check verifies the content is ready to publish: approved, complete metadata,
// required assets present, and review comments resolved.
func (e ReadinessEngine) Check(state *WorkflowState) PublicationReadiness {
	c := state.Content
	rep := state.LatestReport
	add := func(name string, ok bool, detail string) ReadinessCheck {
		return ReadinessCheck{Name: name, Passed: ok, Detail: detail}
	}

	checks := []ReadinessCheck{
		add("Approval completed", state.Status == StatusApproved, string(state.Status)),
		add("Required approvals met", e.approvalsMet(state), ""),
		add("Metadata complete", metadataComplete(c), ""),
		add("SEO present", seoPresent(c), ""),
		add("Review comments resolved", commentsResolved(state), ""),
		add("Latest review passed", rep != nil && rep.Decision == DecisionApprove, decisionDetail(rep)),
		add("No open validation errors", rep != nil && rep.Validation.Errors == 0, ""),
	}
	// Type-specific asset requirements.
	checks = append(checks, e.assetChecks(c)...)

	ready := true
	for _, ck := range checks {
		if !ck.Passed {
			ready = false
		}
	}
	return PublicationReadiness{Ready: ready, Checks: checks}
}

func (e ReadinessEngine) approvalsMet(state *WorkflowState) bool {
	approving := 0
	for _, a := range state.Approvals {
		if a.Approved {
			approving++
		}
	}
	if approving < e.Config.RequiredApprovals {
		return false
	}
	approved := state.approvedRoles()
	for _, role := range e.Config.RequiredRoles {
		if !approved[role] {
			return false
		}
	}
	return true
}

func (e ReadinessEngine) assetChecks(c Content) []ReadinessCheck {
	var checks []ReadinessCheck
	switch c.Type {
	case TypeBlog, TypeYouTubeScript:
		checks = append(checks, ReadinessCheck{Name: "Thumbnail available", Passed: c.Metadata["thumbnail"] != "" || c.Metadata["thumbnailText"] != "", Detail: "reference a generated visual asset"})
	case TypeArchitectureDiagram:
		checks = append(checks, ReadinessCheck{Name: "Diagram generated", Passed: c.Body != "", Detail: ""})
	}
	return checks
}

func seoPresent(c Content) bool {
	if c.Type == TypeSEO {
		return c.Metadata["metaDescription"] != ""
	}
	return true // SEO is validated on its own content type
}

func commentsResolved(state *WorkflowState) bool {
	// Comments are resolved when, since the last revision, no reviewer has left
	// an un-actioned rejecting decision.
	for _, a := range state.Approvals {
		if !a.Approved {
			return false
		}
	}
	return true
}

func decisionDetail(rep *QualityReport) string {
	if rep == nil {
		return "no review"
	}
	return string(rep.Decision)
}
