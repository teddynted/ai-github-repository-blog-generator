package mcp

import (
	"context"
	"fmt"
	"sort"
)

// ContextDoc is one piece of generated project context exposed as an MCP
// resource (e.g. the release context JSON, the generated blog, a storyboard).
type ContextDoc struct {
	URI         string
	Name        string
	Title       string
	Description string
	MimeType    string
	Body        string
}

// contextServer exposes the pipeline's generated artifacts — release context,
// blog, storyboard, social posts — as MCP *resources* so an agent can pull them
// into a prompt by URI. It is the primary example of the resource half of MCP in
// this project. Content is injected (offline, deterministic); a production build
// would read from S3/DynamoDB behind the same resource surface.
type contextServer struct {
	BaseServer
	docs map[string]ContextDoc // uri -> doc
}

func newContextServer(docs []ContextDoc) *contextServer {
	m := map[string]ContextDoc{}
	for _, d := range docs {
		if d.MimeType == "" {
			d.MimeType = "text/plain"
		}
		m[d.URI] = d
	}
	return &contextServer{docs: m}
}

func contextDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "context",
		Info:               ServerInfo{Name: "context", Title: "Project Context", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryContext,
		Capabilities:       Capabilities{Tools: true, Resources: true},
		RequiredPermission: PermReadOnly,
		Description:        "Serves generated project artifacts (release context, blog, storyboard, social posts) as MCP resources.",
	}
}

func (s *contextServer) Info() ServerInfo           { return contextDescriptor().Info }
func (s *contextServer) Capabilities() Capabilities { return contextDescriptor().Capabilities }
func (s *contextServer) Category() Category         { return CategoryContext }

func (s *contextServer) ListResources(context.Context) ([]Resource, error) {
	out := make([]Resource, 0, len(s.docs))
	for _, d := range s.docs {
		out = append(out, Resource{
			URI:         d.URI,
			Name:        d.Name,
			Title:       d.Title,
			Description: d.Description,
			MimeType:    d.MimeType,
			Permission:  PermReadOnly,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URI < out[j].URI })
	return out, nil
}

func (s *contextServer) ReadResource(_ context.Context, uri string) (ResourceContent, error) {
	d, ok := s.docs[uri]
	if !ok {
		return ResourceContent{}, fmt.Errorf("%w: %q", ErrResourceNotFound, uri)
	}
	return ResourceContent{URI: d.URI, MimeType: d.MimeType, Text: d.Body}, nil
}

func (s *contextServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{
		{
			Name:        "list_context",
			Title:       "List context documents",
			Description: "List the URIs and titles of all available context resources.",
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "get_context",
			Title:       "Get context document",
			Description: "Return the body of a context resource by URI (equivalent to reading the resource).",
			Params:      []ParamSpec{{Name: "uri", Type: TypeString, Description: "Resource URI (from list_context).", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
	}, nil
}

func (s *contextServer) CallTool(ctx context.Context, name string, args map[string]any) (ToolResult, error) {
	switch name {
	case "list_context":
		res, _ := s.ListResources(ctx)
		out := make([]map[string]any, 0, len(res))
		for _, r := range res {
			out = append(out, map[string]any{"uri": r.URI, "title": r.Title, "mimeType": r.MimeType})
		}
		return jsonResult(out), nil
	case "get_context":
		rc, err := s.ReadResource(ctx, argString(args, "uri"))
		if err != nil {
			return errorResult("cannot read %q: %v", argString(args, "uri"), err), nil
		}
		return textResult(rc.Text), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}
