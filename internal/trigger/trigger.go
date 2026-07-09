// Package trigger implements the commit-message publishing trigger: blog
// generation runs only when a commit message matches the configured pattern.
// The MVP uses a simple prefix match (default "blog:"); richer patterns
// (regex, "[blog]", ...) are a documented future enhancement.
package trigger

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultPattern is the platform-default trigger prefix.
const DefaultPattern = "blog:"

// RegexPrefix marks a pattern as a regular expression, e.g.
// "regex:^(blog|post):". Without it, a pattern is a literal prefix (so "blog:"
// and "[blog]" both work as prefixes).
const RegexPrefix = "regex:"

// Matches reports whether commitMessage requests a blog run under pattern.
// A blank pattern falls back to DefaultPattern. Leading whitespace in the
// commit message is ignored. A "regex:"-prefixed pattern is matched as a
// regular expression (an invalid regex matches nothing).
func Matches(commitMessage, pattern string) bool {
	if pattern == "" {
		pattern = DefaultPattern
	}
	msg := strings.TrimLeft(commitMessage, " \t\r\n")
	if expr, ok := strings.CutPrefix(pattern, RegexPrefix); ok {
		re, err := regexp.Compile(expr)
		if err != nil {
			return false
		}
		return re.MatchString(msg)
	}
	return strings.HasPrefix(msg, pattern)
}

// Validate returns an error for a malformed trigger pattern (an invalid
// "regex:" expression). Literal prefixes are always valid.
func Validate(pattern string) error {
	if expr, ok := strings.CutPrefix(pattern, RegexPrefix); ok {
		if _, err := regexp.Compile(expr); err != nil {
			return fmt.Errorf("invalid trigger regex: %w", err)
		}
	}
	return nil
}
