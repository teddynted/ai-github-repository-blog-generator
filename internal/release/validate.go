package release

import (
	"context"
	"fmt"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/changelog"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
)

// Validate runs every pre-release check and returns all failures (empty slice =
// OK). It never returns after the first failure so the caller can report the
// full picture. changelogContent is the current CHANGELOG.md (may be "").
func (s *Service) Validate(ctx context.Context, plan Plan, changelogContent string) []error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	// --- Version / tag / commit checks (from the plan) ---
	if plan.PrevTag != "" && !plan.PrevVersion.LessThan(plan.NextVersion) {
		fail("version %s does not increase over %s (downgrades and repeats are not allowed)",
			plan.NextVersion, plan.PrevVersion)
	}
	if plan.PrevTag != "" && plan.Bump == conventional.BumpNone {
		fail("no release-worthy commits since %s (need feat/fix/breaking); pass an explicit bump to override", plan.PrevTag)
	}
	if n := len(plan.NonConventional); n > 0 {
		fail("%d non-conventional commit(s) since %s, e.g. %q", n, tagOrStart(plan.PrevTag), plan.NonConventional[0])
	}
	if exists, err := s.Git.TagExists(ctx, plan.Tag); err != nil {
		fail("check tag %s: %v", plan.Tag, err)
	} else if exists {
		fail("tag %s already exists", plan.Tag)
	}
	if changelog.Contains(changelogContent, plan.NextVersion.String()) {
		fail("CHANGELOG already documents %s", plan.NextVersion)
	}

	// --- Repository-state checks ---
	if clean, err := s.Git.IsClean(ctx); err != nil {
		fail("check working tree: %v", err)
	} else if !clean {
		fail("working tree is not clean (commit or stash changes first)")
	}
	if br, err := s.Git.CurrentBranch(ctx); err != nil {
		fail("check branch: %v", err)
	} else if br != s.Config.ReleaseBranch {
		fail("on branch %q; releases must be cut from %q", br, s.Config.ReleaseBranch)
	}
	if ok, err := s.Git.UpstreamInSync(ctx); err != nil {
		fail("check remote sync: %v", err)
	} else if !ok {
		fail("local branch is not in sync with the remote (pull/push first)")
	}
	if !s.GitHub.Authenticated(ctx) {
		fail("no GitHub authentication available (set GITHUB_TOKEN or GH_TOKEN)")
	}
	return errs
}

func tagOrStart(tag string) string {
	if tag == "" {
		return "the start of history"
	}
	return tag
}
