package governance

// Config holds the configurable governance rules. Zero value is not useful —
// use DefaultConfig() and override.
type Config struct {
	// MinOverallScore is the overall score at/above which content is eligible to
	// move to approval (subject to validation + grounding gates).
	MinOverallScore int
	// RejectBelow is the overall score below which content is rejected outright.
	RejectBelow int
	// MinDimensionScore is the floor any single quality dimension must meet.
	MinDimensionScore int
	// MinGroundingRatio is the fraction of checked claims that must be grounded
	// (0.0–1.0) for the grounding gate to pass.
	MinGroundingRatio float64
	// RequiredApprovals is how many approving decisions are required.
	RequiredApprovals int
	// RequiredRoles are roles that must each approve before publication.
	RequiredRoles []Role
	// RevisionLimit caps revision cycles; <= 0 means unlimited.
	RevisionLimit int
	// BlockOnValidationErrors, when true, forces Needs Revision on any
	// error-severity validation issue regardless of score.
	BlockOnValidationErrors bool
}

// DefaultConfig returns production-sensible defaults.
func DefaultConfig() Config {
	return Config{
		MinOverallScore:         80,
		RejectBelow:             50,
		MinDimensionScore:       60,
		MinGroundingRatio:       0.8,
		RequiredApprovals:       1,
		RequiredRoles:           []Role{RoleReviewer},
		RevisionLimit:           0, // unlimited
		BlockOnValidationErrors: true,
	}
}

// rolePermissions maps a role to the actions it may perform. Configurable by
// swapping this map in a custom build; kept simple and explicit.
var rolePermissions = map[Role]map[string]bool{
	RoleAuthor:            {"revise": true},
	RoleReviewer:          {"approve": true, "revise": true, "reject": true},
	RoleTechnicalReviewer: {"approve": true, "revise": true, "reject": true},
	RolePublisher:         {"approve": true, "publish": true},
	RoleAdministrator:     {"approve": true, "revise": true, "reject": true, "publish": true, "archive": true},
}

// can reports whether a role may perform an action.
func (r Role) can(action string) bool {
	if perms, ok := rolePermissions[r]; ok {
		return perms[action]
	}
	return false
}
