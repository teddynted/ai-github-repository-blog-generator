// Package review performs a lightweight, rule-based quality review of generated
// content before it is published: it checks each asset for non-emptiness, a
// minimum length, and a Markdown heading, returning the assets that pass plus
// findings for those that do not. A future enhancement could add a second local
// model pass for tone/accuracy behind the same Reviewer shape.
package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

// DefaultMinLength is the minimum acceptable Markdown length.
const DefaultMinLength = 100

// Reviewer applies quality rules to generated content.
type Reviewer struct {
	MinLength int
}

// Review returns the assets that pass and human-readable findings for failures.
func (r Reviewer) Review(_ context.Context, assets []generation.Content) ([]generation.Content, []string) {
	min := r.MinLength
	if min <= 0 {
		min = DefaultMinLength
	}

	var passed []generation.Content
	var findings []string
	for _, a := range assets {
		if problems := check(a, min); len(problems) > 0 {
			findings = append(findings, fmt.Sprintf("%s: %s", a.Kind, strings.Join(problems, ", ")))
			continue
		}
		passed = append(passed, a)
	}
	return passed, findings
}

func check(a generation.Content, min int) []string {
	var problems []string
	body := strings.TrimSpace(a.Markdown)
	if body == "" {
		return []string{"empty content"}
	}
	if len(body) < min {
		problems = append(problems, fmt.Sprintf("too short (<%d bytes)", min))
	}
	if !hasHeading(body) {
		problems = append(problems, "no Markdown heading")
	}
	return problems
}

func hasHeading(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			return true
		}
	}
	return false
}
