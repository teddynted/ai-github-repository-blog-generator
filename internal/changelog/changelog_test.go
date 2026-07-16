package changelog

import (
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
)

func mk(t *testing.T, msgs ...string) []conventional.Commit {
	t.Helper()
	var cs []conventional.Commit
	for _, m := range msgs {
		c, err := conventional.Parse(m, conventional.Config{})
		if err != nil {
			t.Fatalf("parse %q: %v", m, err)
		}
		cs = append(cs, c)
	}
	return cs
}

func TestRenderGroupsByCategory(t *testing.T) {
	s := Render("1.2.0", "2026-07-16", mk(t,
		"feat(api): add endpoint",
		"feat: another feature",
		"fix(core): correct nil deref",
		"docs: tidy readme",
		"chore: bump deps", // no category → omitted
	), nil)

	if !strings.Contains(s, "## [1.2.0] - 2026-07-16") {
		t.Errorf("missing heading:\n%s", s)
	}
	for _, want := range []string{"### Features", "**api:** add endpoint", "### Bug Fixes", "**core:** correct nil deref", "### Documentation"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Contains(s, "bump deps") {
		t.Error("chore should not appear (no category)")
	}
	if strings.Contains(s, "### Breaking Changes") {
		t.Error("no breaking changes expected")
	}
}

func TestRenderBreaking(t *testing.T) {
	s := Render("2.0.0", "2026-07-16", mk(t, "feat!: drop v1 API", "fix: small"), nil)
	if !strings.Contains(s, "### Breaking Changes") || !strings.Contains(s, "drop v1 API") {
		t.Errorf("breaking section missing:\n%s", s)
	}
}

func TestUpdateInsertsAndDeduplicates(t *testing.T) {
	sec110 := Render("1.1.0", "2026-01-01", mk(t, "feat: a"), nil)
	first := Update("", "1.1.0", sec110)
	if !strings.HasPrefix(first, "# Changelog") || !strings.Contains(first, "## [1.1.0]") {
		t.Fatalf("first update wrong:\n%s", first)
	}

	// Add a newer version — it must appear ABOVE 1.1.0.
	sec120 := Render("1.2.0", "2026-02-02", mk(t, "feat: b"), nil)
	second := Update(first, "1.2.0", sec120)
	i12 := strings.Index(second, "## [1.2.0]")
	i11 := strings.Index(second, "## [1.1.0]")
	if i12 < 0 || i11 < 0 || i12 > i11 {
		t.Fatalf("1.2.0 should precede 1.1.0:\n%s", second)
	}

	// Re-adding an existing version is a no-op (no duplicate entries).
	third := Update(second, "1.2.0", sec120)
	if third != second {
		t.Error("re-adding an existing version must not change the file")
	}
	if strings.Count(third, "## [1.2.0]") != 1 {
		t.Error("duplicate 1.2.0 heading")
	}
}

func TestContains(t *testing.T) {
	c := Update("", "1.0.0", Render("1.0.0", "2026-01-01", mk(t, "feat: a"), nil))
	if !Contains(c, "1.0.0") || Contains(c, "9.9.9") {
		t.Error("Contains mismatch")
	}
}
