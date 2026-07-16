// Package conventional parses and validates Conventional Commits 1.0.0 messages
// (https://www.conventionalcommits.org/en/v1.0.0/) and derives the Semantic
// Versioning bump implied by a set of commits.
package conventional

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultTypes are the commit types recognised by default.
var DefaultTypes = []string{
	"feat", "fix", "docs", "refactor", "perf", "test", "build", "ci", "chore", "revert",
}

// header matches "type(scope)!: description". Scope and '!' are optional.
var header = regexp.MustCompile(`^(?P<type>[a-zA-Z]+)(?:\((?P<scope>[^)]+)\))?(?P<breaking>!)?: (?P<desc>.+)$`)

// Commit is a parsed Conventional Commit.
type Commit struct {
	Type        string
	Scope       string
	Breaking    bool
	Description string
	Body        string
	// Raw is the original first line (header) for reference.
	Raw string
}

// Bump is the SemVer increment a set of commits implies.
type Bump int

const (
	// BumpNone means no release-worthy change.
	BumpNone Bump = iota
	BumpPatch
	BumpMinor
	BumpMajor
)

func (b Bump) String() string {
	switch b {
	case BumpPatch:
		return "patch"
	case BumpMinor:
		return "minor"
	case BumpMajor:
		return "major"
	default:
		return "none"
	}
}

// Config controls parsing/validation and bump derivation.
type Config struct {
	// Types is the set of allowed commit types. Empty uses DefaultTypes.
	Types []string `json:"types,omitempty"`
	// MinorTypes bump the minor version (default: feat).
	MinorTypes []string `json:"minor_types,omitempty"`
	// PatchTypes bump the patch version (default: fix, perf, revert).
	PatchTypes []string `json:"patch_types,omitempty"`
	// A commit with a breaking change always bumps major regardless of type.
}

func (c Config) types() []string {
	if len(c.Types) > 0 {
		return c.Types
	}
	return DefaultTypes
}

func (c Config) minorTypes() []string {
	if len(c.MinorTypes) > 0 {
		return c.MinorTypes
	}
	return []string{"feat"}
}

func (c Config) patchTypes() []string {
	if len(c.PatchTypes) > 0 {
		return c.PatchTypes
	}
	return []string{"fix", "perf", "revert"}
}

// Parse parses a full commit message (header + optional body/footers) into a
// Commit. The header must conform to Conventional Commits, and the type must be
// one of the configured types; otherwise an error is returned.
func Parse(message string, cfg Config) (Commit, error) {
	message = strings.ReplaceAll(message, "\r\n", "\n")
	lines := strings.SplitN(strings.TrimRight(message, "\n"), "\n", 2)
	head := strings.TrimSpace(lines[0])
	body := ""
	if len(lines) == 2 {
		body = strings.TrimSpace(lines[1])
	}

	m := header.FindStringSubmatch(head)
	if m == nil {
		return Commit{}, fmt.Errorf("not a conventional commit header: %q (want \"type(scope)!: description\")", head)
	}
	idx := func(n string) string { return m[header.SubexpIndex(n)] }
	typ := strings.ToLower(idx("type"))
	if !contains(cfg.types(), typ) {
		return Commit{}, fmt.Errorf("unknown commit type %q (allowed: %s)", typ, strings.Join(cfg.types(), ", "))
	}

	c := Commit{
		Type:        typ,
		Scope:       idx("scope"),
		Breaking:    idx("breaking") == "!",
		Description: strings.TrimSpace(idx("desc")),
		Body:        body,
		Raw:         head,
	}
	// A "BREAKING CHANGE:" (or "BREAKING-CHANGE:") footer also marks a breaking change.
	if hasBreakingFooter(body) {
		c.Breaking = true
	}
	return c, nil
}

// Validate reports whether message is a valid Conventional Commit under cfg.
func Validate(message string, cfg Config) error {
	_, err := Parse(message, cfg)
	return err
}

// DetermineBump returns the highest bump implied by the commits: major for any
// breaking change, else minor for a MinorType (feat), else patch for a
// PatchType (fix/perf/revert), else none. Types outside those sets (docs,
// chore, test, …) do not force a release on their own.
func DetermineBump(commits []Commit, cfg Config) Bump {
	bump := BumpNone
	for _, c := range commits {
		switch {
		case c.Breaking:
			return BumpMajor // nothing outranks major
		case contains(cfg.minorTypes(), c.Type):
			if bump < BumpMinor {
				bump = BumpMinor
			}
		case contains(cfg.patchTypes(), c.Type):
			if bump < BumpPatch {
				bump = BumpPatch
			}
		}
	}
	return bump
}

func hasBreakingFooter(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "BREAKING CHANGE:") || strings.HasPrefix(l, "BREAKING-CHANGE:") {
			return true
		}
	}
	return false
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
