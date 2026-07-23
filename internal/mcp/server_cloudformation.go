package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// cloudformationServer analyses CloudFormation YAML templates. It is deliberately
// dependency-free: rather than pull in a YAML parser, it does a lightweight,
// indentation-aware structural scan sufficient to validate required sections,
// enumerate resources, and summarize a template. All analysis is pure and
// offline (read-only) — it never calls AWS.
type cloudformationServer struct {
	BaseServer
}

func newCloudFormationServer() *cloudformationServer { return &cloudformationServer{} }

func cloudformationDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "cloudformation",
		Info:               ServerInfo{Name: "cloudformation", Title: "CloudFormation", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryCloudFormation,
		Capabilities:       Capabilities{Tools: true},
		RequiredPermission: PermReadOnly,
		Description:        "Validate, summarize, and enumerate resources in CloudFormation templates.",
	}
}

func (s *cloudformationServer) Info() ServerInfo { return cloudformationDescriptor().Info }
func (s *cloudformationServer) Capabilities() Capabilities {
	return cloudformationDescriptor().Capabilities
}
func (s *cloudformationServer) Category() Category { return CategoryCloudFormation }

func (s *cloudformationServer) ListTools(context.Context) ([]Tool, error) {
	tmplParam := ParamSpec{Name: "template", Type: TypeString, Description: "CloudFormation template body (YAML).", Required: true}
	return []Tool{
		{
			Name:        "validate_template",
			Title:       "Validate template",
			Description: "Check that a template has the required structure (a non-empty Resources section, each resource with a Type).",
			Params:      []ParamSpec{tmplParam},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "list_resources",
			Title:       "List resources",
			Description: "List the logical IDs and Types of resources declared in a template.",
			Params:      []ParamSpec{tmplParam},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "summarize_template",
			Title:       "Summarize template",
			Description: "Report the sections present and resource/parameter/output counts.",
			Params:      []ParamSpec{tmplParam},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
	}, nil
}

// cfnResource is one logical resource discovered in a template.
type cfnResource struct {
	LogicalID string `json:"logicalId"`
	Type      string `json:"type"`
}

// scanTemplate does an indentation-aware pass over a template, returning the set
// of top-level sections and the resources under the Resources section.
func scanTemplate(body string) (sections []string, resources []cfnResource) {
	lines := strings.Split(body, "\n")
	seen := map[string]bool{}
	inResources := false
	resIndent := -1
	var cur *cfnResource
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if indent == 0 && strings.Contains(trimmed, ":") {
			key := strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[0])
			if key != "" {
				if !seen[key] {
					seen[key] = true
					sections = append(sections, key)
				}
				inResources = key == "Resources"
				resIndent = -1
				if cur != nil {
					resources = append(resources, *cur)
					cur = nil
				}
			}
			continue
		}
		if !inResources {
			continue
		}
		// First indented level under Resources is a logical ID.
		if resIndent == -1 || indent == resIndent {
			resIndent = indent
			if strings.HasSuffix(trimmed, ":") {
				if cur != nil {
					resources = append(resources, *cur)
				}
				cur = &cfnResource{LogicalID: strings.TrimSuffix(trimmed, ":")}
			}
			continue
		}
		// Deeper lines: capture the resource Type.
		if cur != nil && strings.HasPrefix(trimmed, "Type:") {
			cur.Type = strings.TrimSpace(strings.TrimPrefix(trimmed, "Type:"))
		}
	}
	if cur != nil {
		resources = append(resources, *cur)
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].LogicalID < resources[j].LogicalID })
	return sections, resources
}

func (s *cloudformationServer) CallTool(_ context.Context, name string, args map[string]any) (ToolResult, error) {
	body := argString(args, "template")
	sections, resources := scanTemplate(body)
	switch name {
	case "validate_template":
		var problems []string
		if !contains(sections, "Resources") {
			problems = append(problems, "missing required Resources section")
		}
		if contains(sections, "Resources") && len(resources) == 0 {
			problems = append(problems, "Resources section is empty")
		}
		for _, r := range resources {
			if r.Type == "" {
				problems = append(problems, fmt.Sprintf("resource %q has no Type", r.LogicalID))
			}
		}
		if len(problems) > 0 {
			return jsonResult(map[string]any{"valid": false, "problems": problems}), nil
		}
		return jsonResult(map[string]any{"valid": true, "resources": len(resources)}), nil
	case "list_resources":
		return jsonResult(resources), nil
	case "summarize_template":
		return jsonResult(map[string]any{
			"sections":  sections,
			"resources": len(resources),
		}), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}
