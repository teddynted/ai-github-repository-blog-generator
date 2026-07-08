package repo

import "testing"

func TestParseRepositoryURL(t *testing.T) {
	cases := []struct {
		name, in, owner, repo string
		wantErr               bool
	}{
		{"https", "https://github.com/acme/widget", "acme", "widget", false},
		{"https .git", "https://github.com/acme/widget.git", "acme", "widget", false},
		{"trailing slash", "https://github.com/acme/widget/", "acme", "widget", false},
		{"ssh scp form", "git@github.com:acme/widget.git", "acme", "widget", false},
		{"with whitespace", "  https://github.com/acme/widget  ", "acme", "widget", false},
		{"empty", "", "", "", true},
		{"wrong host", "https://gitlab.com/acme/widget", "", "", true},
		{"missing name", "https://github.com/acme", "", "", true},
		{"too many segments", "https://github.com/acme/widget/tree/main", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner, name, err := ParseRepositoryURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tc.owner || name != tc.repo {
				t.Errorf("got %s/%s, want %s/%s", owner, name, tc.owner, tc.repo)
			}
		})
	}
}

func TestFullName(t *testing.T) {
	if got := FullName("acme", "widget"); got != "acme/widget" {
		t.Errorf("FullName = %q", got)
	}
}
