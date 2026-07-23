package mcp

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// documentationServer provides search + retrieval over a project's Markdown
// documentation tree. It indexes lazily on first use and only ever reads files
// under its root (read-only).
type documentationServer struct {
	BaseServer
	root string
	fsys fs.FS // injectable for tests; nil → os filesystem rooted at root
}

// newDocumentationServer indexes Markdown docs under dir (default "docs").
func newDocumentationServer(dir string) *documentationServer {
	if dir == "" {
		dir = "docs"
	}
	return &documentationServer{root: dir}
}

func documentationDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "documentation",
		Info:               ServerInfo{Name: "documentation", Title: "Documentation", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryDocumentation,
		Capabilities:       Capabilities{Tools: true},
		RequiredPermission: PermReadOnly,
		Description:        "Search and retrieve project Markdown documentation.",
	}
}

func (s *documentationServer) Info() ServerInfo { return documentationDescriptor().Info }
func (s *documentationServer) Capabilities() Capabilities {
	return documentationDescriptor().Capabilities
}
func (s *documentationServer) Category() Category { return CategoryDocumentation }

func (s *documentationServer) fsSys() fs.FS {
	if s.fsys != nil {
		return s.fsys
	}
	return os.DirFS(s.root)
}

func (s *documentationServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{
		{
			Name:        "list_docs",
			Title:       "List documents",
			Description: "List all Markdown documents in the documentation tree.",
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "get_doc",
			Title:       "Get document",
			Description: "Return the full contents of one Markdown document.",
			Params:      []ParamSpec{{Name: "path", Type: TypeString, Description: "Document path (as returned by list_docs).", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "search_docs",
			Title:       "Search documents",
			Description: "Case-insensitive substring search across the documentation, returning matching files with line context.",
			Params:      []ParamSpec{{Name: "query", Type: TypeString, Description: "Text to search for.", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
	}, nil
}

func (s *documentationServer) docs() ([]string, error) {
	var out []string
	err := fs.WalkDir(s.fsSys(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(p), ".md") {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func (s *documentationServer) CallTool(_ context.Context, name string, args map[string]any) (ToolResult, error) {
	switch name {
	case "list_docs":
		docs, err := s.docs()
		if err != nil {
			return errorResult("cannot list docs: %v", err), nil
		}
		return jsonResult(docs), nil
	case "get_doc":
		p := cleanDocPath(argString(args, "path"))
		b, err := fs.ReadFile(s.fsSys(), p)
		if err != nil {
			return errorResult("cannot read %q: %v", p, err), nil
		}
		return textResult(string(b)), nil
	case "search_docs":
		query := strings.ToLower(argString(args, "query"))
		docs, err := s.docs()
		if err != nil {
			return errorResult("cannot search docs: %v", err), nil
		}
		type hit struct {
			Path string `json:"path"`
			Line int    `json:"line"`
			Text string `json:"text"`
		}
		var hits []hit
		for _, p := range docs {
			b, err := fs.ReadFile(s.fsSys(), p)
			if err != nil {
				continue
			}
			for i, ln := range strings.Split(string(b), "\n") {
				if strings.Contains(strings.ToLower(ln), query) {
					hits = append(hits, hit{Path: p, Line: i + 1, Text: strings.TrimSpace(ln)})
				}
			}
		}
		return jsonResult(hits), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}

func cleanDocPath(p string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+p)), "/")
}
