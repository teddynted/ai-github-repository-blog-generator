package platform

import (
	"context"
	"fmt"
	"sort"
)

// MCP capabilities.
const (
	CapToolDiscovery Capability = "tool-discovery"
	CapToolCall      Capability = "tool-call"
	CapResources     Capability = "resources"
)

// MCPTool describes a tool exposed by a Model Context Protocol server.
type MCPTool struct {
	Name        string
	Description string
	InputSchema map[string]string // param -> type (JSON-schema-lite)
}

// MCPResult is a tool-call result.
type MCPResult struct {
	Tool    string
	Content string
	IsError bool
}

// MCPClient is the abstraction for Model Context Protocol servers (GitHub, AWS,
// Slack, Jira, Confluence, PostgreSQL, SQLite, filesystems, search, vector DBs,
// …). Tool discovery + invocation are uniform across servers, so enterprise
// systems are integrated by adding a client, not by changing business logic.
type MCPClient interface {
	Provider
	// ListTools discovers the tools a server exposes.
	ListTools(ctx context.Context) ([]MCPTool, error)
	// CallTool invokes a tool by name with arguments.
	CallTool(ctx context.Context, name string, args map[string]string) (MCPResult, error)
}

// mockMCPServer is the reference MCP client: an in-process server exposing a
// couple of deterministic tools, demonstrating discovery + invocation offline.
type mockMCPServer struct {
	id    string
	tools map[string]MCPTool
}

func newMockMCPServer(id string) *mockMCPServer {
	return &mockMCPServer{
		id: id,
		tools: map[string]MCPTool{
			"echo":  {Name: "echo", Description: "Echo the message argument", InputSchema: map[string]string{"message": "string"}},
			"clock": {Name: "clock", Description: "Return a fixed reference timestamp", InputSchema: map[string]string{}},
		},
	}
}

func (m *mockMCPServer) ID() string { return m.id }
func (m *mockMCPServer) Kind() Kind { return KindMCP }
func (m *mockMCPServer) Capabilities() []Capability {
	return []Capability{CapToolDiscovery, CapToolCall}
}
func (m *mockMCPServer) Health(context.Context) Health { return OK() }

func (m *mockMCPServer) ListTools(context.Context) ([]MCPTool, error) {
	out := make([]MCPTool, 0, len(m.tools))
	for _, t := range m.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *mockMCPServer) CallTool(_ context.Context, name string, args map[string]string) (MCPResult, error) {
	if _, ok := m.tools[name]; !ok {
		return MCPResult{Tool: name, IsError: true, Content: "unknown tool"}, fmt.Errorf("%s: unknown tool %q", m.id, name)
	}
	switch name {
	case "echo":
		return MCPResult{Tool: name, Content: args["message"]}, nil
	case "clock":
		return MCPResult{Tool: name, Content: "2026-01-01T00:00:00Z"}, nil
	default:
		return MCPResult{Tool: name, IsError: true}, ErrUnsupported
	}
}

// NewMockMCPServer builds the reference MCP client.
func NewMockMCPServer(id string) MCPClient { return newMockMCPServer(id) }

func registerMCP(r *Registry) {
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "mock", Kind: KindMCP, Name: "Mock MCP (reference)", Version: "1.0.0",
			Priority: 1, Description: "In-process reference MCP server",
			Capabilities: []Capability{CapToolDiscovery, CapToolCall},
		},
		Factory: func(ConfigSource) (Provider, error) { return newMockMCPServer("mock"), nil },
	})
}
