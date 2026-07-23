package mcp

import "context"

// InProcessTransport connects to servers that live in this process (the default).
// It resolves a descriptor to the live Server held by a Registry. Remote
// transports (stdio, streamable HTTP) implement the same Transport port and can
// be dropped in without changing the Client — the extensibility seam for talking
// to external MCP servers.
type InProcessTransport struct {
	Registry *Registry
}

// NewInProcessTransport builds an in-process transport over a registry.
func NewInProcessTransport(r *Registry) *InProcessTransport {
	if r == nil {
		r = DefaultRegistry
	}
	return &InProcessTransport{Registry: r}
}

func (t *InProcessTransport) Name() string { return "in-process" }

func (t *InProcessTransport) Connect(_ context.Context, d ServerDescriptor) (Server, error) {
	reg := t.Registry
	if reg == nil {
		reg = DefaultRegistry
	}
	return reg.Instance(d.ID)
}
