package mcp

import (
	"context"
	"fmt"
)

// Permission is a coarse capability a caller holds. Permissions are ordered by
// privilege: a higher permission implies all lower ones (except the orthogonal
// Analytics/Publish which are additive scopes granted explicitly).
type Permission string

const (
	PermNone      Permission = "none"
	PermReadOnly  Permission = "read-only"
	PermReadWrite Permission = "read-write"
	PermAnalytics Permission = "analytics"
	PermPublish   Permission = "publish"
	PermAdmin     Permission = "admin"
)

// rank orders the read ladder (higher = more privilege). Only the ladder
// permissions (none < read-only < read-write) imply one another. Analytics and
// Publish are additive, orthogonal scopes: they satisfy only themselves, and
// Admin, which covers everything.
var rank = map[Permission]int{
	PermNone:      0,
	PermReadOnly:  1,
	PermReadWrite: 2,
	PermAnalytics: 2,
	PermPublish:   3,
	PermAdmin:     4,
}

// ladder reports whether a permission is on the read ladder (participates in
// implication). Analytics/Publish are off-ladder scopes.
func ladder(p Permission) bool {
	return p == PermNone || p == PermReadOnly || p == PermReadWrite
}

// grants reports whether holding `held` satisfies `required`. Admin implies all;
// otherwise an exact scope match, or a higher rung on the read ladder, grants.
func grants(held, required Permission) bool {
	if held == PermAdmin {
		return true
	}
	if held == required {
		return true
	}
	if ladder(held) && ladder(required) {
		return rank[held] >= rank[required]
	}
	return false
}

// PolicyAuthorizer authorizes callers against a per-caller permission grant. It
// is deny-by-default: an unknown caller holds PermNone and can do nothing.
type PolicyAuthorizer struct {
	// Grants maps a caller id to the permission it holds. A "*" entry is the
	// default for callers not listed.
	Grants map[string]Permission
}

// NewPolicyAuthorizer builds an authorizer from a caller→permission map.
func NewPolicyAuthorizer(grants map[string]Permission) *PolicyAuthorizer {
	if grants == nil {
		grants = map[string]Permission{}
	}
	return &PolicyAuthorizer{Grants: grants}
}

// Authorize enforces least privilege: the caller must hold at least `required`.
func (a *PolicyAuthorizer) Authorize(_ context.Context, caller, serverID string, required Permission) error {
	held, ok := a.Grants[caller]
	if !ok {
		held = a.Grants["*"] // default policy
	}
	if held == "" {
		held = PermNone
	}
	if grants(held, required) {
		return nil
	}
	return fmt.Errorf("%w: caller %q holds %q, needs %q for %s", ErrForbidden, caller, held, required, serverID)
}

// AllowAllAuthorizer permits everything — for trusted, single-tenant local use
// only (e.g. the CLI). Never use it for a multi-caller deployment.
type AllowAllAuthorizer struct{}

func (AllowAllAuthorizer) Authorize(context.Context, string, string, Permission) error { return nil }
