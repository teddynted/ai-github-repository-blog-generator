package publish

import (
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

// runSegments returns the path segments that identify a run within a
// repository. A release run is first-class and tag-addressable
// ("releases/<tag>"); a snapshot run keeps the dated layout ("<date>"). Both
// publishers (S3 and filesystem) share this so the layouts stay in lock-step.
func runSegments(release, date string) []string {
	if r := strings.TrimSpace(release); r != "" {
		return []string{"releases", safeSegment(r)}
	}
	return []string{date}
}

// batchRelease returns the release tag for a batch of assets (uniform within a
// single run), or "" when it is a snapshot run.
func batchRelease(assets []generation.Content) string {
	for _, a := range assets {
		if r := strings.TrimSpace(a.Release); r != "" {
			return r
		}
	}
	return ""
}

// safeSegment neutralises anything that could let a tag escape the destination
// (path separators, parent-directory traversal). SemVer tags are otherwise
// left intact, so "v1.2.0" and "v1.2.0-rc.1+build" pass through unchanged.
func safeSegment(s string) string {
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "..", "-")
	return strings.Trim(s, "-")
}
