package mcp

import (
	"context"
	"fmt"
	"strings"
)

// mermaidServer generates and validates Mermaid diagram source. All operations
// are pure string transforms — no rendering, no network — so the server is
// read-only and deterministic.
type mermaidServer struct {
	BaseServer
}

func newMermaidServer() *mermaidServer { return &mermaidServer{} }

// known Mermaid diagram headers used for lightweight validation.
var mermaidHeaders = []string{
	"graph", "flowchart", "sequenceDiagram", "classDiagram", "stateDiagram",
	"stateDiagram-v2", "erDiagram", "gantt", "pie", "journey", "gitGraph", "mindmap",
}

func mermaidDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "mermaid",
		Info:               ServerInfo{Name: "mermaid", Title: "Mermaid", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryMermaid,
		Capabilities:       Capabilities{Tools: true},
		RequiredPermission: PermReadOnly,
		Description:        "Generate and validate Mermaid diagram source.",
	}
}

func (s *mermaidServer) Info() ServerInfo           { return mermaidDescriptor().Info }
func (s *mermaidServer) Capabilities() Capabilities { return mermaidDescriptor().Capabilities }
func (s *mermaidServer) Category() Category         { return CategoryMermaid }

func (s *mermaidServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{
		{
			Name:        "flowchart",
			Title:       "Generate flowchart",
			Description: "Build a Mermaid flowchart from an ordered list of step labels joined into a linear flow.",
			Params: []ParamSpec{
				{Name: "direction", Type: TypeString, Description: "Layout direction.", Default: "TD", Enum: []string{"TD", "LR", "BT", "RL"}},
				{Name: "steps", Type: TypeString, Description: "Step labels separated by '|', in order.", Required: true},
			},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "sequence",
			Title:       "Generate sequence diagram",
			Description: "Build a Mermaid sequence diagram from 'Actor->Target: message' lines separated by '|'.",
			Params:      []ParamSpec{{Name: "messages", Type: TypeString, Description: "Messages separated by '|'.", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "validate",
			Title:       "Validate diagram",
			Description: "Check that Mermaid source begins with a recognized diagram header.",
			Params:      []ParamSpec{{Name: "source", Type: TypeString, Description: "Mermaid source.", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
	}, nil
}

func (s *mermaidServer) CallTool(_ context.Context, name string, args map[string]any) (ToolResult, error) {
	switch name {
	case "flowchart":
		dir := argString(args, "direction")
		steps := splitPipes(argString(args, "steps"))
		if len(steps) == 0 {
			return errorResult("no steps provided"), nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "flowchart %s\n", dir)
		for i, step := range steps {
			fmt.Fprintf(&b, "    n%d[%q]\n", i, step)
		}
		for i := 0; i < len(steps)-1; i++ {
			fmt.Fprintf(&b, "    n%d --> n%d\n", i, i+1)
		}
		return textResult(b.String()), nil
	case "sequence":
		msgs := splitPipes(argString(args, "messages"))
		if len(msgs) == 0 {
			return errorResult("no messages provided"), nil
		}
		var b strings.Builder
		b.WriteString("sequenceDiagram\n")
		for _, m := range msgs {
			fmt.Fprintf(&b, "    %s\n", strings.TrimSpace(m))
		}
		out := b.String()
		if r := validateMermaid(out); r.IsError {
			return r, nil
		}
		return textResult(out), nil
	case "validate":
		return validateMermaid(argString(args, "source")), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}

func validateMermaid(src string) ToolResult {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" {
		return errorResult("empty diagram")
	}
	first := strings.Fields(trimmed)[0]
	for _, h := range mermaidHeaders {
		if first == h || strings.HasPrefix(trimmed, h) {
			return jsonResult(map[string]any{"valid": true, "type": h})
		}
	}
	return errorResult("unrecognized diagram header %q", first)
}

func splitPipes(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "|") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
