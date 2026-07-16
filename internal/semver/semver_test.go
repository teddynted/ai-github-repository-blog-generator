package semver

import "testing"

func TestParseValid(t *testing.T) {
	cases := []struct {
		in            string
		maj, min, pat int
		pre, build    string
	}{
		{"1.2.3", 1, 2, 3, "", ""},
		{"v1.2.3", 1, 2, 3, "", ""},
		{"0.0.0", 0, 0, 0, "", ""},
		{"1.0.0-alpha", 1, 0, 0, "alpha", ""},
		{"1.0.0-alpha.1", 1, 0, 0, "alpha.1", ""},
		{"1.0.0-0.3.7", 1, 0, 0, "0.3.7", ""},
		{"1.0.0+build.5", 1, 0, 0, "", "build.5"},
		{"1.0.0-rc.1+exp.sha.5114f85", 1, 0, 0, "rc.1", "exp.sha.5114f85"},
		{"10.20.30", 10, 20, 30, "", ""},
	}
	for _, c := range cases {
		v, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if v.Major != c.maj || v.Minor != c.min || v.Patch != c.pat || v.PreRelease != c.pre || v.Build != c.build {
			t.Errorf("Parse(%q) = %+v", c.in, v)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{
		"", "1", "1.2", "1.2.3.4", "01.2.3", "1.02.3", "1.2.03",
		"1.2.3-", "1.2.3-01", "v", "x.y.z", "1.2.3-alpha_beta", "-1.0.0",
		"1.0.0+", "1.0.0-+build",
	} {
		if v, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %+v, want error", in, v)
		}
		if IsValid(in) {
			t.Errorf("IsValid(%q) = true, want false", in)
		}
	}
}

func TestString(t *testing.T) {
	for _, in := range []string{"1.2.3", "1.0.0-rc.1", "1.0.0-rc.1+exp.sha.5114f85", "2.0.0+build"} {
		v, _ := Parse(in)
		if got := v.String(); got != in {
			t.Errorf("String round-trip: %q -> %q", in, got)
		}
	}
}

func TestIncrement(t *testing.T) {
	v, _ := Parse("1.2.3-rc.1+build")
	if got := v.IncMajor().String(); got != "2.0.0" {
		t.Errorf("IncMajor = %s, want 2.0.0", got)
	}
	if got := v.IncMinor().String(); got != "1.3.0" {
		t.Errorf("IncMinor = %s, want 1.3.0", got)
	}
	if got := v.IncPatch().String(); got != "1.2.4" {
		t.Errorf("IncPatch = %s, want 1.2.4", got)
	}
	if got := v.Core().IncPatch().WithPreRelease("rc.2").String(); got != "1.2.4-rc.2" {
		t.Errorf("chained = %s", got)
	}
}

func TestPrecedence(t *testing.T) {
	// SemVer §11 example ordering (ascending).
	ordered := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0",
		"1.0.1", "1.1.0", "2.0.0",
	}
	for i := 1; i < len(ordered); i++ {
		a, _ := Parse(ordered[i-1])
		b, _ := Parse(ordered[i])
		if !a.LessThan(b) {
			t.Errorf("expected %s < %s", ordered[i-1], ordered[i])
		}
		if a.Compare(b) != -1 || b.Compare(a) != 1 {
			t.Errorf("Compare mismatch for %s vs %s", ordered[i-1], ordered[i])
		}
	}
}

func TestEqualIgnoresBuild(t *testing.T) {
	a, _ := Parse("1.2.3+build.1")
	b, _ := Parse("1.2.3+build.2")
	if !a.Equal(b) {
		t.Error("build metadata must not affect precedence")
	}
}
