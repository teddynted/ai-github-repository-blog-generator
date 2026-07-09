package archdiagram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

// writeRepo materialises a fixture repository on disk and returns its root.
func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func serviceNames(services []Service) map[string]Confidence {
	m := map[string]Confidence{}
	for _, s := range services {
		m[s.Name] = s.Confidence
	}
	return m
}

func TestDetectTerraformServicesHighConfidence(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"infra/main.tf": `
resource "aws_s3_bucket" "assets" {}
resource "aws_dynamodb_table" "state" {}
resource "aws_lambda_function" "api" {}
resource "aws_sqs_queue" "jobs" {}
`,
	})
	snap := processing.Snapshot{
		RepoFullName: "acme/widget",
		LocalPath:    root,
		Analysis:     processing.Analysis{Languages: []string{"Go"}},
	}

	det := Detect(snap)
	names := serviceNames(det.Services)

	for _, want := range []string{"Amazon S3", "Amazon DynamoDB", "AWS Lambda", "Amazon SQS"} {
		if names[want] != High {
			t.Errorf("expected %s at High confidence, got %q", want, names[want])
		}
	}
	// Lambda evidence should make Lambda the compute substrate (not the EC2 default).
	if det.Compute.Name != "AWS Lambda" {
		t.Errorf("compute substrate = %q, want AWS Lambda (from evidence)", det.Compute.Name)
	}
	if !det.HasDatabase || !det.HasQueue {
		t.Errorf("capability flags not set: db=%v queue=%v", det.HasDatabase, det.HasQueue)
	}
	// Evidence must reference the file + token, never be empty.
	for _, s := range det.Services {
		if s.ID != "github" && s.ID != "ec2" && len(s.Evidence) == 0 {
			t.Errorf("service %s has no evidence", s.Name)
		}
	}
}

func TestDetectCloudFormationAndSDK(t *testing.T) {
	root := writeRepo(t, map[string]string{
		"template.yaml": `
Resources:
  Bucket:
    Type: AWS::S3::Bucket
  Secret:
    Type: AWS::SecretsManager::Secret
`,
		"package.json": `{"dependencies":{"@aws-sdk/client-dynamodb":"^3.0.0","ioredis":"^5.0.0"}}`,
	})
	det := Detect(processing.Snapshot{LocalPath: root})
	names := serviceNames(det.Services)

	if names["Amazon S3"] != High || names["AWS Secrets Manager"] != High {
		t.Errorf("CloudFormation services not High: %+v", names)
	}
	if names["Amazon DynamoDB"] != High {
		t.Errorf("SDK client should yield High DynamoDB, got %q", names["Amazon DynamoDB"])
	}
	if names["Amazon ElastiCache"] != Medium {
		t.Errorf("redis dependency should yield Medium ElastiCache, got %q", names["Amazon ElastiCache"])
	}
}

func TestDetectNoInvention(t *testing.T) {
	// A bare repo with no AWS evidence must not conjure services.
	root := writeRepo(t, map[string]string{
		"main.go":   "package main\nfunc main() {}\n",
		"README.md": "# Hello\nA small CLI tool.",
	})
	det := Detect(processing.Snapshot{LocalPath: root, Readme: "# Hello\nA small CLI tool."})

	for _, s := range det.Services {
		switch s.ID {
		case "github", "ec2":
			// Allowed: GitHub source + EC2 deployment-context default.
		default:
			t.Errorf("invented service with no evidence: %s (%s)", s.Name, s.Confidence)
		}
	}
	if det.Compute.Name != "Amazon EC2 (Spot)" {
		t.Errorf("compute default = %q, want Amazon EC2 (Spot)", det.Compute.Name)
	}
}

func TestDetectAIMapsToOpenClaw(t *testing.T) {
	det := Detect(processing.Snapshot{
		Readme: "This service uses an LLM via LangChain for RAG over documents.",
	})
	if !det.HasAI {
		t.Fatal("expected HasAI from README keywords")
	}
	var found bool
	for _, s := range det.Services {
		if s.ID == "openclaw" {
			found = true
			if strings.Contains(strings.ToLower(s.Name), "bedrock") {
				t.Error("AI must never map to Bedrock")
			}
		}
	}
	if !found {
		t.Error("AI usage should add an OpenClaw-on-EC2 service")
	}
}
