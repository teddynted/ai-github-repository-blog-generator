package release

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo creates a throwaway git repo with one commit and returns its dir.
func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "Tester"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "seed"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("add", "-A")
	run("commit", "-q", "-m", "chore: seed")
	return dir
}

func headSubject(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	return string(out)
}

func TestCommitFileCommitsThenNoOps(t *testing.T) {
	dir := initRepo(t)
	g := NewExecGit(dir)
	ctx := context.Background()
	path := filepath.Join(dir, "CHANGELOG.md")

	// First write + commit records a new commit.
	if err := os.WriteFile(path, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.CommitFile(ctx, path, "chore(release): v1.0.0"); err != nil {
		t.Fatalf("first CommitFile: %v", err)
	}
	if got := headSubject(t, dir); got != "chore(release): v1.0.0\n" {
		t.Fatalf("HEAD subject = %q", got)
	}

	// Re-writing identical content must be a no-op (no "nothing to commit" error,
	// HEAD unchanged) — the idempotent-recovery path.
	if err := os.WriteFile(path, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.CommitFile(ctx, path, "chore(release): v1.0.0"); err != nil {
		t.Fatalf("idempotent CommitFile returned error: %v", err)
	}
	if got := headSubject(t, dir); got != "chore(release): v1.0.0\n" {
		t.Fatalf("HEAD moved on no-op commit: %q", got)
	}
	if clean, _ := g.IsClean(ctx); !clean {
		t.Error("tree should be clean after no-op commit")
	}
}
