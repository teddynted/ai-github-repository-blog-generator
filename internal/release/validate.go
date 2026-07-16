package release

import (
	"context"
	"fmt"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/changelog"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
)

// Validate runs every pre-release check and returns a severity Report. In
// dryRun the GitHub authentication/connectivity checks are non-blocking
// warnings (a plan can be validated locally without credentials); for a real
// release they are errors. Repository/version/tag/commit checks are always
// release-blocking errors. Adding a rule is a new `add` call — existing flows
// are untouched.
func (s *Service) Validate(ctx context.Context, plan Plan, changelogContent string, dryRun bool) Report {
	var r Report

	// --- Semantic Version (always required) ---
	if plan.PrevTag != "" && !plan.PrevVersion.LessThan(plan.NextVersion) {
		r.add("Semantic Version valid", SeverityError,
			fmt.Sprintf("%s does not increase over %s (no downgrade or repeat)", plan.NextVersion, plan.PrevVersion))
	} else if plan.PrevTag != "" && plan.Bump == conventional.BumpNone {
		r.add("Semantic Version valid", SeverityError,
			fmt.Sprintf("no release-worthy commits since %s; pass an explicit bump to override", plan.PrevTag))
	} else {
		r.add("Semantic Version valid", SeverityOK, plan.NextVersion.String())
	}

	// --- Conventional Commits (always required; root commit already excluded) ---
	if n := len(plan.NonConventional); n > 0 {
		r.add("Conventional Commits validated", SeverityError,
			fmt.Sprintf("%d non-conventional commit(s) since %s", n, tagOrStart(plan.PrevTag)),
			"e.g. "+quote(plan.NonConventional[0]))
	} else {
		r.add("Conventional Commits validated", SeverityOK, fmt.Sprintf("%d commit(s)", len(plan.Commits)))
	}

	// --- Working tree (always required) ---
	if clean, err := s.Git.IsClean(ctx); err != nil {
		r.add("Repository clean", SeverityError, "could not check working tree: "+err.Error())
	} else if !clean {
		r.add("Repository clean", SeverityError, "working tree is not clean (commit or stash changes first)")
	} else {
		r.add("Repository clean", SeverityOK, "")
	}

	// --- Release branch (always required) ---
	if br, err := s.Git.CurrentBranch(ctx); err != nil {
		r.add("Release branch verified", SeverityError, "could not read branch: "+err.Error())
	} else if br != s.Config.ReleaseBranch {
		r.add("Current branch", SeverityError, "", "Current : "+br, "Expected: "+s.Config.ReleaseBranch)
	} else {
		r.add("Release branch verified", SeverityOK, br)
	}

	// --- Tag does not exist (always required) ---
	if exists, err := s.Git.TagExists(ctx, plan.Tag); err != nil {
		r.add("Tag does not exist", SeverityError, "could not check tag "+plan.Tag+": "+err.Error())
	} else if exists {
		r.add("Tag does not exist", SeverityError, "tag "+plan.Tag+" already exists")
	} else {
		r.add("Tag does not exist", SeverityOK, plan.Tag)
	}

	// --- CHANGELOG not already documenting this version (guard against repeats) ---
	if changelog.Contains(changelogContent, plan.NextVersion.String()) {
		r.add("CHANGELOG updated", SeverityError, "CHANGELOG already documents "+plan.NextVersion.String())
	}

	// --- Remote sync (always required) ---
	if ok, err := s.Git.UpstreamInSync(ctx); err != nil {
		r.add("Synchronized with remote", SeverityError, "could not check remote: "+err.Error())
	} else if !ok {
		r.add("Synchronized with remote", SeverityError, "local branch is not in sync with the remote (pull/push first)")
	} else {
		r.add("Synchronized with remote", SeverityOK, "")
	}

	// --- GitHub authentication (warning in dry-run, error for a real release) ---
	if s.GitHub.Authenticated(ctx) {
		r.add("GitHub authentication", SeverityOK, "")
	} else if dryRun {
		r.add("GitHub authentication not configured", SeverityWarning, "GITHUB_TOKEN or GH_TOKEN not found.")
	} else {
		r.add("GitHub authentication", SeverityError, "no GitHub authentication available (set GITHUB_TOKEN or GH_TOKEN)")
	}

	// --- GitHub connectivity + duplicate-release (skipped/warned in dry-run) ---
	if dryRun {
		r.add("GitHub connectivity skipped (dry-run)", SeverityWarning, "")
	} else if !s.GitHub.Authenticated(ctx) {
		// Auth already failed above; don't attempt a call.
		r.add("GitHub connectivity", SeverityError, "cannot reach GitHub without authentication")
	} else if exists, err := s.GitHub.ReleaseExists(ctx, plan.Tag); err != nil {
		r.add("GitHub connectivity", SeverityError, "GitHub request failed: "+err.Error())
	} else if exists {
		r.add("GitHub release does not exist", SeverityError, "a release for "+plan.Tag+" already exists")
	} else {
		r.add("GitHub connectivity", SeverityOK, "")
	}

	return r
}

func tagOrStart(tag string) string {
	if tag == "" {
		return "the start of history"
	}
	return tag
}

func quote(s string) string { return "\"" + s + "\"" }
