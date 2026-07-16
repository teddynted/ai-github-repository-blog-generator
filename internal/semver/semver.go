// Package semver implements parsing, validation, comparison, and incrementing
// of Semantic Versioning 2.0.0 versions (https://semver.org/spec/v2.0.0.html).
//
// A version is MAJOR.MINOR.PATCH, optionally followed by a pre-release
// (-alpha.1) and/or build metadata (+build.5). Build metadata is ignored for
// precedence, per the spec.
package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// semverRE is the official SemVer 2.0.0 regular expression (named groups).
var semverRE = regexp.MustCompile(`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// Version is a parsed SemVer 2.0.0 version.
type Version struct {
	Major, Minor, Patch int
	PreRelease          string // without the leading '-'
	Build               string // without the leading '+'
}

// Parse parses s (with or without a leading 'v') into a Version. It rejects any
// string that does not conform to SemVer 2.0.0.
func Parse(s string) (Version, error) {
	raw := strings.TrimPrefix(strings.TrimSpace(s), "v")
	m := semverRE.FindStringSubmatch(raw)
	if m == nil {
		return Version{}, fmt.Errorf("invalid semantic version %q", s)
	}
	idx := func(name string) string { return m[semverRE.SubexpIndex(name)] }
	// The regex already guarantees the numeric fields fit the grammar; Atoi is
	// safe but we still surface an error rather than panic on absurd inputs.
	major, err := strconv.Atoi(idx("major"))
	if err != nil {
		return Version{}, fmt.Errorf("invalid major in %q: %w", s, err)
	}
	minor, err := strconv.Atoi(idx("minor"))
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor in %q: %w", s, err)
	}
	patch, err := strconv.Atoi(idx("patch"))
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch in %q: %w", s, err)
	}
	return Version{
		Major: major, Minor: minor, Patch: patch,
		PreRelease: idx("prerelease"),
		Build:      idx("buildmetadata"),
	}, nil
}

// IsValid reports whether s is a valid SemVer 2.0.0 version.
func IsValid(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// String renders the version in canonical form (no leading 'v').
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Core returns MAJOR.MINOR.PATCH without pre-release or build metadata.
func (v Version) Core() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
}

// IncMajor returns the next major release (X+1.0.0), clearing pre-release/build.
func (v Version) IncMajor() Version { return Version{Major: v.Major + 1} }

// IncMinor returns the next minor release (X.Y+1.0), clearing pre-release/build.
func (v Version) IncMinor() Version { return Version{Major: v.Major, Minor: v.Minor + 1} }

// IncPatch returns the next patch release (X.Y.Z+1), clearing pre-release/build.
func (v Version) IncPatch() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

// WithPreRelease returns a copy with the given pre-release identifier set (e.g.
// "rc.1"). An empty string clears it.
func (v Version) WithPreRelease(pre string) Version {
	v.PreRelease = strings.TrimPrefix(pre, "-")
	return v
}

// WithBuild returns a copy with the given build metadata set. Empty clears it.
func (v Version) WithBuild(build string) Version {
	v.Build = strings.TrimPrefix(build, "+")
	return v
}

// Compare returns -1, 0, or +1 as v is less than, equal to, or greater than o,
// per SemVer precedence (build metadata is ignored).
func (v Version) Compare(o Version) int {
	if c := cmpInt(v.Major, o.Major); c != 0 {
		return c
	}
	if c := cmpInt(v.Minor, o.Minor); c != 0 {
		return c
	}
	if c := cmpInt(v.Patch, o.Patch); c != 0 {
		return c
	}
	return comparePreRelease(v.PreRelease, o.PreRelease)
}

// LessThan reports whether v has lower precedence than o.
func (v Version) LessThan(o Version) bool { return v.Compare(o) < 0 }

// Equal reports whether v and o have equal precedence (ignoring build metadata).
func (v Version) Equal(o Version) bool { return v.Compare(o) == 0 }

// comparePreRelease implements SemVer §11: a version WITHOUT a pre-release has
// higher precedence than one WITH; otherwise identifiers are compared per §11.4.
func comparePreRelease(a, b string) int {
	switch {
	case a == "" && b == "":
		return 0
	case a == "":
		return 1 // no pre-release > pre-release
	case b == "":
		return -1
	}
	ai, bi := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(ai) && i < len(bi); i++ {
		if c := compareIdentifier(ai[i], bi[i]); c != 0 {
			return c
		}
	}
	// A larger set of identifiers, when the preceding ones are equal, wins.
	return cmpInt(len(ai), len(bi))
}

func compareIdentifier(a, b string) int {
	an, aNum := toInt(a)
	bn, bNum := toInt(b)
	switch {
	case aNum && bNum:
		return cmpInt(an, bn) // both numeric: compare numerically
	case aNum:
		return -1 // numeric identifiers have lower precedence than alphanumeric
	case bNum:
		return 1
	default:
		return strings.Compare(a, b) // ASCII sort order
	}
}

func toInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
