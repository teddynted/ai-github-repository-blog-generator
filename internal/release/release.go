package release

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/changelog"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/semver"
)

// Git is the port for the local git operations the workflow needs. It is
// satisfied by release.ExecGit (shells out to `git`).
type Git interface {
	CurrentBranch(ctx context.Context) (string, error)
	IsClean(ctx context.Context) (bool, error)
	// LatestSemverTag returns the highest semver tag (with prefix), or "" if none.
	LatestSemverTag(ctx context.Context, prefix string) (string, error)
	// CommitSubjectsSince returns commit subjects (first lines) since tag,
	// newest first; tag "" means the entire history.
	CommitSubjectsSince(ctx context.Context, tag string) ([]string, error)
	// InitialCommit returns the SHA of the repository's root commit, used to
	// exclude a non-conventional initial commit ("first commit") from
	// validation. Returns "" when it cannot be determined.
	InitialCommit(ctx context.Context) (string, error)
	TagExists(ctx context.Context, tag string) (bool, error)
	CreateAnnotatedTag(ctx context.Context, tag, message string) error
	PushTag(ctx context.Context, tag string) error
	// UpstreamInSync reports whether the local branch is in sync with its remote
	// tracking branch (no ahead/behind divergence).
	UpstreamInSync(ctx context.Context) (bool, error)
	// ContributorsSince returns author names for commits since tag.
	ContributorsSince(ctx context.Context, tag string) ([]string, error)
}

// GitHubReleases is the port for GitHub Release operations.
type GitHubReleases interface {
	// Authenticated reports whether a usable token/credential is configured.
	Authenticated(ctx context.Context) bool
	ReleaseExists(ctx context.Context, tag string) (bool, error)
	CreateRelease(ctx context.Context, tag, name, body string, prerelease bool) (htmlURL string, err error)
}

// Service runs the release workflow.
type Service struct {
	Git    Git
	GitHub GitHubReleases
	Config Config
	Now    func() time.Time
	Log    *slog.Logger
}

// Plan is the deterministic description of the release to be made.
type Plan struct {
	PrevVersion semver.Version
	NextVersion semver.Version
	Bump        conventional.Bump
	Tag         string
	PrevTag     string
	Date        string
	Commits     []conventional.Commit
	// NonConventional holds commit subjects that failed Conventional Commit
	// parsing (surfaced by validation, ignored for bump derivation).
	NonConventional  []string
	Notes            string
	ChangelogSection string
	Contributors     []string
	// Analysed is the number of commits considered (excludes the root commit
	// when there is no prior tag).
	Analysed int
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) log(msg string, args ...any) {
	if s.Log != nil {
		s.Log.Info(msg, args...)
	}
}

// DeterminePlan computes the next version and the release artifacts. forceBump
// (non-nil) overrides automatic derivation; pre applies a pre-release identifier
// to the computed version (e.g. "rc.1"). It never mutates the repository.
func (s *Service) DeterminePlan(ctx context.Context, forceBump *conventional.Bump, pre string) (Plan, error) {
	prevTag, err := s.Git.LatestSemverTag(ctx, s.Config.TagPrefix)
	if err != nil {
		return Plan{}, fmt.Errorf("find latest tag: %w", err)
	}

	prev := semver.Version{}
	if prevTag == "" {
		prev, err = semver.Parse(s.Config.InitialVersion)
		if err != nil {
			return Plan{}, fmt.Errorf("invalid initial_version %q: %w", s.Config.InitialVersion, err)
		}
	} else {
		if prev, err = semver.Parse(prevTag); err != nil {
			return Plan{}, fmt.Errorf("latest tag %q is not semver: %w", prevTag, err)
		}
	}

	// The range to analyse. With no prior tag, exclude the repository's root
	// commit so a non-conventional initial commit ("first commit") never fails
	// validation and history need not be rewritten.
	since := prevTag
	if since == "" {
		if root, rerr := s.Git.InitialCommit(ctx); rerr == nil && root != "" {
			since = root
		}
	}
	subjects, err := s.Git.CommitSubjectsSince(ctx, since)
	if err != nil {
		return Plan{}, fmt.Errorf("read commits: %w", err)
	}
	var commits []conventional.Commit
	var bad []string
	for _, subj := range subjects {
		c, perr := conventional.Parse(subj, s.Config.Commit)
		if perr != nil {
			bad = append(bad, subj)
			continue
		}
		commits = append(commits, c)
	}

	bump := conventional.DetermineBump(commits, s.Config.Commit)
	if forceBump != nil {
		bump = *forceBump
	}

	var next semver.Version
	if prevTag == "" {
		next = prev // first release uses initial_version as-is
	} else {
		switch bump {
		case conventional.BumpMajor:
			next = prev.IncMajor()
		case conventional.BumpMinor:
			next = prev.IncMinor()
		case conventional.BumpPatch:
			next = prev.IncPatch()
		default:
			next = prev // no bump
		}
	}
	if pre != "" {
		next = next.WithPreRelease(pre)
	}

	date := s.now().UTC().Format("2006-01-02")
	contributors, _ := s.Git.ContributorsSince(ctx, since)
	tag := s.Config.TagPrefix + next.String()

	plan := Plan{
		PrevVersion:      prev,
		NextVersion:      next,
		Bump:             bump,
		Tag:              tag,
		PrevTag:          prevTag,
		Date:             date,
		Commits:          commits,
		NonConventional:  bad,
		Analysed:         len(subjects),
		Contributors:     contributors,
		ChangelogSection: changelog.Render(next.String(), date, commits, s.Config.ChangelogCategories),
		Notes:            Notes(next.String(), date, "./CHANGELOG.md#"+anchor(next.String()), commits, contributors),
	}
	s.log("release plan determined",
		slog.String("prev", prevTag), slog.String("next", tag),
		slog.String("bump", bump.String()), slog.Int("commits", len(commits)))
	return plan, nil
}

// Summary is returned after a successful (or dry-run) release.
type Summary struct {
	Tag        string
	Version    string
	Bump       string
	ReleaseURL string
	DryRun     bool
	Changelog  string // the updated CHANGELOG.md content
}

// Apply performs the mutating steps: update the changelog content (returned to
// the caller to persist), create + push the annotated tag, and create the
// GitHub Release. In dryRun nothing is executed — only the computed content is
// returned. It is idempotent: an existing tag or release is treated as done.
func (s *Service) Apply(ctx context.Context, plan Plan, changelogOld string, dryRun bool) (Summary, error) {
	sum := Summary{
		Tag:       plan.Tag,
		Version:   plan.NextVersion.String(),
		Bump:      plan.Bump.String(),
		DryRun:    dryRun,
		Changelog: changelog.Update(changelogOld, plan.NextVersion.String(), plan.ChangelogSection),
	}
	if dryRun {
		s.log("dry run: no changes made", slog.String("tag", plan.Tag))
		return sum, nil
	}

	// Tag (idempotent).
	if exists, err := s.Git.TagExists(ctx, plan.Tag); err != nil {
		return sum, fmt.Errorf("check tag: %w", err)
	} else if !exists {
		if err := s.Git.CreateAnnotatedTag(ctx, plan.Tag, fmt.Sprintf("Release %s", plan.Tag)); err != nil {
			return sum, fmt.Errorf("create tag: %w", err)
		}
		s.log("created annotated tag", slog.String("tag", plan.Tag))
	}
	if err := s.Git.PushTag(ctx, plan.Tag); err != nil {
		return sum, fmt.Errorf("push tag: %w", err)
	}

	// GitHub Release (idempotent).
	if exists, err := s.GitHub.ReleaseExists(ctx, plan.Tag); err != nil {
		return sum, fmt.Errorf("check release: %w", err)
	} else if exists {
		s.log("github release already exists", slog.String("tag", plan.Tag))
		return sum, nil
	}
	url, err := s.GitHub.CreateRelease(ctx, plan.Tag, plan.Tag, plan.Notes, plan.NextVersion.PreRelease != "")
	if err != nil {
		return sum, fmt.Errorf("create github release: %w", err)
	}
	sum.ReleaseURL = url
	s.log("github release created", slog.String("tag", plan.Tag), slog.String("url", url))
	return sum, nil
}

// anchor renders a Keep-a-Changelog heading anchor for a version.
func anchor(version string) string {
	out := make([]rune, 0, len(version))
	for _, r := range version {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+32)
		default:
			// '.', '-', '+' are dropped in GitHub-style anchors.
		}
	}
	return string(out)
}
