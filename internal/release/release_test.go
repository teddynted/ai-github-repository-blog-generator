package release

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/conventional"
)

type fakeGit struct {
	branch    string
	clean     bool
	latestTag string
	subjects  []string
	tagExists bool
	synced    bool
	created   string
	pushed    string
	contribs  []string
}

func (f *fakeGit) CurrentBranch(context.Context) (string, error)           { return f.branch, nil }
func (f *fakeGit) IsClean(context.Context) (bool, error)                   { return f.clean, nil }
func (f *fakeGit) LatestSemverTag(context.Context, string) (string, error) { return f.latestTag, nil }
func (f *fakeGit) CommitSubjectsSince(context.Context, string) ([]string, error) {
	return f.subjects, nil
}
func (f *fakeGit) TagExists(context.Context, string) (bool, error) { return f.tagExists, nil }
func (f *fakeGit) CreateAnnotatedTag(_ context.Context, tag, _ string) error {
	f.created = tag
	return nil
}
func (f *fakeGit) PushTag(_ context.Context, tag string) error  { f.pushed = tag; return nil }
func (f *fakeGit) UpstreamInSync(context.Context) (bool, error) { return f.synced, nil }
func (f *fakeGit) ContributorsSince(context.Context, string) ([]string, error) {
	return f.contribs, nil
}

type fakeGH struct {
	authed  bool
	exists  bool
	created bool
	tag     string
}

func (f *fakeGH) Authenticated(context.Context) bool                  { return f.authed }
func (f *fakeGH) ReleaseExists(context.Context, string) (bool, error) { return f.exists, nil }
func (f *fakeGH) CreateRelease(_ context.Context, tag, _, _ string, _ bool) (string, error) {
	f.created, f.tag = true, tag
	return "https://github.com/acme/x/releases/tag/" + tag, nil
}

func newSvc(g *fakeGit, gh *fakeGH) *Service {
	return &Service{Git: g, GitHub: gh, Config: Default(),
		Now: func() time.Time { return time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC) }}
}

func TestDeterminePlanAutoBump(t *testing.T) {
	g := &fakeGit{latestTag: "v1.2.3", subjects: []string{"feat: a", "fix: b"}}
	plan, err := newSvc(g, &fakeGH{}).DeterminePlan(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("DeterminePlan: %v", err)
	}
	if plan.NextVersion.String() != "1.3.0" || plan.Tag != "v1.3.0" || plan.Bump != conventional.BumpMinor {
		t.Errorf("plan = %+v", plan)
	}
	if plan.Date != "2026-07-16" {
		t.Errorf("date = %q", plan.Date)
	}
	if !strings.Contains(plan.ChangelogSection, "## [1.3.0]") || !strings.Contains(plan.Notes, "## 1.3.0") {
		t.Errorf("artifacts not rendered")
	}
}

func TestDeterminePlanFirstRelease(t *testing.T) {
	g := &fakeGit{latestTag: "", subjects: []string{"feat: initial"}}
	plan, err := newSvc(g, &fakeGH{}).DeterminePlan(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("DeterminePlan: %v", err)
	}
	if plan.NextVersion.String() != "0.1.0" || plan.Tag != "v0.1.0" {
		t.Errorf("first release should use initial_version, got %s", plan.Tag)
	}
}

func TestDeterminePlanOverrideAndPrerelease(t *testing.T) {
	g := &fakeGit{latestTag: "v1.2.3", subjects: []string{"fix: b"}}
	major := conventional.BumpMajor
	plan, _ := newSvc(g, &fakeGH{}).DeterminePlan(context.Background(), &major, "rc.1")
	if plan.NextVersion.String() != "2.0.0-rc.1" || plan.Tag != "v2.0.0-rc.1" {
		t.Errorf("override+pre = %s", plan.Tag)
	}
}

func TestValidatePasses(t *testing.T) {
	g := &fakeGit{branch: "main", clean: true, latestTag: "v1.0.0", subjects: []string{"feat: a"}, synced: true}
	svc := newSvc(g, &fakeGH{authed: true})
	plan, _ := svc.DeterminePlan(context.Background(), nil, "")
	if errs := svc.Validate(context.Background(), plan, ""); len(errs) != 0 {
		t.Errorf("expected clean validation, got %v", errs)
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	g := &fakeGit{branch: "feature", clean: false, latestTag: "v1.0.0",
		subjects: []string{"feat: a", "not conventional"}, tagExists: true, synced: false}
	svc := newSvc(g, &fakeGH{authed: false})
	plan, _ := svc.DeterminePlan(context.Background(), nil, "")
	errs := svc.Validate(context.Background(), plan, "")
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, want := range []string{"working tree is not clean", "branch", "not in sync", "non-conventional", "already exists", "no GitHub authentication"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing validation %q in:\n%s", want, joined)
		}
	}
}

func TestValidateRejectsDowngrade(t *testing.T) {
	g := &fakeGit{branch: "main", clean: true, latestTag: "v2.0.0", subjects: []string{"fix: b"}, synced: true}
	svc := newSvc(g, &fakeGH{authed: true})
	patch := conventional.BumpPatch
	// Force a patch but pretend latest is higher by overriding next below prev is
	// impossible here; instead verify the "no downgrade" guard via changelog dup.
	plan, _ := svc.DeterminePlan(context.Background(), &patch, "")
	if errs := svc.Validate(context.Background(), plan, "## [2.0.1] - x"); len(errs) == 0 {
		t.Error("expected CHANGELOG-duplicate validation to fail")
	}
}

func TestApplyDryRunMakesNoChanges(t *testing.T) {
	g := &fakeGit{latestTag: "v1.0.0", subjects: []string{"feat: a"}}
	gh := &fakeGH{authed: true}
	svc := newSvc(g, gh)
	plan, _ := svc.DeterminePlan(context.Background(), nil, "")
	sum, err := svc.Apply(context.Background(), plan, "", true)
	if err != nil {
		t.Fatalf("Apply dry-run: %v", err)
	}
	if g.created != "" || g.pushed != "" || gh.created {
		t.Error("dry-run must not tag, push, or create a release")
	}
	if !strings.Contains(sum.Changelog, "## [1.1.0]") {
		t.Error("dry-run should still compute the changelog")
	}
}

func TestApplyRealCreatesTagAndRelease(t *testing.T) {
	g := &fakeGit{latestTag: "v1.0.0", subjects: []string{"feat: a"}}
	gh := &fakeGH{authed: true}
	svc := newSvc(g, gh)
	plan, _ := svc.DeterminePlan(context.Background(), nil, "")
	sum, err := svc.Apply(context.Background(), plan, "", false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if g.created != "v1.1.0" || g.pushed != "v1.1.0" || !gh.created || sum.ReleaseURL == "" {
		t.Errorf("release not created: git=%+v gh=%+v sum=%+v", g, gh, sum)
	}
}
