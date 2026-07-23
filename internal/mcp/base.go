package mcp

import (
	"context"
	"fmt"
	"strconv"
)

// BaseServer provides no-op implementations of the optional MCP methods
// (resources + prompts). A server that only exposes tools embeds it and
// overrides just ListTools/CallTool, keeping each server small.
type BaseServer struct{}

func (BaseServer) ListResources(context.Context) ([]Resource, error) { return nil, nil }
func (BaseServer) ReadResource(context.Context, string) (ResourceContent, error) {
	return ResourceContent{}, ErrResourceNotFound
}
func (BaseServer) ListPrompts(context.Context) ([]Prompt, error) { return nil, nil }
func (BaseServer) GetPrompt(context.Context, string, map[string]string) ([]PromptMessage, error) {
	return nil, ErrPromptNotFound
}
func (BaseServer) Health(context.Context) Health { return Health{Status: HealthOK} }

// --- result builders ---

// textResult wraps a plain-text tool result.
func textResult(text string) ToolResult {
	return ToolResult{Content: []Content{{Kind: ContentText, Text: text}}}
}

// jsonResult wraps a structured tool result.
func jsonResult(v any) ToolResult {
	return ToolResult{Content: []Content{{Kind: ContentJSON, JSON: v}}}
}

// errorResult is a tool-level (grounded) error — a valid response, not a fault.
func errorResult(format string, a ...any) ToolResult {
	return ToolResult{IsError: true, Content: []Content{{Kind: ContentText, Text: fmt.Sprintf(format, a...)}}}
}

// --- argument validation (shared by every server's CallTool) ---

// validate checks args against a tool's ParamSpecs: required presence, enum
// membership, and coarse type. It returns a normalized args map (defaults
// applied) or an ErrInvalidArgument. This is the single input-validation seam, so
// no server hand-rolls validation.
func validate(specs []ParamSpec, args map[string]any) (map[string]any, error) {
	out := map[string]any{}
	byName := map[string]ParamSpec{}
	for _, s := range specs {
		byName[s.Name] = s
	}
	// Reject unknown params (secure-by-default: no silent extra input).
	for k := range args {
		if _, ok := byName[k]; !ok {
			return nil, fmt.Errorf("%w: unknown parameter %q", ErrInvalidArgument, k)
		}
	}
	for _, s := range specs {
		v, present := args[s.Name]
		if !present {
			if s.Required {
				return nil, fmt.Errorf("%w: missing required parameter %q", ErrInvalidArgument, s.Name)
			}
			if s.Default != "" {
				out[s.Name] = s.Default
			}
			continue
		}
		if err := checkType(s, v); err != nil {
			return nil, err
		}
		if len(s.Enum) > 0 {
			sv := fmt.Sprint(v)
			if !contains(s.Enum, sv) {
				return nil, fmt.Errorf("%w: %q must be one of %v", ErrInvalidArgument, s.Name, s.Enum)
			}
		}
		out[s.Name] = v
	}
	return out, nil
}

func checkType(s ParamSpec, v any) error {
	switch s.Type {
	case TypeString, "":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("%w: %q must be a string", ErrInvalidArgument, s.Name)
		}
	case TypeBoolean:
		switch tv := v.(type) {
		case bool:
		case string:
			if _, err := strconv.ParseBool(tv); err != nil {
				return fmt.Errorf("%w: %q must be a boolean", ErrInvalidArgument, s.Name)
			}
		default:
			return fmt.Errorf("%w: %q must be a boolean", ErrInvalidArgument, s.Name)
		}
	case TypeInteger, TypeNumber:
		switch tv := v.(type) {
		case int, int64, float64:
		case string:
			if _, err := strconv.ParseFloat(tv, 64); err != nil {
				return fmt.Errorf("%w: %q must be numeric", ErrInvalidArgument, s.Name)
			}
		default:
			return fmt.Errorf("%w: %q must be numeric", ErrInvalidArgument, s.Name)
		}
	}
	return nil
}

// argString reads a string argument (already validated).
func argString(args map[string]any, name string) string {
	if v, ok := args[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprint(v)
	}
	return ""
}

// argBool reads a boolean argument.
func argBool(args map[string]any, name string) bool {
	switch v := args[name].(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	}
	return false
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
