package trigger

import "testing"

func TestMatches(t *testing.T) {
	cases := []struct {
		name, msg, pattern string
		want               bool
	}{
		{"default match", "blog: Added OAuth", "", true},
		{"default no match", "fix(api): resolve issue", "", false},
		{"leading whitespace", "  blog: hi", "", true},
		{"docs commit ignored", "docs: update README", "", false},
		{"refactor ignored", "refactor(core): simplify", "", false},
		{"custom bracket pattern", "[blog] new post", "[blog]", true},
		{"custom pattern no match", "blog: hi", "[blog]", false},
		{"empty message", "", "", false},
		{"prefix only substring not at start", "please blog: later", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Matches(tc.msg, tc.pattern); got != tc.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tc.msg, tc.pattern, got, tc.want)
			}
		})
	}
}
