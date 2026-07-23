package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// --- test helpers -----------------------------------------------------------

// fixedClock returns a deterministic, advancing clock for audit/rate tests.
func fixedClock(start time.Time, step time.Duration) Clock {
	t := start
	return func() time.Time {
		cur := t
		t = t.Add(step)
		return cur
	}
}

// newTestRegistry builds an isolated registry with the default servers, seeding
// the data-backed ones so calls return deterministic results.
func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	must(reg.Register(repositoryDescriptor(), func() (Server, error) {
		return newRepositoryServer(RepoSnapshot{
			Name:          "demo",
			DefaultBranch: "main",
			Commits:       []RepoCommit{{SHA: "a1", Author: "t", Subject: "first"}, {SHA: "b2", Author: "t", Subject: "second"}},
			Tags:          []string{"v1.0.0", "v0.9.0"},
			Files:         []string{"main.go", "docs/x.md"},
		}), nil
	}))
	must(reg.Register(databaseDescriptor(), func() (Server, error) {
		return newDatabaseServer(DBSnapshot{
			Engine: "postgres",
			Tables: map[string]DBTable{
				"users": {Columns: []string{"id", "name"}, Rows: []map[string]any{
					{"id": "1", "name": "ada"}, {"id": "2", "name": "grace"},
				}},
			},
		}), nil
	}))
	must(reg.Register(cloudformationDescriptor(), func() (Server, error) { return newCloudFormationServer(), nil }))
	must(reg.Register(mermaidDescriptor(), func() (Server, error) { return newMermaidServer(), nil }))
	must(reg.Register(awsDescriptor(), func() (Server, error) {
		return newAWSServer(AWSMock{Buckets: []string{"b1", "b2"}, Stacks: map[string]string{"prod": "CREATE_COMPLETE"}, Parameters: map[string]string{"/x": "y"}}), nil
	}))
	must(reg.Register(publishingDescriptor(), func() (Server, error) { return newPublishingServer(DefaultPublishPlatforms), nil }))
	must(reg.Register(contextDescriptor(), func() (Server, error) {
		return newContextServer([]ContextDoc{
			{URI: "context://blog", Name: "blog", Title: "Blog", MimeType: "text/markdown", Body: "# Post"},
			{URI: "context://release", Name: "release", Title: "Release", MimeType: "application/json", Body: `{"tag":"v1.0.0"}`},
		}), nil
	}))
	return reg
}

// adminClient builds a client whose caller holds admin (sees everything).
func adminClient(t *testing.T, reg *Registry, opts ...Option) *Client {
	t.Helper()
	base := []Option{
		WithAuthorizer(NewPolicyAuthorizer(map[string]Permission{"tester": PermAdmin})),
		WithAudit(&RecordingAudit{}),
		WithClock(fixedClock(time.Unix(0, 0).UTC(), time.Millisecond)),
	}
	return NewClient(reg, Config{Caller: "tester"}, append(base, opts...)...)
}

// --- registry ---------------------------------------------------------------

func TestRegistryRegisterAndInstance(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(mermaidDescriptor(), func() (Server, error) { return newMermaidServer(), nil }); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(mermaidDescriptor(), func() (Server, error) { return newMermaidServer(), nil }); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("want ErrAlreadyRegistered, got %v", err)
	}
	s1, err := reg.Instance("mermaid")
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := reg.Instance("mermaid")
	if s1 != s2 {
		t.Fatal("Instance should cache and return the same server")
	}
	if _, err := reg.Instance("nope"); !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("want ErrServerNotFound, got %v", err)
	}
}

func TestRegistryRejectsBadRegistration(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(ServerDescriptor{}, func() (Server, error) { return nil, nil }); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("empty id should be rejected, got %v", err)
	}
	if err := reg.Register(mermaidDescriptor(), nil); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("nil factory should be rejected, got %v", err)
	}
}

func TestRegisterDefaultsPopulatesAllServers(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterDefaults(reg); err != nil {
		t.Fatal(err)
	}
	want := []string{"aws", "cloudformation", "context", "database", "documentation", "filesystem", "mermaid", "publishing", "repository"}
	got := reg.Descriptors()
	if len(got) != len(want) {
		t.Fatalf("want %d servers, got %d", len(want), len(got))
	}
	for i, d := range got {
		if d.ID != want[i] {
			t.Errorf("descriptor %d: want %s, got %s", i, want[i], d.ID)
		}
	}
	if err := RegisterDefaults(reg); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("second RegisterDefaults should fail loud, got %v", err)
	}
}

func TestDefaultRegistryHasBuiltins(t *testing.T) {
	if _, ok := DefaultRegistry.Descriptor("context"); !ok {
		t.Fatal("package init should register built-ins into DefaultRegistry")
	}
}

// --- discovery --------------------------------------------------------------

func TestClientServersRespectAllowList(t *testing.T) {
	reg := newTestRegistry(t)
	c := NewClient(reg, Config{Caller: "tester", AllowedServers: []string{"mermaid"}},
		WithAuthorizer(AllowAllAuthorizer{}))
	servers := c.Servers()
	if len(servers) != 1 || servers[0].ID != "mermaid" {
		t.Fatalf("allow-list not enforced: %+v", servers)
	}
	if _, err := c.ListTools(context.Background(), "aws"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("disallowed server should be forbidden, got %v", err)
	}
}

func TestListToolsSorted(t *testing.T) {
	c := adminClient(t, newTestRegistry(t))
	tools, err := c.ListTools(context.Background(), "repository")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(tools); i++ {
		if tools[i-1].Name > tools[i].Name {
			t.Fatalf("tools not sorted: %s > %s", tools[i-1].Name, tools[i].Name)
		}
	}
}

// --- authorization ----------------------------------------------------------

func TestAuthorizationDenyByDefault(t *testing.T) {
	reg := newTestRegistry(t)
	// PolicyAuthorizer with no grants: caller holds nothing.
	c := NewClient(reg, Config{Caller: "stranger"}, WithAuthorizer(NewPolicyAuthorizer(nil)))
	if _, err := c.CallTool(context.Background(), "repository", "list_tags", nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("deny-by-default failed, got %v", err)
	}
}

func TestPublishRequiresPublishPermission(t *testing.T) {
	reg := newTestRegistry(t)
	// Reader can validate but not publish.
	reader := NewClient(reg, Config{Caller: "reader"},
		WithAuthorizer(NewPolicyAuthorizer(map[string]Permission{"reader": PermReadOnly})))
	args := map[string]any{"platform": "x", "content": "hello"}
	if _, err := reader.CallTool(context.Background(), "publishing", "validate_post", args); err != nil {
		t.Fatalf("reader should be able to validate: %v", err)
	}
	if _, err := reader.CallTool(context.Background(), "publishing", "publish_post", args); !errors.Is(err, ErrForbidden) {
		t.Fatalf("reader must not publish, got %v", err)
	}
	// Publisher can publish.
	pub := NewClient(reg, Config{Caller: "pub"},
		WithAuthorizer(NewPolicyAuthorizer(map[string]Permission{"pub": PermPublish})))
	res, err := pub.CallTool(context.Background(), "publishing", "publish_post", args)
	if err != nil {
		t.Fatalf("publisher should publish: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res)
	}
}

func TestGrantsLadder(t *testing.T) {
	cases := []struct {
		held, required Permission
		want           bool
	}{
		{PermAdmin, PermPublish, true},
		{PermReadWrite, PermReadOnly, true},
		{PermReadOnly, PermReadWrite, false},
		{PermReadOnly, PermReadOnly, true},
		{PermReadWrite, PermPublish, false},
		{PermNone, PermReadOnly, false},
		{PermPublish, PermReadOnly, false}, // publish is an orthogonal scope
	}
	for _, c := range cases {
		if got := grants(c.held, c.required); got != c.want {
			t.Errorf("grants(%s,%s)=%v want %v", c.held, c.required, got, c.want)
		}
	}
}

// --- authentication ---------------------------------------------------------

func TestMissingCredentialIsUnauthenticated(t *testing.T) {
	reg := NewRegistry()
	// A server descriptor is fine; auth provider yields an empty bearer token.
	_ = reg.Register(awsDescriptor(), func() (Server, error) { return newAWSServer(AWSMock{Buckets: []string{"b"}}), nil })
	c := NewClient(reg, Config{Caller: "t"},
		WithAuthorizer(AllowAllAuthorizer{}),
		WithAuth(MapAuth{Kind: AuthBearer, Secrets: map[string]string{}})) // no secret for "aws"
	if _, err := c.CallTool(context.Background(), "aws", "list_buckets", nil); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("missing credential should be ErrUnauthenticated, got %v", err)
	}
	// With a secret present, the call succeeds.
	c2 := NewClient(reg, Config{Caller: "t"},
		WithAuthorizer(AllowAllAuthorizer{}),
		WithAuth(MapAuth{Kind: AuthBearer, Secrets: map[string]string{"aws": "tok"}}))
	if _, err := c2.CallTool(context.Background(), "aws", "list_buckets", nil); err != nil {
		t.Fatalf("present credential should authenticate: %v", err)
	}
}

func TestEnvAuthName(t *testing.T) {
	e := NewEnvAuth("MCP_", AuthBearer)
	got := e.envName("github-repo")
	if got != "MCP_GITHUB_REPO_TOKEN" {
		t.Fatalf("envName = %q", got)
	}
	e.lookup = func(k string) string {
		if k == "MCP_GITHUB_REPO_TOKEN" {
			return "secret"
		}
		return ""
	}
	cred, _ := e.Credential(context.Background(), "github-repo")
	if !cred.Present() || cred.Value() != "secret" {
		t.Fatalf("credential not resolved from env")
	}
}

func TestSecretsManagerAuth(t *testing.T) {
	sr := secretResolverFunc(func(_ context.Context, id string) (string, error) {
		if id == "mcp/github" {
			return "tok", nil
		}
		return "", errors.New("not found")
	})
	a := SecretsManagerAuth{Resolver: sr}
	cred, err := a.Credential(context.Background(), "github")
	if err != nil || cred.Value() != "tok" {
		t.Fatalf("secrets-manager auth failed: %v %q", err, cred.Value())
	}
}

type secretResolverFunc func(context.Context, string) (string, error)

func (f secretResolverFunc) Resolve(ctx context.Context, id string) (string, error) {
	return f(ctx, id)
}

func TestCredentialNeverLeaksInString(t *testing.T) {
	cred := NewCredential(AuthBearer, "super-secret-token")
	if strings.Contains(fmt.Sprintf("%v %+v %#v", cred, cred, cred), "super-secret-token") {
		t.Fatal("credential value must not appear in its formatted form")
	}
}

// --- validation -------------------------------------------------------------

func TestInputValidation(t *testing.T) {
	c := adminClient(t, newTestRegistry(t))
	ctx := context.Background()
	// Missing required arg.
	if _, err := c.CallTool(ctx, "database", "describe_table", nil); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("missing required arg should fail, got %v", err)
	}
	// Unknown arg rejected.
	if _, err := c.CallTool(ctx, "database", "list_tables", map[string]any{"bogus": "x"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("unknown arg should be rejected, got %v", err)
	}
	// Enum violation.
	if _, err := c.CallTool(ctx, "mermaid", "flowchart", map[string]any{"steps": "a|b", "direction": "SIDEWAYS"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("enum violation should fail, got %v", err)
	}
	// Unknown tool.
	if _, err := c.CallTool(ctx, "mermaid", "nope", nil); !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("unknown tool should fail, got %v", err)
	}
	// Unknown server.
	if _, err := c.CallTool(ctx, "ghost", "x", nil); !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("unknown server should fail, got %v", err)
	}
}

// --- audit ------------------------------------------------------------------

func TestAuditRecordsOutcome(t *testing.T) {
	reg := newTestRegistry(t)
	rec := &RecordingAudit{}
	c := NewClient(reg, Config{Caller: "tester"},
		WithAuthorizer(NewPolicyAuthorizer(map[string]Permission{"tester": PermReadOnly})),
		WithAudit(rec),
		WithClock(fixedClock(time.Unix(0, 0).UTC(), time.Millisecond)))
	if _, err := c.CallTool(context.Background(), "repository", "list_tags", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CallTool(context.Background(), "publishing", "publish_post",
		map[string]any{"platform": "x", "content": "hi"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if len(rec.Entries) != 2 {
		t.Fatalf("want 2 audit entries, got %d", len(rec.Entries))
	}
	if rec.Entries[0].Outcome != "ok" || rec.Entries[0].Tool != "list_tags" {
		t.Errorf("first entry wrong: %+v", rec.Entries[0])
	}
	if rec.Entries[1].Outcome != "denied" {
		t.Errorf("denied outcome not recorded: %+v", rec.Entries[1])
	}
}

// --- rate limiting ----------------------------------------------------------

func TestRateLimiting(t *testing.T) {
	reg := newTestRegistry(t)
	c := NewClient(reg, Config{Caller: "tester", RatePerMinute: 2},
		WithAuthorizer(AllowAllAuthorizer{}),
		WithClock(fixedClock(time.Unix(0, 0).UTC(), 0))) // frozen clock: no window decay
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := c.CallTool(ctx, "repository", "list_tags", nil); err != nil {
			t.Fatalf("call %d should pass: %v", i, err)
		}
	}
	if _, err := c.CallTool(ctx, "repository", "list_tags", nil); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("third call should be rate-limited, got %v", err)
	}
}

// --- retry / recoverability -------------------------------------------------

// flakyServer fails a set number of times with a recoverable error, then succeeds.
type flakyServer struct {
	BaseServer
	failsLeft int
	recover   bool
}

func (f *flakyServer) Info() ServerInfo { return ServerInfo{Name: "flaky"} }
func (f *flakyServer) Capabilities() Capabilities {
	return Capabilities{Tools: true}
}
func (f *flakyServer) Category() Category { return CategoryRepository }
func (f *flakyServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{{Name: "go", Permission: PermReadOnly}}, nil
}
func (f *flakyServer) CallTool(context.Context, string, map[string]any) (ToolResult, error) {
	if f.failsLeft > 0 {
		f.failsLeft--
		return ToolResult{}, &InvocationError{Server: "flaky", Tool: "go", Recoverable: f.recover, Err: ErrTimeout}
	}
	return textResult("ok"), nil
}

func TestRetryRecoverable(t *testing.T) {
	reg := NewRegistry()
	fs := &flakyServer{failsLeft: 2, recover: true}
	_ = reg.Register(ServerDescriptor{ID: "flaky", RequiredPermission: PermReadOnly}, func() (Server, error) { return fs, nil })
	c := NewClient(reg, Config{Caller: "t", MaxRetries: 3, RetryBackoff: time.Millisecond},
		WithAuthorizer(AllowAllAuthorizer{}))
	res, err := c.CallTool(context.Background(), "flaky", "go", nil)
	if err != nil {
		t.Fatalf("recoverable error should be retried to success: %v", err)
	}
	if res.Content[0].Text != "ok" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestRetryGivesUpOnNonRecoverable(t *testing.T) {
	reg := NewRegistry()
	fs := &flakyServer{failsLeft: 5, recover: false}
	_ = reg.Register(ServerDescriptor{ID: "flaky", RequiredPermission: PermReadOnly}, func() (Server, error) { return fs, nil })
	c := NewClient(reg, Config{Caller: "t", MaxRetries: 3, RetryBackoff: time.Millisecond},
		WithAuthorizer(AllowAllAuthorizer{}))
	_, err := c.CallTool(context.Background(), "flaky", "go", nil)
	if err == nil {
		t.Fatal("non-recoverable error should not be retried to success")
	}
	if fs.failsLeft != 4 { // called exactly once
		t.Fatalf("non-recoverable error retried: failsLeft=%d", fs.failsLeft)
	}
}

// --- resources --------------------------------------------------------------

func TestResources(t *testing.T) {
	c := adminClient(t, newTestRegistry(t))
	ctx := context.Background()
	res, err := c.ListResources(ctx, "context")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].URI != "context://blog" {
		t.Fatalf("unexpected resources: %+v", res)
	}
	rc, err := c.ReadResource(ctx, "context", "context://release")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rc.Text, "v1.0.0") {
		t.Fatalf("resource body wrong: %q", rc.Text)
	}
	if _, err := c.ReadResource(ctx, "context", "context://missing"); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("missing resource should error, got %v", err)
	}
}

func TestServerWithoutResources(t *testing.T) {
	c := adminClient(t, newTestRegistry(t))
	// repository embeds BaseServer → empty resources, not an error.
	res, err := c.ListResources(context.Background(), "repository")
	if err != nil {
		t.Fatalf("BaseServer.ListResources should be a no-op: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected no resources, got %+v", res)
	}
}
