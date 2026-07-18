package main

import "testing"

func TestParseArgsFlagPosition(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		sub    string
		dryRun bool
		pre    string
	}{
		{"flag before subcommand", []string{"--dry-run", "patch"}, "patch", true, ""},
		{"flag after subcommand", []string{"patch", "--dry-run"}, "patch", true, ""},
		{"flags both sides", []string{"--dry-run", "minor", "--no-verify"}, "minor", true, ""},
		{"value flag after subcommand", []string{"minor", "--pre", "rc.1"}, "minor", false, "rc.1"},
		{"value flag before subcommand", []string{"--pre", "rc.1", "major"}, "major", false, "rc.1"},
		{"no subcommand defaults to release", []string{"--dry-run"}, "release", true, ""},
		{"bare subcommand", []string{"patch"}, "patch", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o, code := parseArgs(tc.args)
			if o == nil {
				t.Fatalf("parseArgs(%v) failed with code %d", tc.args, code)
			}
			if o.sub != tc.sub {
				t.Errorf("sub = %q, want %q", o.sub, tc.sub)
			}
			if o.dryRun != tc.dryRun {
				t.Errorf("dryRun = %v, want %v", o.dryRun, tc.dryRun)
			}
			if o.pre != tc.pre {
				t.Errorf("pre = %q, want %q", o.pre, tc.pre)
			}
		})
	}
}

func TestParseArgsRejectsStrayArg(t *testing.T) {
	// A second positional token must be rejected, not silently ignored.
	if o, code := parseArgs([]string{"patch", "minor"}); o != nil || code != 2 {
		t.Errorf("parseArgs(patch minor) = (%v, %d), want (nil, 2)", o, code)
	}
	if o, code := parseArgs([]string{"patch", "--dry-run", "oops"}); o != nil || code != 2 {
		t.Errorf("parseArgs(patch --dry-run oops) = (%v, %d), want (nil, 2)", o, code)
	}
}

func TestParseArgsRejectsUnknownFlag(t *testing.T) {
	if o, code := parseArgs([]string{"patch", "--nope"}); o != nil || code != 2 {
		t.Errorf("parseArgs(patch --nope) = (%v, %d), want (nil, 2)", o, code)
	}
}
