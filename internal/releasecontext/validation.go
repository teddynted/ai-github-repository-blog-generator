package releasecontext

import (
	"regexp"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// ownerRepoRe is GitHub's allowed character set for owner and repository names.
var ownerRepoRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// tagRe is a permissive but safe Git tag matcher (no whitespace or path tricks).
var tagRe = regexp.MustCompile(`^[A-Za-z0-9_.+/-]+$`)

// Validate checks a request and returns an apperror.CodeInvalidInput error that
// names every problem, so the API can return one clear 400.
func (r Request) Validate() error {
	var problems []string
	check := func(name, val string, re *regexp.Regexp) {
		switch {
		case strings.TrimSpace(val) == "":
			problems = append(problems, name+" is required")
		case len(val) > 200:
			problems = append(problems, name+" is too long")
		case !re.MatchString(val):
			problems = append(problems, name+" contains invalid characters")
		}
	}
	check("owner", r.Owner, ownerRepoRe)
	check("repository", r.Repository, ownerRepoRe)
	check("releaseTag", r.ReleaseTag, tagRe)

	if len(problems) > 0 {
		return apperror.New(apperror.CodeInvalidInput, "invalid request: "+strings.Join(problems, "; "))
	}
	return nil
}
