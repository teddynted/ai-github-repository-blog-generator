package mcp

import (
	"context"
	"time"
)

// Server is the port every MCP server implements. It is the single seam between
// the client and any tool provider: repositories, filesystems, databases,
// CloudFormation, AWS, publishing, etc. A server that does not support resources
// or prompts returns empty slices (embed BaseServer for the no-op defaults), so
// each server implements only what it exposes.
type Server interface {
	// Info identifies the server + protocol.
	Info() ServerInfo
	// Capabilities advertises supported MCP features.
	Capabilities() Capabilities
	// Category groups the server for discovery + policy.
	Category() Category
	// Health reports availability (drives selection + monitoring).
	Health(ctx context.Context) Health

	// ListTools discovers the server's callable tools.
	ListTools(ctx context.Context) ([]Tool, error)
	// CallTool executes a tool with validated arguments.
	CallTool(ctx context.Context, name string, args map[string]any) (ToolResult, error)

	// ListResources discovers readable resources (empty if unsupported).
	ListResources(ctx context.Context) ([]Resource, error)
	// ReadResource returns a resource's content by URI.
	ReadResource(ctx context.Context, uri string) (ResourceContent, error)

	// ListPrompts discovers prompt templates (empty if unsupported).
	ListPrompts(ctx context.Context) ([]Prompt, error)
	// GetPrompt renders a prompt with arguments.
	GetPrompt(ctx context.Context, name string, args map[string]string) ([]PromptMessage, error)
}

// Transport resolves a registered server descriptor to a live Server. The
// in-process transport returns the local instance; a future stdio/HTTP transport
// returns a proxy Server that speaks MCP over the wire — with no change to the
// client, which depends only on this port.
type Transport interface {
	// Connect returns a live Server for a descriptor (may be a proxy).
	Connect(ctx context.Context, d ServerDescriptor) (Server, error)
	// Name identifies the transport (in-process | stdio | http).
	Name() string
}

// Credential is a resolved, opaque secret for authenticating to a server. Its
// value is never logged, serialized, or embedded in errors — only its Kind and
// presence are observable.
type Credential struct {
	Kind  AuthKind // bearer | api-key | iam | none
	value string   // unexported → cannot be marshaled/logged accidentally
}

// NewCredential constructs a credential (tests + adapters).
func NewCredential(kind AuthKind, value string) Credential {
	return Credential{Kind: kind, value: value}
}

// Value returns the secret. Callers must never log it.
func (c Credential) Value() string { return c.value }

// Present reports whether a secret was resolved.
func (c Credential) Present() bool { return c.value != "" }

// String redacts the secret so it cannot leak through %v/%s formatting.
func (c Credential) String() string {
	if c.value == "" {
		return "Credential{" + string(c.Kind) + ", <empty>}"
	}
	return "Credential{" + string(c.Kind) + ", <redacted>}"
}

// GoString redacts the secret under %#v as well (fmt would otherwise dump the
// unexported field via reflection).
func (c Credential) GoString() string { return c.String() }

// AuthProvider resolves the credential for a server. Implementations read from
// the environment, AWS Secrets Manager, or an in-memory map (tests) — never from
// hard-coded values. A server with AuthKindNone needs no credential.
type AuthProvider interface {
	Credential(ctx context.Context, serverID string) (Credential, error)
}

// Authorizer enforces fine-grained, tool-scoped permission policies. It is
// consulted before every tool call / resource read, so least-privilege is the
// default: a caller must hold at least the required permission.
type Authorizer interface {
	Authorize(ctx context.Context, caller, serverID string, required Permission) error
}

// AuditLogger records every MCP operation for the audit trail. The CloudWatch
// adapter ships by default; a no-op adapter is used when auditing is disabled.
type AuditLogger interface {
	Log(ctx context.Context, e AuditEntry)
}

// Clock is the time port (deterministic in tests).
type Clock func() time.Time
