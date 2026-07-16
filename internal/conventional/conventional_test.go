package conventional

import "testing"

func TestParseValid(t *testing.T) {
	cfg := Config{}
	cases := []struct {
		msg      string
		typ      string
		scope    string
		breaking bool
		desc     string
	}{
		{"feat: add thing", "feat", "", false, "add thing"},
		{"fix(api): handle nil", "fix", "api", false, "handle nil"},
		{"feat(ui)!: drop legacy prop", "feat", "ui", true, "drop legacy prop"},
		{"refactor: tidy", "refactor", "", false, "tidy"},
		{"feat: x\n\nBREAKING CHANGE: removes y", "feat", "", true, "x"},
	}
	for _, c := range cases {
		got, err := Parse(c.msg, cfg)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.msg, err)
			continue
		}
		if got.Type != c.typ || got.Scope != c.scope || got.Breaking != c.breaking || got.Description != c.desc {
			t.Errorf("Parse(%q) = %+v", c.msg, got)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	cfg := Config{}
	for _, msg := range []string{
		"add thing",      // no type
		"feat add thing", // no colon
		"feat:",          // no description (needs ": desc")
		"unknown: x",     // unknown type
		"Feat : x",       // malformed
		"",               // empty
	} {
		if _, err := Parse(msg, cfg); err == nil {
			t.Errorf("Parse(%q) = nil error, want error", msg)
		}
		if Validate(msg, cfg) == nil {
			t.Errorf("Validate(%q) = nil, want error", msg)
		}
	}
}

func TestDetermineBump(t *testing.T) {
	cfg := Config{}
	mk := func(msgs ...string) []Commit {
		var cs []Commit
		for _, m := range msgs {
			c, err := Parse(m, cfg)
			if err != nil {
				t.Fatalf("seed parse %q: %v", m, err)
			}
			cs = append(cs, c)
		}
		return cs
	}
	cases := []struct {
		name string
		msgs []string
		want Bump
	}{
		{"feat -> minor", []string{"feat: a"}, BumpMinor},
		{"fix -> patch", []string{"fix: a"}, BumpPatch},
		{"breaking -> major", []string{"feat!: a"}, BumpMajor},
		{"breaking footer -> major", []string{"fix: a\n\nBREAKING CHANGE: x"}, BumpMajor},
		{"docs/chore -> none", []string{"docs: a", "chore: b"}, BumpNone},
		{"mixed picks highest", []string{"fix: a", "feat: b", "docs: c"}, BumpMinor},
		{"major beats all", []string{"fix: a", "feat!: b", "feat: c"}, BumpMajor},
		{"empty -> none", nil, BumpNone},
	}
	for _, c := range cases {
		if got := DetermineBump(mk(c.msgs...), cfg); got != c.want {
			t.Errorf("%s: DetermineBump = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestBumpString(t *testing.T) {
	for b, want := range map[Bump]string{BumpNone: "none", BumpPatch: "patch", BumpMinor: "minor", BumpMajor: "major"} {
		if b.String() != want {
			t.Errorf("Bump(%d).String() = %q, want %q", b, b.String(), want)
		}
	}
}
