package releasecontext

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMermaidCapturesNodeLabels(t *testing.T) {
	block := "flowchart LR\n" +
		"  GH[GitHub Release] -->|push| WH[Webhook]\n" +
		"  WH --> SQS(Amazon SQS)\n" +
		"  SQS --> OLL[(Ollama Runtime)]\n"
	d := parseMermaid(block)

	want := map[string]string{
		"GH":  "GitHub Release",
		"WH":  "Webhook",
		"SQS": "Amazon SQS",
		"OLL": "Ollama Runtime", // cylinder "[(...)]" shape stripped
	}
	for id, label := range want {
		if got := d.NodeLabels[id]; got != label {
			t.Errorf("NodeLabels[%q] = %q, want %q", id, got, label)
		}
	}
}

func TestBuildRepoLevelFromWorkingTree(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# Demo\n\nAn event-driven platform on AWS using Ollama and Amazon Bedrock.\n")
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/demo\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(dir, "docs", "architecture.md"),
		"# Architecture\n\n```mermaid\nflowchart LR\n  SQS[Amazon SQS] --> LAM[AWS Lambda]\n  LAM --> S3[Amazon S3]\n```\n")
	mustWrite(t, filepath.Join(dir, "internal", "worker", "worker.go"), "package worker\n")
	// A CloudFormation template grounds the AWS services (prose alone does not).
	mustWrite(t, filepath.Join(dir, "infrastructure", "stack.yaml"),
		"Resources:\n  Queue:\n    Type: AWS::SQS::Queue\n  Fn:\n    Type: AWS::Lambda::Function\n  Bucket:\n    Type: AWS::S3::Bucket\n")

	rc, err := BuildRepoLevel(context.Background(), dir, "demo-repo")
	if err != nil {
		t.Fatalf("BuildRepoLevel: %v", err)
	}
	if rc.Repository.Name != "demo-repo" {
		t.Errorf("repository name = %q, want demo-repo", rc.Repository.Name)
	}
	// Version-independent: no release info.
	if rc.Release.Tag != "" || rc.Release.PublishedAt != "" {
		t.Errorf("repo-level context must have no release, got %+v", rc.Release)
	}
	if len(rc.Mermaid) == 0 {
		t.Error("expected the docs/ mermaid diagram to be parsed")
	}
	if len(rc.Architecture.AWSServices) == 0 {
		t.Errorf("expected AWS services grounded from the tree, got none")
	}
	if len(rc.RepositoryStructure.Directories) == 0 {
		t.Error("expected top-level directories from the tree")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ensure the ignore set actually excludes generated dirs.
func TestInventorySkipsIgnoredDirs(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# x\n")
	mustWrite(t, filepath.Join(dir, "output", "releases", "v1", "blog.md"), "generated\n")
	mustWrite(t, filepath.Join(dir, "_scratch", "note.md"), "temp\n")
	files, err := inventory(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasPrefix(f.Path, "output/") || strings.HasPrefix(f.Path, "_scratch/") {
			t.Errorf("inventory should skip ignored dir, found %q", f.Path)
		}
	}
}
