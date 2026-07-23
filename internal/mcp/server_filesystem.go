package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// filesystemServer exposes read-only, sandboxed file access rooted at a single
// directory. Every path is resolved against the root and rejected if it escapes
// it (path-traversal protection) — the server can never read outside its sandbox.
type filesystemServer struct {
	BaseServer
	root    string
	maxSize int64
}

// newFilesystemServer roots a filesystem server at dir (defaulting to "."). It
// caps individual file reads at maxSize bytes (0 → 1 MiB) to bound resource use.
func newFilesystemServer(dir string, maxSize int64) *filesystemServer {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err == nil {
		dir = abs
	}
	if maxSize <= 0 {
		maxSize = 1 << 20
	}
	return &filesystemServer{root: filepath.Clean(dir), maxSize: maxSize}
}

func filesystemDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "filesystem",
		Info:               ServerInfo{Name: "filesystem", Title: "Filesystem", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryFilesystem,
		Capabilities:       Capabilities{Tools: true},
		RequiredPermission: PermReadOnly,
		Description:        "Sandboxed, read-only access to files under a fixed root directory.",
	}
}

func (s *filesystemServer) Info() ServerInfo           { return filesystemDescriptor().Info }
func (s *filesystemServer) Capabilities() Capabilities { return filesystemDescriptor().Capabilities }
func (s *filesystemServer) Category() Category         { return CategoryFilesystem }

func (s *filesystemServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{
		{
			Name:        "read_file",
			Title:       "Read file",
			Description: "Read the contents of a file within the sandbox root.",
			Params:      []ParamSpec{{Name: "path", Type: TypeString, Description: "Path relative to the root.", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "list_dir",
			Title:       "List directory",
			Description: "List entries of a directory within the sandbox root.",
			Params:      []ParamSpec{{Name: "path", Type: TypeString, Description: "Directory path relative to the root.", Default: "."}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "stat",
			Title:       "Stat path",
			Description: "Report size and kind for a path within the sandbox root.",
			Params:      []ParamSpec{{Name: "path", Type: TypeString, Description: "Path relative to the root.", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
	}, nil
}

// resolve joins a caller-supplied path to the root and verifies it stays inside.
func (s *filesystemServer) resolve(rel string) (string, error) {
	clean := filepath.Clean("/" + rel) // strip leading traversal, force absolute-in-root
	full := filepath.Join(s.root, clean)
	if full != s.root && !strings.HasPrefix(full, s.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: path escapes sandbox", ErrForbidden)
	}
	return full, nil
}

func (s *filesystemServer) CallTool(_ context.Context, name string, args map[string]any) (ToolResult, error) {
	switch name {
	case "read_file":
		full, err := s.resolve(argString(args, "path"))
		if err != nil {
			return ToolResult{}, err
		}
		info, err := os.Stat(full)
		if err != nil {
			return errorResult("cannot stat %s: %v", argString(args, "path"), err), nil
		}
		if info.IsDir() {
			return errorResult("%s is a directory", argString(args, "path")), nil
		}
		if info.Size() > s.maxSize {
			return errorResult("file exceeds max read size of %d bytes", s.maxSize), nil
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return errorResult("cannot read %s: %v", argString(args, "path"), err), nil
		}
		return textResult(string(b)), nil
	case "list_dir":
		full, err := s.resolve(argString(args, "path"))
		if err != nil {
			return ToolResult{}, err
		}
		entries, err := os.ReadDir(full)
		if err != nil {
			return errorResult("cannot list %s: %v", argString(args, "path"), err), nil
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			n := e.Name()
			if e.IsDir() {
				n += "/"
			}
			names = append(names, n)
		}
		sort.Strings(names)
		return jsonResult(names), nil
	case "stat":
		full, err := s.resolve(argString(args, "path"))
		if err != nil {
			return ToolResult{}, err
		}
		info, err := os.Stat(full)
		if err != nil {
			return errorResult("cannot stat %s: %v", argString(args, "path"), err), nil
		}
		kind := "file"
		if info.IsDir() {
			kind = "directory"
		}
		return jsonResult(map[string]any{"kind": kind, "size": info.Size(), "name": info.Name()}), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}
