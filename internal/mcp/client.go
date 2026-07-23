package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Client is the single entry point callers use to talk to MCP servers. It layers
// the cross-cutting concerns every invocation needs — discovery, authentication,
// authorization, input validation, timeout, retry, rate limiting, and audit —
// around whatever Transport/Registry it is given, so individual servers stay
// free of policy. It is safe for concurrent use.
type Client struct {
	registry   *Registry
	transport  Transport
	auth       AuthProvider
	authorizer Authorizer
	audit      AuditLogger
	clock      Clock
	cfg        Config
	limiter    *rateLimiter
}

// Option configures a Client.
type Option func(*Client)

// WithTransport overrides the transport (default: in-process over the registry).
func WithTransport(t Transport) Option { return func(c *Client) { c.transport = t } }

// WithAuth sets the credential provider (default: NoAuth).
func WithAuth(a AuthProvider) Option { return func(c *Client) { c.auth = a } }

// WithAuthorizer sets the authorizer (default: deny-by-default PolicyAuthorizer
// with no grants — callers must opt in explicitly).
func WithAuthorizer(a Authorizer) Option { return func(c *Client) { c.authorizer = a } }

// WithAudit sets the audit logger (default: SlogAudit, or NopAudit if disabled).
func WithAudit(a AuditLogger) Option { return func(c *Client) { c.audit = a } }

// WithClock injects a clock (tests).
func WithClock(clk Clock) Option { return func(c *Client) { c.clock = clk } }

// NewClient builds a Client over a registry with the given config and options.
func NewClient(registry *Registry, cfg Config, opts ...Option) *Client {
	if registry == nil {
		registry = DefaultRegistry
	}
	c := &Client{
		registry: registry,
		cfg:      cfg.normalize(),
	}
	for _, o := range opts {
		o(c)
	}
	if c.transport == nil {
		c.transport = NewInProcessTransport(registry)
	}
	if c.auth == nil {
		c.auth = NoAuth{}
	}
	if c.authorizer == nil {
		c.authorizer = NewPolicyAuthorizer(nil)
	}
	if c.audit == nil {
		if c.cfg.AuditEnabled {
			c.audit = SlogAudit{}
		} else {
			c.audit = NopAudit{}
		}
	}
	if c.clock == nil {
		c.clock = time.Now
	}
	c.limiter = newRateLimiter(c.cfg.RatePerMinute, func() time.Time { return c.clock() })
	return c
}

// Servers lists the descriptors this caller is allowed to see (least privilege).
func (c *Client) Servers() []ServerDescriptor {
	all := c.registry.Descriptors()
	out := all[:0:0]
	for _, d := range all {
		if c.cfg.serverAllowed(d.ID) {
			out = append(out, d)
		}
	}
	return out
}

// connect resolves an id to a live Server, enforcing the allow-list.
func (c *Client) connect(ctx context.Context, id string) (Server, ServerDescriptor, error) {
	if !c.cfg.serverAllowed(id) {
		return nil, ServerDescriptor{}, fmt.Errorf("%w: %q not in allow-list", ErrForbidden, id)
	}
	d, ok := c.registry.Descriptor(id)
	if !ok {
		return nil, ServerDescriptor{}, fmt.Errorf("%w: %q", ErrServerNotFound, id)
	}
	srv, err := c.transport.Connect(ctx, d)
	if err != nil {
		return nil, d, err
	}
	return srv, d, nil
}

// ListTools returns a server's tools (sorted), after authorization at the
// server's baseline permission.
func (c *Client) ListTools(ctx context.Context, serverID string) ([]Tool, error) {
	srv, d, err := c.connect(ctx, serverID)
	if err != nil {
		return nil, err
	}
	if err := c.authorize(ctx, d, d.RequiredPermission); err != nil {
		c.record(ctx, serverID, "list_tools", "", "", d.RequiredPermission, 0, err)
		return nil, err
	}
	tools, err := srv.ListTools(ctx)
	if err == nil {
		sortTools(tools)
	}
	return tools, err
}

// ListResources returns a server's resources (sorted).
func (c *Client) ListResources(ctx context.Context, serverID string) ([]Resource, error) {
	srv, d, err := c.connect(ctx, serverID)
	if err != nil {
		return nil, err
	}
	if err := c.authorize(ctx, d, PermReadOnly); err != nil {
		c.record(ctx, serverID, "list_resources", "", "", PermReadOnly, 0, err)
		return nil, err
	}
	res, err := srv.ListResources(ctx)
	if err == nil {
		sortResources(res)
	}
	return res, err
}

// ReadResource reads one resource by URI, enforcing read authorization + audit.
func (c *Client) ReadResource(ctx context.Context, serverID, uri string) (ResourceContent, error) {
	start := c.clock()
	srv, d, err := c.connect(ctx, serverID)
	if err != nil {
		return ResourceContent{}, err
	}
	required := PermReadOnly
	if err := c.authorize(ctx, d, required); err != nil {
		c.record(ctx, serverID, "read_resource", "", uri, required, 0, err)
		return ResourceContent{}, err
	}
	if _, err := c.credential(ctx, d); err != nil {
		c.record(ctx, serverID, "read_resource", "", uri, required, 0, err)
		return ResourceContent{}, err
	}
	rc, err := srv.ReadResource(ctx, uri)
	c.record(ctx, serverID, "read_resource", "", uri, required, sinceMs(start, c.clock), err)
	return rc, err
}

// CallTool is the primary operation: discover → authorize → authenticate →
// validate → rate-limit → invoke (with timeout + retry) → audit.
func (c *Client) CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (ToolResult, error) {
	start := c.clock()
	srv, d, err := c.connect(ctx, serverID)
	if err != nil {
		return ToolResult{}, err
	}

	tool, err := c.findTool(ctx, srv, toolName)
	if err != nil {
		c.record(ctx, serverID, "call_tool", toolName, "", "", 0, err)
		return ToolResult{}, err
	}

	required := tool.Permission
	if rank[required] < rank[d.RequiredPermission] {
		required = d.RequiredPermission
	}
	if err := c.authorize(ctx, d, required); err != nil {
		c.record(ctx, serverID, "call_tool", toolName, "", required, 0, err)
		return ToolResult{}, err
	}

	if _, err := c.credential(ctx, d); err != nil {
		c.record(ctx, serverID, "call_tool", toolName, "", required, 0, err)
		return ToolResult{}, err
	}

	if !c.limiter.allow(c.cfg.Caller) {
		err := fmt.Errorf("%w: %s", ErrRateLimited, c.cfg.Caller)
		c.record(ctx, serverID, "call_tool", toolName, "", required, 0, err)
		return ToolResult{}, err
	}

	normArgs, err := validate(tool.Params, args)
	if err != nil {
		c.record(ctx, serverID, "call_tool", toolName, "", required, 0, err)
		return ToolResult{}, err
	}

	res, err := c.invoke(ctx, srv, serverID, toolName, normArgs)
	c.record(ctx, serverID, "call_tool", toolName, "", required, sinceMs(start, c.clock), err)
	return res, err
}

// invoke runs a tool call under a per-call timeout, retrying recoverable faults
// with linear backoff.
func (c *Client) invoke(ctx context.Context, srv Server, serverID, toolName string, args map[string]any) (ToolResult, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ToolResult{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * c.cfg.RetryBackoff):
			}
		}
		callCtx, cancel := context.WithTimeout(ctx, c.cfg.CallTimeout)
		res, err := srv.CallTool(callCtx, toolName, args)
		cancel()
		if err == nil {
			return res, nil
		}
		if ctx.Err() == nil && isDeadline(err) {
			err = fmt.Errorf("%w: %s/%s", ErrTimeout, serverID, toolName)
		}
		lastErr = &InvocationError{Server: serverID, Tool: toolName, Recoverable: recoverable(err), Err: err}
		if !recoverable(err) {
			return ToolResult{}, lastErr
		}
	}
	return ToolResult{}, lastErr
}

// findTool locates a tool by name on a server.
func (c *Client) findTool(ctx context.Context, srv Server, name string) (Tool, error) {
	tools, err := srv.ListTools(ctx)
	if err != nil {
		return Tool{}, err
	}
	for _, t := range tools {
		if t.Name == name {
			return t, nil
		}
	}
	return Tool{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
}

// authorize checks the caller against the required permission.
func (c *Client) authorize(ctx context.Context, d ServerDescriptor, required Permission) error {
	return c.authorizer.Authorize(ctx, c.cfg.Caller, d.ID, required)
}

// credential resolves (but never logs) the server credential, enforcing that a
// server requiring auth actually has one available.
func (c *Client) credential(ctx context.Context, d ServerDescriptor) (Credential, error) {
	cred, err := c.auth.Credential(ctx, d.ID)
	if err != nil {
		return Credential{}, err
	}
	if cred.Kind != AuthNone && cred.Kind != "" && !cred.Present() {
		return Credential{}, fmt.Errorf("%w: %q requires a %s credential", ErrUnauthenticated, d.ID, cred.Kind)
	}
	return cred, nil
}

// record emits an audit entry for an operation.
func (c *Client) record(ctx context.Context, server, op, tool, resource string, perm Permission, durMs int64, err error) {
	e := AuditEntry{
		Timestamp:  nowOr(c.clock),
		Caller:     c.cfg.Caller,
		Server:     server,
		Operation:  op,
		Tool:       tool,
		Resource:   resource,
		Permission: perm,
		Outcome:    auditOutcome(err),
		DurationMs: durMs,
	}
	if err != nil {
		e.Error = err.Error()
	}
	c.audit.Log(ctx, e)
}

func sinceMs(start time.Time, now Clock) int64 {
	return now().Sub(start).Milliseconds()
}

func isDeadline(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
