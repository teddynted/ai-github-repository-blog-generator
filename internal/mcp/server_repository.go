package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// RepoSnapshot is an offline, deterministic view of a git repository that the
// repository server serves. Production wiring populates it from `git` (or the
// GitHub API) at construction; tests inject a fixed snapshot. Keeping the data
// behind a plain struct means the server never shells out during a call and is
// fully testable without a real repo.
type RepoSnapshot struct {
	Name          string
	DefaultBranch string
	Commits       []RepoCommit
	Tags          []string
	Files         []string
}

// RepoCommit is a single commit in the snapshot.
type RepoCommit struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Subject string `json:"subject"`
}

type repositoryServer struct {
	BaseServer
	snap RepoSnapshot
}

func newRepositoryServer(snap RepoSnapshot) *repositoryServer {
	return &repositoryServer{snap: snap}
}

func repositoryDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "repository",
		Info:               ServerInfo{Name: "repository", Title: "Repository", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryRepository,
		Capabilities:       Capabilities{Tools: true},
		RequiredPermission: PermReadOnly,
		Description:        "Read-only git repository metadata: branch, commits, tags, tracked files.",
	}
}

func (s *repositoryServer) Info() ServerInfo           { return repositoryDescriptor().Info }
func (s *repositoryServer) Capabilities() Capabilities { return repositoryDescriptor().Capabilities }
func (s *repositoryServer) Category() Category         { return CategoryRepository }

func (s *repositoryServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{
		{
			Name:        "describe_repo",
			Title:       "Describe repository",
			Description: "Return the repository name, default branch, and counts.",
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "list_commits",
			Title:       "List commits",
			Description: "List recent commits, most recent first.",
			Params:      []ParamSpec{{Name: "limit", Type: TypeInteger, Description: "Maximum commits to return (default 20).", Default: "20"}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "list_tags",
			Title:       "List tags",
			Description: "List all release tags.",
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "list_files",
			Title:       "List files",
			Description: "List tracked files, optionally filtered by a path prefix.",
			Params:      []ParamSpec{{Name: "prefix", Type: TypeString, Description: "Only return files under this prefix.", Default: ""}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
	}, nil
}

func (s *repositoryServer) CallTool(_ context.Context, name string, args map[string]any) (ToolResult, error) {
	switch name {
	case "describe_repo":
		return jsonResult(map[string]any{
			"name":          s.snap.Name,
			"defaultBranch": s.snap.DefaultBranch,
			"commits":       len(s.snap.Commits),
			"tags":          len(s.snap.Tags),
			"files":         len(s.snap.Files),
		}), nil
	case "list_commits":
		limit := parseIntDefault(argString(args, "limit"), 20)
		commits := s.snap.Commits
		if limit >= 0 && limit < len(commits) {
			commits = commits[:limit]
		}
		return jsonResult(commits), nil
	case "list_tags":
		tags := append([]string(nil), s.snap.Tags...)
		sort.Strings(tags)
		return jsonResult(tags), nil
	case "list_files":
		prefix := argString(args, "prefix")
		var out []string
		for _, f := range s.snap.Files {
			if prefix == "" || strings.HasPrefix(f, prefix) {
				out = append(out, f)
			}
		}
		sort.Strings(out)
		return jsonResult(out), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}
