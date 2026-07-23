package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// PublishPlatform describes a destination the publishing server can target.
type PublishPlatform struct {
	Name     string
	MaxChars int // 0 = unlimited
}

// publishingServer validates and (dry-run) publishes content to social/blog
// platforms. Read operations (list/validate) need only read access; the publish
// tool requires the elevated `publish` permission and is marked non-read-only in
// its annotations. The default build performs a dry run — it never calls an
// external API — so it is safe and deterministic; a production build swaps in a
// real client behind the same tool surface, authenticated via the AuthProvider.
type publishingServer struct {
	BaseServer
	platforms map[string]PublishPlatform
	live      bool // when false, publish is a dry run (the offline default)
}

func newPublishingServer(platforms []PublishPlatform) *publishingServer {
	m := map[string]PublishPlatform{}
	for _, p := range platforms {
		m[p.Name] = p
	}
	return &publishingServer{platforms: m}
}

func publishingDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "publishing",
		Info:               ServerInfo{Name: "publishing", Title: "Publishing", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryPublishing,
		Capabilities:       Capabilities{Tools: true},
		RequiredPermission: PermReadOnly,
		Description:        "Validate and publish content to blog and social platforms.",
	}
}

func (s *publishingServer) Info() ServerInfo           { return publishingDescriptor().Info }
func (s *publishingServer) Capabilities() Capabilities { return publishingDescriptor().Capabilities }
func (s *publishingServer) Category() Category         { return CategoryPublishing }

func (s *publishingServer) ListTools(context.Context) ([]Tool, error) {
	platformParam := ParamSpec{Name: "platform", Type: TypeString, Description: "Target platform name.", Required: true}
	contentParam := ParamSpec{Name: "content", Type: TypeString, Description: "Content to publish.", Required: true}
	return []Tool{
		{
			Name:        "list_platforms",
			Title:       "List platforms",
			Description: "List configured publishing platforms and their limits.",
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "validate_post",
			Title:       "Validate post",
			Description: "Check that content fits a platform's constraints without publishing.",
			Params:      []ParamSpec{platformParam, contentParam},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "publish_post",
			Title:       "Publish post",
			Description: "Publish content to a platform, returning a receipt. Requires publish permission.",
			Params:      []ParamSpec{platformParam, contentParam},
			Annotations: ToolAnnotations{ReadOnly: false, Destructive: false, Idempotent: false},
			Permission:  PermPublish,
		},
	}, nil
}

func (s *publishingServer) validate(platform, content string) (PublishPlatform, *ToolResult) {
	p, ok := s.platforms[platform]
	if !ok {
		r := errorResult("unknown platform: %s", platform)
		return PublishPlatform{}, &r
	}
	if content == "" {
		r := errorResult("content is empty")
		return p, &r
	}
	if p.MaxChars > 0 && len(content) > p.MaxChars {
		r := errorResult("content is %d chars, exceeds %s limit of %d", len(content), platform, p.MaxChars)
		return p, &r
	}
	return p, nil
}

func (s *publishingServer) CallTool(_ context.Context, name string, args map[string]any) (ToolResult, error) {
	switch name {
	case "list_platforms":
		out := make([]map[string]any, 0, len(s.platforms))
		for _, p := range s.platforms {
			out = append(out, map[string]any{"name": p.Name, "maxChars": p.MaxChars})
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
		return jsonResult(out), nil
	case "validate_post":
		if _, bad := s.validate(argString(args, "platform"), argString(args, "content")); bad != nil {
			return *bad, nil
		}
		return jsonResult(map[string]any{"valid": true}), nil
	case "publish_post":
		platform := argString(args, "platform")
		content := argString(args, "content")
		if _, bad := s.validate(platform, content); bad != nil {
			return *bad, nil
		}
		sum := sha256.Sum256([]byte(platform + "\x00" + content))
		receipt := map[string]any{
			"platform":  platform,
			"published": s.live,
			"dryRun":    !s.live,
			"contentId": hex.EncodeToString(sum[:8]),
			"chars":     len(content),
		}
		return jsonResult(receipt), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}
