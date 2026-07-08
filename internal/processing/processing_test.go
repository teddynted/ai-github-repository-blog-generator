package processing

import (
	"context"
	"errors"
	"testing"
)

type fakeCloner struct {
	path string
	err  error
	ref  string
}

func (f *fakeCloner) Clone(_ context.Context, _, ref string) (string, error) {
	f.ref = ref
	return f.path, f.err
}

type stubReadme struct{ err error }

func (s stubReadme) Readme(_ context.Context, _ string) (string, error) {
	return "readme", s.err
}

type stubDocs struct{ err error }

func (s stubDocs) Docs(_ context.Context, _ string) ([]Document, error) {
	return []Document{{Path: "d.md", Content: "c"}}, s.err
}

type stubCommits struct {
	gotLimit int
	err      error
}

func (s *stubCommits) Commits(_ context.Context, _ string, limit int) ([]Commit, error) {
	s.gotLimit = limit
	return []Commit{{SHA: "s1", Message: "blog: x"}}, s.err
}

func TestProcessGathersSnapshot(t *testing.T) {
	cl := &fakeCloner{path: "/work/acme/widget"}
	commits := &stubCommits{}
	p := &Processor{Cloner: cl, Readme: stubReadme{}, Docs: stubDocs{}, Commits: commits, CommitLimit: 5}

	snap, err := p.Process(context.Background(), "acme/widget", "refs/heads/main")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if snap.RepoFullName != "acme/widget" || snap.Ref != "refs/heads/main" || snap.LocalPath != "/work/acme/widget" {
		t.Errorf("snapshot header = %+v", snap)
	}
	if snap.Readme != "readme" || len(snap.Docs) != 1 || len(snap.Commits) != 1 {
		t.Errorf("snapshot content = %+v", snap)
	}
	if cl.ref != "refs/heads/main" {
		t.Errorf("cloner ref = %q", cl.ref)
	}
	if commits.gotLimit != 5 {
		t.Errorf("commit limit = %d, want 5", commits.gotLimit)
	}
}

func TestProcessDefaultsCommitLimit(t *testing.T) {
	commits := &stubCommits{}
	p := &Processor{Cloner: &fakeCloner{path: "/w"}, Readme: stubReadme{}, Docs: stubDocs{}, Commits: commits}
	if _, err := p.Process(context.Background(), "a/b", "r"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if commits.gotLimit != 20 {
		t.Errorf("default commit limit = %d, want 20", commits.gotLimit)
	}
}

func TestProcessStopsOnCloneError(t *testing.T) {
	p := &Processor{Cloner: &fakeCloner{err: errors.New("no access")}, Readme: stubReadme{}, Docs: stubDocs{}, Commits: &stubCommits{}}
	if _, err := p.Process(context.Background(), "a/b", "r"); err == nil {
		t.Error("expected clone error to propagate")
	}
}

func TestProcessPropagatesRetrieverError(t *testing.T) {
	p := &Processor{Cloner: &fakeCloner{path: "/w"}, Readme: stubReadme{err: errors.New("boom")}, Docs: stubDocs{}, Commits: &stubCommits{}}
	if _, err := p.Process(context.Background(), "a/b", "r"); err == nil {
		t.Error("expected readme error to propagate")
	}
}

func TestPlaceholderProcessorProducesSnapshot(t *testing.T) {
	snap, err := NewPlaceholderProcessor().Process(context.Background(), "acme/widget", "refs/heads/main")
	if err != nil {
		t.Fatalf("placeholder Process: %v", err)
	}
	if snap.LocalPath == "" || snap.Readme == "" || len(snap.Docs) == 0 || len(snap.Commits) == 0 {
		t.Errorf("placeholder snapshot incomplete: %+v", snap)
	}
}

func TestPlaceholderCommitsRespectsLimit(t *testing.T) {
	c, err := PlaceholderCommits{}.Commits(context.Background(), "/x", 1)
	if err != nil {
		t.Fatalf("Commits: %v", err)
	}
	if len(c) != 1 {
		t.Errorf("limit not applied: got %d", len(c))
	}
}
