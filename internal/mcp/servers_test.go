package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// call is a helper that runs a server tool and fails on a transport error.
func call(t *testing.T, s Server, tool string, args map[string]any) ToolResult {
	t.Helper()
	res, err := s.CallTool(context.Background(), tool, args)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	return res
}

// jsonOf returns the JSON payload of the first content item.
func jsonOf(t *testing.T, r ToolResult) any {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("no content")
	}
	return r.Content[0].JSON
}

func TestFilesystemServer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newFilesystemServer(dir, 0)
	if s.Info().Name != "filesystem" || s.Category() != CategoryFilesystem || !s.Capabilities().Tools {
		t.Fatal("descriptor accessors wrong")
	}

	if got := call(t, s, "read_file", map[string]any{"path": "a.txt"}).Content[0].Text; got != "hello" {
		t.Fatalf("read_file = %q", got)
	}
	names := jsonOf(t, call(t, s, "list_dir", map[string]any{"path": "."})).([]string)
	if len(names) != 2 || names[0] != "a.txt" || names[1] != "sub/" {
		t.Fatalf("list_dir = %+v", names)
	}
	stat := jsonOf(t, call(t, s, "stat", map[string]any{"path": "sub"})).(map[string]any)
	if stat["kind"] != "directory" {
		t.Fatalf("stat kind = %v", stat["kind"])
	}

	// Path traversal is neutralized: "../../etc/passwd" is clamped inside the
	// sandbox root, so it resolves to a non-existent path rather than escaping.
	if got := call(t, s, "read_file", map[string]any{"path": "../../etc/passwd"}); !got.IsError {
		t.Fatal("traversal must not read outside the sandbox")
	}
	// Reading a directory / missing file returns tool errors, not faults.
	if r := call(t, s, "read_file", map[string]any{"path": "sub"}); !r.IsError {
		t.Fatal("reading a directory should be a tool error")
	}
	if r := call(t, s, "read_file", map[string]any{"path": "missing"}); !r.IsError {
		t.Fatal("missing file should be a tool error")
	}
	// Max size guard.
	small := newFilesystemServer(dir, 2)
	if r := call(t, small, "read_file", map[string]any{"path": "a.txt"}); !r.IsError {
		t.Fatal("oversize read should be a tool error")
	}
	if _, err := s.CallTool(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown tool should error")
	}
}

func TestDocumentationServer(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# Guide\nInstall the tool\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o600)
	s := newDocumentationServer(dir)
	if s.Info().Name != "documentation" || s.Category() != CategoryDocumentation {
		t.Fatal("descriptor wrong")
	}
	docs := jsonOf(t, call(t, s, "list_docs", nil)).([]string)
	if len(docs) != 1 || docs[0] != "guide.md" {
		t.Fatalf("list_docs = %+v", docs)
	}
	if got := call(t, s, "get_doc", map[string]any{"path": "guide.md"}).Content[0].Text; !strings.Contains(got, "Install") {
		t.Fatalf("get_doc = %q", got)
	}
	if r := call(t, s, "get_doc", map[string]any{"path": "missing.md"}); !r.IsError {
		t.Fatal("missing doc should be a tool error")
	}
	hits := jsonOf(t, call(t, s, "search_docs", map[string]any{"query": "install"}))
	if hits == nil {
		t.Fatal("search should find the install line")
	}
	if _, err := s.CallTool(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown tool should error")
	}
}

func TestRepositoryServer(t *testing.T) {
	s := newRepositoryServer(RepoSnapshot{
		Name: "demo", DefaultBranch: "main",
		Commits: []RepoCommit{{SHA: "1"}, {SHA: "2"}, {SHA: "3"}},
		Tags:    []string{"v2", "v1"},
		Files:   []string{"a.go", "docs/x.md", "b.go"},
	})
	desc := jsonOf(t, call(t, s, "describe_repo", nil)).(map[string]any)
	if desc["name"] != "demo" || desc["commits"].(int) != 3 {
		t.Fatalf("describe = %+v", desc)
	}
	commits := jsonOf(t, call(t, s, "list_commits", map[string]any{"limit": "2"})).([]RepoCommit)
	if len(commits) != 2 {
		t.Fatalf("limit not applied: %+v", commits)
	}
	tags := jsonOf(t, call(t, s, "list_tags", nil)).([]string)
	if tags[0] != "v1" {
		t.Fatalf("tags not sorted: %+v", tags)
	}
	files := jsonOf(t, call(t, s, "list_files", map[string]any{"prefix": "docs/"})).([]string)
	if len(files) != 1 || files[0] != "docs/x.md" {
		t.Fatalf("prefix filter wrong: %+v", files)
	}
	if _, err := s.CallTool(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown tool should error")
	}
}

func TestDatabaseServer(t *testing.T) {
	s := newDatabaseServer(DBSnapshot{Tables: map[string]DBTable{
		"users": {Columns: []string{"id", "name"}, Rows: []map[string]any{
			{"id": "1", "name": "ada"}, {"id": "2", "name": "grace"}, {"id": "3", "name": "ada"},
		}},
	}})
	list := jsonOf(t, call(t, s, "list_tables", nil)).(map[string]any)
	if list["engine"] != "in-memory" {
		t.Fatalf("engine default wrong: %+v", list)
	}
	desc := jsonOf(t, call(t, s, "describe_table", map[string]any{"table": "users"})).(map[string]any)
	if desc["rows"].(int) != 3 {
		t.Fatalf("describe wrong: %+v", desc)
	}
	rows := jsonOf(t, call(t, s, "query", map[string]any{"table": "users", "filter_column": "name", "filter_value": "ada"})).([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("filter wrong: %+v", rows)
	}
	limited := jsonOf(t, call(t, s, "query", map[string]any{"table": "users", "limit": "1"})).([]map[string]any)
	if len(limited) != 1 {
		t.Fatalf("limit wrong: %+v", limited)
	}
	if r := call(t, s, "describe_table", map[string]any{"table": "nope"}); !r.IsError {
		t.Fatal("missing table should be a tool error")
	}
	if r := call(t, s, "query", map[string]any{"table": "nope"}); !r.IsError {
		t.Fatal("missing table query should be a tool error")
	}
	if r := call(t, s, "query", map[string]any{"table": "users", "filter_column": "ghost"}); !r.IsError {
		t.Fatal("bad column should be a tool error")
	}
	if _, err := s.CallTool(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown tool should error")
	}
}

func TestCloudFormationServer(t *testing.T) {
	s := newCloudFormationServer()
	tmpl := `AWSTemplateFormatVersion: '2010-09-09'
Description: demo
Parameters:
  Env:
    Type: String
Resources:
  Bucket:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: x
  Queue:
    Type: AWS::SQS::Queue
Outputs:
  BucketName:
    Value: !Ref Bucket
`
	valid := jsonOf(t, call(t, s, "validate_template", map[string]any{"template": tmpl})).(map[string]any)
	if valid["valid"] != true {
		t.Fatalf("template should be valid: %+v", valid)
	}
	resources := jsonOf(t, call(t, s, "list_resources", map[string]any{"template": tmpl})).([]cfnResource)
	if len(resources) != 2 || resources[0].LogicalID != "Bucket" || resources[0].Type != "AWS::S3::Bucket" {
		t.Fatalf("resources wrong: %+v", resources)
	}
	summary := jsonOf(t, call(t, s, "summarize_template", map[string]any{"template": tmpl})).(map[string]any)
	if summary["resources"].(int) != 2 {
		t.Fatalf("summary wrong: %+v", summary)
	}
	// A template missing Resources is invalid.
	bad := jsonOf(t, call(t, s, "validate_template", map[string]any{"template": "Description: nope\n"})).(map[string]any)
	if bad["valid"] != false {
		t.Fatalf("template without Resources should be invalid: %+v", bad)
	}
	// A resource without a Type is flagged.
	noType := jsonOf(t, call(t, s, "validate_template", map[string]any{"template": "Resources:\n  X:\n    Properties: {}\n"})).(map[string]any)
	if noType["valid"] != false {
		t.Fatalf("resource without Type should be invalid: %+v", noType)
	}
	if _, err := s.CallTool(context.Background(), "unknown", map[string]any{"template": "x"}); err == nil {
		t.Fatal("unknown tool should error")
	}
}

func TestMermaidServer(t *testing.T) {
	s := newMermaidServer()
	fc := call(t, s, "flowchart", map[string]any{"steps": "Start|Middle|End", "direction": "LR"}).Content[0].Text
	if !strings.HasPrefix(fc, "flowchart LR") || !strings.Contains(fc, "n0 --> n1") {
		t.Fatalf("flowchart wrong:\n%s", fc)
	}
	seq := call(t, s, "sequence", map[string]any{"messages": "A->>B: hi|B->>A: yo"}).Content[0].Text
	if !strings.HasPrefix(seq, "sequenceDiagram") {
		t.Fatalf("sequence wrong:\n%s", seq)
	}
	ok := jsonOf(t, call(t, s, "validate", map[string]any{"source": "graph TD\n A-->B"})).(map[string]any)
	if ok["valid"] != true {
		t.Fatalf("valid diagram rejected: %+v", ok)
	}
	if r := call(t, s, "validate", map[string]any{"source": "not a diagram"}); !r.IsError {
		t.Fatal("bad diagram should be a tool error")
	}
	if r := call(t, s, "flowchart", map[string]any{"steps": "", "direction": "TD"}); !r.IsError {
		t.Fatal("empty steps should be a tool error")
	}
	if _, err := s.CallTool(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown tool should error")
	}
}

func TestAWSServer(t *testing.T) {
	s := newAWSServer(AWSMock{
		Buckets:    []string{"z", "a"},
		Stacks:     map[string]string{"prod": "CREATE_COMPLETE"},
		Parameters: map[string]string{"/db/url": "postgres://x"},
	})
	buckets := jsonOf(t, call(t, s, "list_buckets", nil)).(map[string]any)
	if buckets["region"] != "us-east-1" {
		t.Fatalf("default region wrong: %+v", buckets)
	}
	if bs := buckets["buckets"].([]string); bs[0] != "a" {
		t.Fatalf("buckets not sorted: %+v", bs)
	}
	stack := jsonOf(t, call(t, s, "describe_stack", map[string]any{"name": "prod"})).(map[string]any)
	if stack["status"] != "CREATE_COMPLETE" {
		t.Fatalf("stack wrong: %+v", stack)
	}
	param := jsonOf(t, call(t, s, "get_parameter", map[string]any{"name": "/db/url"})).(map[string]any)
	if param["value"] != "postgres://x" {
		t.Fatalf("param wrong: %+v", param)
	}
	if r := call(t, s, "describe_stack", map[string]any{"name": "ghost"}); !r.IsError {
		t.Fatal("missing stack should be a tool error")
	}
	if r := call(t, s, "get_parameter", map[string]any{"name": "ghost"}); !r.IsError {
		t.Fatal("missing param should be a tool error")
	}
	if _, err := s.CallTool(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown tool should error")
	}
}

func TestPublishingServer(t *testing.T) {
	s := newPublishingServer(DefaultPublishPlatforms)
	platforms := jsonOf(t, call(t, s, "list_platforms", nil)).([]map[string]any)
	if len(platforms) != 4 || platforms[0]["name"] != "blog" {
		t.Fatalf("platforms wrong: %+v", platforms)
	}
	ok := jsonOf(t, call(t, s, "validate_post", map[string]any{"platform": "x", "content": "short"})).(map[string]any)
	if ok["valid"] != true {
		t.Fatalf("valid post rejected: %+v", ok)
	}
	// Over the limit for x (280).
	long := strings.Repeat("a", 300)
	if r := call(t, s, "validate_post", map[string]any{"platform": "x", "content": long}); !r.IsError {
		t.Fatal("over-limit content should be a tool error")
	}
	if r := call(t, s, "validate_post", map[string]any{"platform": "ghost", "content": "x"}); !r.IsError {
		t.Fatal("unknown platform should be a tool error")
	}
	if r := call(t, s, "validate_post", map[string]any{"platform": "x", "content": ""}); !r.IsError {
		t.Fatal("empty content should be a tool error")
	}
	receipt := jsonOf(t, call(t, s, "publish_post", map[string]any{"platform": "blog", "content": "hello"})).(map[string]any)
	if receipt["dryRun"] != true || receipt["contentId"] == "" {
		t.Fatalf("receipt wrong: %+v", receipt)
	}
	if _, err := s.CallTool(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown tool should error")
	}
}

func TestContextServer(t *testing.T) {
	s := newContextServer([]ContextDoc{
		{URI: "context://blog", Name: "blog", Title: "Blog", Body: "# Post"},
	})
	if !s.Capabilities().Resources {
		t.Fatal("context server should advertise resources")
	}
	list := jsonOf(t, call(t, s, "list_context", nil)).([]map[string]any)
	if len(list) != 1 || list[0]["uri"] != "context://blog" {
		t.Fatalf("list_context wrong: %+v", list)
	}
	if got := call(t, s, "get_context", map[string]any{"uri": "context://blog"}).Content[0].Text; got != "# Post" {
		t.Fatalf("get_context = %q", got)
	}
	if r := call(t, s, "get_context", map[string]any{"uri": "context://missing"}); !r.IsError {
		t.Fatal("missing context should be a tool error")
	}
	// MimeType defaults to text/plain.
	res, _ := s.ListResources(context.Background())
	if res[0].MimeType != "text/plain" {
		t.Fatalf("default mime wrong: %+v", res[0])
	}
	if _, err := s.CallTool(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown tool should error")
	}
}
