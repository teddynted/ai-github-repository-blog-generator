// Package mcp is the Model Context Protocol integration layer (Milestone 22). It
// standardizes how AI providers (Amazon Bedrock/Claude, Anthropic, and future
// models) interact with repositories, documentation, CloudFormation, databases,
// AWS resources, and publishing platforms — through discoverable MCP tools and
// resources instead of bespoke, per-integration code.
//
// It follows Clean Architecture and Ports & Adapters: the domain (this file) is
// pure; the client, registry, transport, auth, authz, and audit layers depend
// only on ports (Server, Transport, AuthProvider, Authorizer, AuditLogger,
// Clock); and each MCP server is an independent, self-registering plug-in, so new
// servers are added without touching existing code. It is secure-by-default
// (least-privilege authorization, credential isolation, input/output validation,
// full audit) and fully offline-testable (an in-process transport + in-memory
// servers need no network or credentials).
//
// The types below mirror the MCP specification's shapes (tools, resources,
// prompts, capabilities) in idiomatic Go, so an out-of-process stdio/HTTP
// transport can be added later behind the same Transport port without changing
// callers.
package mcp

import "time"

// ProtocolVersion is the MCP protocol revision this layer implements.
const ProtocolVersion = "2025-06-18"

// Category groups servers by the domain they expose (for discovery + policy).
type Category string

const (
	CategoryRepository     Category = "repository"
	CategoryFilesystem     Category = "filesystem"
	CategoryDatabase       Category = "database"
	CategoryCloudFormation Category = "cloudformation"
	CategoryMermaid        Category = "mermaid"
	CategoryDocumentation  Category = "documentation"
	CategoryAWS            Category = "aws"
	CategoryPublishing     Category = "publishing"
	CategoryContext        Category = "context"
)

// ServerInfo identifies a server and its protocol support (MCP `serverInfo`).
type ServerInfo struct {
	Name            string `json:"name"`
	Title           string `json:"title,omitempty"`
	Version         string `json:"version"`
	ProtocolVersion string `json:"protocolVersion"`
}

// Capabilities advertises which MCP features a server supports.
type Capabilities struct {
	Tools     bool `json:"tools"`
	Resources bool `json:"resources"`
	Prompts   bool `json:"prompts"`
	Logging   bool `json:"logging"`
}

// ServerDescriptor is the registry metadata for a server — what discovery and
// authorization reason over without instantiating the server.
type ServerDescriptor struct {
	ID                 string       `json:"id"`
	Info               ServerInfo   `json:"info"`
	Category           Category     `json:"category"`
	Capabilities       Capabilities `json:"capabilities"`
	RequiredPermission Permission   `json:"requiredPermission"` // minimum permission to use this server
	Description        string       `json:"description,omitempty"`
}

// ParamType is a tool parameter's JSON-schema-lite type.
type ParamType string

const (
	TypeString  ParamType = "string"
	TypeInteger ParamType = "integer"
	TypeNumber  ParamType = "number"
	TypeBoolean ParamType = "boolean"
	TypeArray   ParamType = "array"
	TypeObject  ParamType = "object"
)

// ParamSpec describes one tool input parameter (a small, explicit subset of JSON
// Schema — enough to validate and to render for a model).
type ParamSpec struct {
	Name        string    `json:"name"`
	Type        ParamType `json:"type"`
	Description string    `json:"description,omitempty"`
	Required    bool      `json:"required,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
	Default     string    `json:"default,omitempty"`
}

// ToolAnnotations are MCP behavioural hints that drive safe invocation.
type ToolAnnotations struct {
	ReadOnly    bool `json:"readOnly"`
	Destructive bool `json:"destructive"`
	Idempotent  bool `json:"idempotent"`
}

// Tool is a callable MCP tool (MCP `tool`). Permission is the tool-scoped minimum
// permission a caller must hold, so authorization is fine-grained per tool.
type Tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description"`
	Params      []ParamSpec     `json:"parameters"`
	Annotations ToolAnnotations `json:"annotations"`
	Permission  Permission      `json:"permission"`
}

// ContentKind is the type of a content block in a tool/resource result.
type ContentKind string

const (
	ContentText     ContentKind = "text"
	ContentJSON     ContentKind = "json"
	ContentResource ContentKind = "resource"
)

// Content is one block of a tool result (MCP content). Text or JSON is set per Kind.
type Content struct {
	Kind ContentKind `json:"kind"`
	Text string      `json:"text,omitempty"`
	JSON any         `json:"json,omitempty"`
	URI  string      `json:"uri,omitempty"` // for resource-reference content
}

// ToolResult is what a tool returns (MCP `CallToolResult`). IsError marks a
// tool-level (grounded) failure that is still a valid, non-fatal response.
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError"`
}

// Resource is a discoverable MCP resource (MCP `resource`).
type Resource struct {
	URI         string     `json:"uri"`
	Name        string     `json:"name"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	MimeType    string     `json:"mimeType,omitempty"`
	Permission  Permission `json:"permission"`
}

// ResourceContent is the body of a resource read (MCP `ReadResourceResult`).
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// PromptArg is a prompt argument (MCP `prompt.arguments`).
type PromptArg struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Prompt is a reusable prompt template a server exposes (MCP `prompt`).
type Prompt struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Arguments   []PromptArg `json:"arguments,omitempty"`
}

// PromptMessage is one rendered message of a prompt (MCP `PromptMessage`).
type PromptMessage struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// HealthStatus is a server's coarse availability.
type HealthStatus string

const (
	HealthOK      HealthStatus = "ok"
	HealthDegrade HealthStatus = "degraded"
	HealthDown    HealthStatus = "down"
)

// Health is a server's reported health.
type Health struct {
	Status HealthStatus `json:"status"`
	Detail string       `json:"detail,omitempty"`
}

// Healthy reports whether the server is usable.
func (h Health) Healthy() bool { return h.Status == HealthOK || h.Status == HealthDegrade }

// AuditEntry is one immutable record of an MCP operation (audit trail).
type AuditEntry struct {
	Timestamp  time.Time  `json:"timestamp"`
	Caller     string     `json:"caller"`
	Server     string     `json:"server"`
	Operation  string     `json:"operation"` // discover-tools | call-tool | read-resource | ...
	Tool       string     `json:"tool,omitempty"`
	Resource   string     `json:"resource,omitempty"`
	Permission Permission `json:"permission,omitempty"`
	Outcome    string     `json:"outcome"` // ok | denied | error | timeout
	DurationMs int64      `json:"durationMs"`
	Error      string     `json:"error,omitempty"`
}
