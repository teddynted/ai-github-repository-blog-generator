package mcp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestBaseServerDefaults(t *testing.T) {
	var b BaseServer
	ctx := context.Background()
	if res, err := b.ListResources(ctx); err != nil || res != nil {
		t.Fatalf("ListResources default wrong: %v %v", res, err)
	}
	if _, err := b.ReadResource(ctx, "x"); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("ReadResource default wrong: %v", err)
	}
	if ps, err := b.ListPrompts(ctx); err != nil || ps != nil {
		t.Fatalf("ListPrompts default wrong: %v %v", ps, err)
	}
	if _, err := b.GetPrompt(ctx, "x", nil); !errors.Is(err, ErrPromptNotFound) {
		t.Fatalf("GetPrompt default wrong: %v", err)
	}
	if h := b.Health(ctx); !h.Healthy() {
		t.Fatalf("Health default should be ok: %+v", h)
	}
}

func TestHealthHealthy(t *testing.T) {
	if !(Health{Status: HealthOK}).Healthy() {
		t.Fatal("ok should be healthy")
	}
	if (Health{Status: HealthDown}).Healthy() {
		t.Fatal("down should not be healthy")
	}
}

func TestArgHelpers(t *testing.T) {
	args := map[string]any{"s": "v", "b1": true, "b2": "true", "n": 42}
	if argString(args, "s") != "v" {
		t.Fatal("argString")
	}
	if argString(args, "n") != "42" {
		t.Fatal("argString non-string fallback")
	}
	if argString(args, "missing") != "" {
		t.Fatal("argString missing")
	}
	if !argBool(args, "b1") || !argBool(args, "b2") || argBool(args, "s") {
		t.Fatal("argBool")
	}
}

func TestValidateTypeCoercion(t *testing.T) {
	specs := []ParamSpec{
		{Name: "flag", Type: TypeBoolean},
		{Name: "count", Type: TypeInteger},
		{Name: "ratio", Type: TypeNumber},
	}
	// Valid mixed forms.
	if _, err := validate(specs, map[string]any{"flag": true, "count": 3, "ratio": 1.5}); err != nil {
		t.Fatalf("native types should validate: %v", err)
	}
	if _, err := validate(specs, map[string]any{"flag": "true", "count": "3", "ratio": "1.5"}); err != nil {
		t.Fatalf("string-encoded types should validate: %v", err)
	}
	// Invalid forms.
	if _, err := validate(specs, map[string]any{"flag": "notbool"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal("bad bool should fail")
	}
	if _, err := validate(specs, map[string]any{"count": "x"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal("bad int should fail")
	}
	if _, err := validate(specs, map[string]any{"flag": 3}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal("int for bool should fail")
	}
	if _, err := validate(specs, map[string]any{"count": true}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal("bool for int should fail")
	}
	// Default applied for absent optional param.
	out, err := validate([]ParamSpec{{Name: "x", Type: TypeString, Default: "d"}}, map[string]any{})
	if err != nil || out["x"] != "d" {
		t.Fatalf("default not applied: %v %v", out, err)
	}
}

func TestSlogAuditLogsWithoutSecret(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	a := SlogAudit{Logger: logger}
	a.Log(context.Background(), AuditEntry{
		Caller: "c", Server: "s", Operation: "call_tool", Tool: "t",
		Resource: "r", Permission: PermReadOnly, Outcome: "ok", DurationMs: 5, Error: "boom",
	})
	out := buf.String()
	for _, want := range []string{"caller=c", "server=s", "tool=t", "resource=r", "permission=read-only", "error=boom"} {
		if !strings.Contains(out, want) {
			t.Fatalf("audit log missing %q in: %s", want, out)
		}
	}
	// Default logger path (no panic).
	SlogAudit{}.Log(context.Background(), AuditEntry{Caller: "c"})
	NopAudit{}.Log(context.Background(), AuditEntry{})
}

func TestAuditOutcome(t *testing.T) {
	cases := map[error]string{
		nil:                "ok",
		ErrForbidden:       "denied",
		ErrUnauthenticated: "denied",
		ErrTimeout:         "timeout",
		ErrToolNotFound:    "error",
	}
	for err, want := range cases {
		if got := auditOutcome(err); got != want {
			t.Errorf("auditOutcome(%v)=%s want %s", err, got, want)
		}
	}
}

func TestRegistryPackageHelpers(t *testing.T) {
	// MustRegister on a fresh registry succeeds; a duplicate panics.
	reg := NewRegistry()
	reg.MustRegister(mermaidDescriptor(), func() (Server, error) { return newMermaidServer(), nil })
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("MustRegister should panic on duplicate")
			}
		}()
		reg.MustRegister(mermaidDescriptor(), func() (Server, error) { return newMermaidServer(), nil })
	}()

	// Package-level Register/MustRegister target DefaultRegistry.
	d := ServerDescriptor{ID: "extras-probe", RequiredPermission: PermReadOnly}
	if err := Register(d, func() (Server, error) { return newMermaidServer(), nil }); err != nil {
		t.Fatalf("package Register: %v", err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("package MustRegister should panic on duplicate")
			}
		}()
		MustRegister(d, func() (Server, error) { return newMermaidServer(), nil })
	}()
}

func TestTransport(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(mermaidDescriptor(), func() (Server, error) { return newMermaidServer(), nil })
	tr := NewInProcessTransport(reg)
	if tr.Name() != "in-process" {
		t.Fatalf("Name = %q", tr.Name())
	}
	srv, err := tr.Connect(context.Background(), mermaidDescriptor())
	if err != nil || srv == nil {
		t.Fatalf("Connect: %v", err)
	}
	// nil registry falls back to DefaultRegistry.
	if NewInProcessTransport(nil).Registry != DefaultRegistry {
		t.Fatal("nil registry should default to DefaultRegistry")
	}
}

func TestConfigNormalizeDefaults(t *testing.T) {
	c := Config{}.normalize()
	if c.Caller != "anonymous" || c.CallTimeout != 30*time.Second || c.MaxRetries != 2 || c.RetryBackoff != 100*time.Millisecond {
		t.Fatalf("defaults wrong: %+v", c)
	}
	// Negative retries clamp to the default.
	if got := (Config{MaxRetries: -5}).normalize().MaxRetries; got != 2 {
		t.Fatalf("negative retries not clamped: %d", got)
	}
}

func TestCredentialString(t *testing.T) {
	if got := NewCredential(AuthBearer, "").String(); !strings.Contains(got, "empty") {
		t.Fatalf("empty credential string wrong: %q", got)
	}
	if got := NewCredential(AuthBearer, "x").GoString(); !strings.Contains(got, "redacted") {
		t.Fatalf("GoString should redact: %q", got)
	}
}

// slowServer blocks until its context is cancelled, to exercise the timeout path.
type slowServer struct{ BaseServer }

func (slowServer) Info() ServerInfo           { return ServerInfo{Name: "slow"} }
func (slowServer) Capabilities() Capabilities { return Capabilities{Tools: true} }
func (slowServer) Category() Category         { return CategoryRepository }
func (slowServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{{Name: "wait", Permission: PermReadOnly}}, nil
}
func (slowServer) CallTool(ctx context.Context, _ string, _ map[string]any) (ToolResult, error) {
	<-ctx.Done()
	return ToolResult{}, ctx.Err()
}

func TestCallTimeout(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(ServerDescriptor{ID: "slow", RequiredPermission: PermReadOnly}, func() (Server, error) { return slowServer{}, nil })
	c := NewClient(reg, Config{Caller: "t", CallTimeout: 20 * time.Millisecond, MaxRetries: 1, RetryBackoff: time.Millisecond},
		WithAuthorizer(AllowAllAuthorizer{}))
	_, err := c.CallTool(context.Background(), "slow", "wait", nil)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("slow call should time out, got %v", err)
	}
}

func TestReadResourceAudited(t *testing.T) {
	reg := newTestRegistry(t)
	rec := &RecordingAudit{}
	c := NewClient(reg, Config{Caller: "tester"},
		WithAuthorizer(NewPolicyAuthorizer(map[string]Permission{"tester": PermReadOnly})),
		WithAudit(rec))
	if _, err := c.ReadResource(context.Background(), "context", "context://blog"); err != nil {
		t.Fatal(err)
	}
	if len(rec.Entries) != 1 || rec.Entries[0].Operation != "read_resource" || rec.Entries[0].Resource != "context://blog" {
		t.Fatalf("read_resource not audited: %+v", rec.Entries)
	}
}

func TestListResourcesForbidden(t *testing.T) {
	reg := newTestRegistry(t)
	c := NewClient(reg, Config{Caller: "nobody"}, WithAuthorizer(NewPolicyAuthorizer(nil)))
	if _, err := c.ListResources(context.Background(), "context"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unauthorized ListResources should be forbidden, got %v", err)
	}
}

func TestRecoverable(t *testing.T) {
	if !recoverable(&InvocationError{Recoverable: true, Err: errors.New("x")}) {
		t.Fatal("recoverable InvocationError")
	}
	if recoverable(&InvocationError{Recoverable: false, Err: errors.New("x")}) {
		t.Fatal("non-recoverable InvocationError")
	}
	if !recoverable(ErrRateLimited) {
		t.Fatal("rate-limited is recoverable")
	}
	if recoverable(ErrToolNotFound) {
		t.Fatal("not-found is not recoverable")
	}
	// InvocationError string + unwrap.
	ie := &InvocationError{Server: "s", Tool: "t", Err: ErrTimeout}
	if !strings.Contains(ie.Error(), "s/t") || !errors.Is(ie, ErrTimeout) {
		t.Fatalf("InvocationError format/unwrap wrong: %v", ie)
	}
}

func TestDisabledAuditUsesNop(t *testing.T) {
	c := NewClient(newTestRegistry(t), Config{Caller: "t", AuditEnabled: false},
		WithAuthorizer(AllowAllAuthorizer{}))
	if _, ok := c.audit.(NopAudit); !ok {
		t.Fatalf("disabled audit should be NopAudit, got %T", c.audit)
	}
	// Enabled → SlogAudit default.
	c2 := NewClient(newTestRegistry(t), Config{Caller: "t", AuditEnabled: true},
		WithAuthorizer(AllowAllAuthorizer{}))
	if _, ok := c2.audit.(SlogAudit); !ok {
		t.Fatalf("enabled audit should be SlogAudit, got %T", c2.audit)
	}
}

func TestParseIntDefault(t *testing.T) {
	if parseIntDefault("", 7) != 7 || parseIntDefault("bad", 7) != 7 || parseIntDefault("3", 7) != 3 {
		t.Fatal("parseIntDefault")
	}
}

// TestAllServerAccessors exercises Info/Capabilities/Category/ListTools on every
// built-in server through a live registry, covering the descriptor accessors.
func TestAllServerAccessors(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterDefaults(reg); err != nil {
		t.Fatal(err)
	}
	for _, d := range reg.Descriptors() {
		srv, err := reg.Instance(d.ID)
		if err != nil {
			t.Fatalf("%s: instance: %v", d.ID, err)
		}
		if srv.Info().Name != d.ID {
			t.Errorf("%s: Info().Name = %q", d.ID, srv.Info().Name)
		}
		if srv.Category() != d.Category {
			t.Errorf("%s: Category mismatch", d.ID)
		}
		if !srv.Capabilities().Tools {
			t.Errorf("%s: should advertise tools", d.ID)
		}
		tools, err := srv.ListTools(context.Background())
		if err != nil || len(tools) == 0 {
			t.Errorf("%s: ListTools: %v (%d)", d.ID, err, len(tools))
		}
		if h := srv.Health(context.Background()); !h.Healthy() {
			t.Errorf("%s: default health should be ok", d.ID)
		}
	}
}

func TestAuthProviderEdges(t *testing.T) {
	ctx := context.Background()
	// NoAuth yields a none credential.
	if cred, _ := (NoAuth{}).Credential(ctx, "x"); cred.Kind != AuthNone {
		t.Fatal("NoAuth kind")
	}
	// NewEnvAuth applies defaults.
	e := NewEnvAuth("", "")
	if e.Prefix != "MCP_" || e.Kind != AuthBearer {
		t.Fatalf("NewEnvAuth defaults wrong: %+v", e)
	}
	// MapAuth default kind is bearer.
	if cred, _ := (MapAuth{Secrets: map[string]string{"s": "v"}}).Credential(ctx, "s"); cred.Kind != AuthBearer || cred.Value() != "v" {
		t.Fatalf("MapAuth default kind wrong: %+v", cred)
	}
	// SecretsManagerAuth with nil resolver returns a none credential.
	if cred, err := (SecretsManagerAuth{}).Credential(ctx, "s"); err != nil || cred.Kind != AuthNone {
		t.Fatalf("nil resolver should yield none credential: %+v %v", cred, err)
	}
	// SecretsManagerAuth propagates resolver errors.
	failing := SecretsManagerAuth{Resolver: secretResolverFunc(func(context.Context, string) (string, error) {
		return "", errors.New("boom")
	})}
	if _, err := failing.Credential(ctx, "s"); err == nil {
		t.Fatal("resolver error should propagate")
	}
}

func TestWithTransportOption(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(mermaidDescriptor(), func() (Server, error) { return newMermaidServer(), nil })
	c := NewClient(reg, Config{Caller: "t"},
		WithAuthorizer(AllowAllAuthorizer{}),
		WithTransport(NewInProcessTransport(reg)))
	if _, err := c.ListTools(context.Background(), "mermaid"); err != nil {
		t.Fatalf("custom transport should work: %v", err)
	}
}
