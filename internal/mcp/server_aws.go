package mcp

import (
	"context"
	"fmt"
	"sort"
)

// AWSMock is offline, read-only AWS data served by the aws server. In production
// the same server would be backed by the AWS SDK (using IAM credentials resolved
// through the AuthProvider) behind an identical tool surface; the mock keeps the
// package offline and its behavior deterministic. It exposes only read
// operations — least privilege by construction.
type AWSMock struct {
	Region     string
	Buckets    []string
	Stacks     map[string]string // stack name -> status
	Parameters map[string]string // SSM parameter name -> value
}

type awsServer struct {
	BaseServer
	data AWSMock
}

func newAWSServer(data AWSMock) *awsServer {
	if data.Region == "" {
		data.Region = "us-east-1"
	}
	return &awsServer{data: data}
}

func awsDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "aws",
		Info:               ServerInfo{Name: "aws", Title: "AWS", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryAWS,
		Capabilities:       Capabilities{Tools: true},
		RequiredPermission: PermReadOnly,
		Description:        "Read-only AWS inspection: S3 buckets, CloudFormation stacks, SSM parameters.",
	}
}

func (s *awsServer) Info() ServerInfo           { return awsDescriptor().Info }
func (s *awsServer) Capabilities() Capabilities { return awsDescriptor().Capabilities }
func (s *awsServer) Category() Category         { return CategoryAWS }

func (s *awsServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{
		{
			Name:        "list_buckets",
			Title:       "List S3 buckets",
			Description: "List S3 bucket names in the account.",
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "describe_stack",
			Title:       "Describe stack",
			Description: "Return the status of a CloudFormation stack.",
			Params:      []ParamSpec{{Name: "name", Type: TypeString, Description: "Stack name.", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "get_parameter",
			Title:       "Get SSM parameter",
			Description: "Return the value of an SSM parameter.",
			Params:      []ParamSpec{{Name: "name", Type: TypeString, Description: "Parameter name.", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
	}, nil
}

func (s *awsServer) CallTool(_ context.Context, name string, args map[string]any) (ToolResult, error) {
	switch name {
	case "list_buckets":
		buckets := append([]string(nil), s.data.Buckets...)
		sort.Strings(buckets)
		return jsonResult(map[string]any{"region": s.data.Region, "buckets": buckets}), nil
	case "describe_stack":
		status, ok := s.data.Stacks[argString(args, "name")]
		if !ok {
			return errorResult("no such stack: %s", argString(args, "name")), nil
		}
		return jsonResult(map[string]any{"name": argString(args, "name"), "status": status}), nil
	case "get_parameter":
		val, ok := s.data.Parameters[argString(args, "name")]
		if !ok {
			return errorResult("no such parameter: %s", argString(args, "name")), nil
		}
		return jsonResult(map[string]any{"name": argString(args, "name"), "value": val}), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}
