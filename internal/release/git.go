package release

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/semver"
)

// ExecGit implements the Git port by shelling out to the `git` binary in the
// repository at Dir (or the current directory when Dir is ""). It is the
// production adapter; tests use fakes.
type ExecGit struct {
	Dir string
}

// NewExecGit builds an ExecGit rooted at dir ("" = current directory).
func NewExecGit(dir string) *ExecGit { return &ExecGit{Dir: dir} }

func (g *ExecGit) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if g.Dir != "" {
		cmd.Dir = g.Dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", &GitError{Args: args, Msg: msg}
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// GitError carries the failing git command and its stderr.
type GitError struct {
	Args []string
	Msg  string
}

func (e *GitError) Error() string {
	return "git " + strings.Join(e.Args, " ") + ": " + e.Msg
}

// CurrentBranch returns the checked-out branch name.
func (g *ExecGit) CurrentBranch(ctx context.Context) (string, error) {
	return g.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

// IsClean reports whether the working tree has no uncommitted changes.
func (g *ExecGit) IsClean(ctx context.Context) (bool, error) {
	out, err := g.run(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

// LatestSemverTag returns the highest semver tag matching prefix, or "".
func (g *ExecGit) LatestSemverTag(ctx context.Context, prefix string) (string, error) {
	out, err := g.run(ctx, "tag", "--list", prefix+"*")
	if err != nil {
		return "", err
	}
	var best string
	var bestV semver.Version
	for _, line := range strings.Split(out, "\n") {
		tag := strings.TrimSpace(line)
		if tag == "" {
			continue
		}
		v, perr := semver.Parse(strings.TrimPrefix(tag, prefix))
		if perr != nil {
			continue // ignore non-semver tags
		}
		if best == "" || bestV.LessThan(v) {
			best, bestV = tag, v
		}
	}
	return best, nil
}

// CommitSubjectsSince returns commit subjects newest-first since tag ("" = all).
func (g *ExecGit) CommitSubjectsSince(ctx context.Context, tag string) ([]string, error) {
	return g.logField(ctx, "%s", tag)
}

// InitialCommit returns the SHA of the repository's root commit (the first
// commit with no parents). If the history has multiple roots, the first is
// returned. Errors (e.g. an empty repo) yield "".
func (g *ExecGit) InitialCommit(ctx context.Context) (string, error) {
	out, err := g.run(ctx, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", nil
	}
	if i := strings.IndexByte(out, '\n'); i >= 0 {
		return out[:i], nil
	}
	return out, nil
}

// ContributorsSince returns author names for commits since tag.
func (g *ExecGit) ContributorsSince(ctx context.Context, tag string) ([]string, error) {
	return g.logField(ctx, "%an", tag)
}

func (g *ExecGit) logField(ctx context.Context, format, tag string) ([]string, error) {
	args := []string{"log", "--no-merges", "--format=" + format}
	if tag != "" {
		args = append(args, tag+"..HEAD")
	}
	out, err := g.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// TagExists reports whether tag exists locally.
func (g *ExecGit) TagExists(ctx context.Context, tag string) (bool, error) {
	out, err := g.run(ctx, "tag", "--list", tag)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// CreateAnnotatedTag creates an annotated tag at HEAD.
func (g *ExecGit) CreateAnnotatedTag(ctx context.Context, tag, message string) error {
	_, err := g.run(ctx, "tag", "-a", tag, "-m", message)
	return err
}

// PushTag pushes the tag to origin.
func (g *ExecGit) PushTag(ctx context.Context, tag string) error {
	_, err := g.run(ctx, "push", "origin", tag)
	return err
}

// CommitFile stages a single path and commits just that path. Verification
// hooks are skipped (--no-verify): the release CLI has already run its own
// validation, and the commit is a machine-generated CHANGELOG update, so
// re-running the pre-commit test suite here would be redundant.
func (g *ExecGit) CommitFile(ctx context.Context, path, message string) error {
	if _, err := g.run(ctx, "add", "--", path); err != nil {
		return err
	}
	_, err := g.run(ctx, "commit", "--no-verify", "-m", message, "--", path)
	return err
}

// PushBranch pushes the current HEAD to origin's branch.
func (g *ExecGit) PushBranch(ctx context.Context, branch string) error {
	_, err := g.run(ctx, "push", "origin", "HEAD:"+branch)
	return err
}

// UpstreamInSync reports whether HEAD and its upstream have not diverged. If
// there is no upstream tracking branch, it returns (false, nil) so validation
// flags it.
func (g *ExecGit) UpstreamInSync(ctx context.Context) (bool, error) {
	out, err := g.run(ctx, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		return false, nil // typically "no upstream configured"
	}
	fields := strings.Fields(out)
	return len(fields) == 2 && fields[0] == "0" && fields[1] == "0", nil
}
