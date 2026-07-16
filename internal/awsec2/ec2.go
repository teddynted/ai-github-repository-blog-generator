// Package awsec2 adapts the EC2 API to the lifecycle.EC2 and power.Instances
// ports: it locates the platform's instance by its Project tag (for the webhook
// window gate) and starts/stops a specific instance by ID (for the scheduler).
package awsec2

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/lifecycle"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/power"
)

// API is the subset of the EC2 client this adapter uses.
type API interface {
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	StartInstances(ctx context.Context, in *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
	StopInstances(ctx context.Context, in *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
}

// Client implements lifecycle.EC2.
type Client struct {
	api API
}

// New builds a Client.
func New(api API) *Client { return &Client{api: api} }

// FindInstance returns the instance tagged Project=project (terminated
// instances excluded). Returns a zero-value Instance when none is found.
func (c *Client) FindInstance(ctx context.Context, project string) (lifecycle.Instance, error) {
	out, err := c.api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("tag:Project"), Values: []string{project}},
			{Name: aws.String("instance-state-name"), Values: []string{"pending", "running", "stopping", "stopped"}},
		},
	})
	if err != nil {
		return lifecycle.Instance{}, fmt.Errorf("describe instances: %w", err)
	}
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			result := lifecycle.Instance{ID: aws.ToString(inst.InstanceId)}
			if inst.State != nil {
				result.State = string(inst.State.Name)
			}
			if inst.LaunchTime != nil {
				result.LaunchTime = *inst.LaunchTime
			}
			return result, nil
		}
	}
	return lifecycle.Instance{}, nil
}

// InstanceState returns the current state name of a specific instance located
// by ID (e.g. "running", "stopped"). It returns an empty string when no such
// instance exists, so callers can distinguish "absent" from a real state. A
// terminated instance may briefly remain visible and is reported as such.
func (c *Client) InstanceState(ctx context.Context, id string) (string, error) {
	out, err := c.api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{id},
	})
	if err != nil {
		return "", fmt.Errorf("describe instances: %w", err)
	}
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			if inst.State != nil {
				return string(inst.State.Name), nil
			}
		}
	}
	return "", nil
}

// StartInstance starts the given instance.
func (c *Client) StartInstance(ctx context.Context, id string) error {
	_, err := c.api.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{id}})
	if err != nil {
		return fmt.Errorf("start instances: %w", err)
	}
	return nil
}

// StopInstance stops the given instance.
func (c *Client) StopInstance(ctx context.Context, id string) error {
	_, err := c.api.StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: []string{id}})
	if err != nil {
		return fmt.Errorf("stop instances: %w", err)
	}
	return nil
}

// Guards: *Client satisfies both the tag-based lifecycle.EC2 port and the
// ID-based power.Instances port (scheduled start/stop).
var (
	_ lifecycle.EC2   = (*Client)(nil)
	_ power.Instances = (*Client)(nil)
)
