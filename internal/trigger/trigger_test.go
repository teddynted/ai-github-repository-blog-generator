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
		{"regex match", "post: hello", "regex:^(blog|post):", true},
		{"regex match blog", "blog: hello", "regex:^(blog|post):", true},
		{"regex no match", "fix: hello", "regex:^(blog|post):", false},
		{"regex anchored not at start", "x post: hello", "regex:^post:", false},
		{"invalid regex matches nothing", "blog: hi", "regex:[unclosed", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Matches(tc.msg, tc.pattern); got != tc.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tc.msg, tc.pattern, got, tc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	for _, ok := range []string{"blog:", "[blog]", "regex:^blog:", "regex:(a|b)", ""} {
		if err := Validate(ok); err != nil {
			t.Errorf("Validate(%q) unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"regex:[unclosed", "regex:(", "regex:*"} {
		if err := Validate(bad); err == nil {
			t.Errorf("Validate(%q) expected error", bad)
		}
	}
}
