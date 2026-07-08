// Package trigger implements the commit-message publishing trigger: blog
// generation runs only when a commit message matches the configured pattern.
// The MVP uses a simple prefix match (default "blog:"); richer patterns
// (regex, "[blog]", ...) are a documented future enhancement.
package trigger

import "strings"

// DefaultPattern is the platform-default trigger prefix.
const DefaultPattern = "blog:"

// Matches reports whether commitMessage requests a blog run under pattern.
// A blank pattern falls back to DefaultPattern. Leading whitespace in the
// commit message is ignored.
func Matches(commitMessage, pattern string) bool {
	if pattern == "" {
		pattern = DefaultPattern
	}
	return strings.HasPrefix(strings.TrimLeft(commitMessage, " \t\r\n"), pattern)
}
