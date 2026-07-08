// Package awsec2 adapts the EC2 API to the lifecycle.EC2 port, locating the
// platform's instance by its Project tag and starting it.
package awsec2

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// API is the subset of the EC2 client this adapter uses.
type API interface {
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	StartInstances(ctx context.Context, in *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
}

// Client implements lifecycle.EC2.
type Client struct {
	api API
}

// New builds a Client.
func New(api API) *Client { return &Client{api: api} }

// FindInstance returns the id and state of the instance tagged Project=project.
// Terminated instances are excluded. Returns an empty id when none is found.
func (c *Client) FindInstance(ctx context.Context, project string) (string, string, error) {
	out, err := c.api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("tag:Project"), Values: []string{project}},
			{Name: aws.String("instance-state-name"), Values: []string{"pending", "running", "stopping", "stopped"}},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("describe instances: %w", err)
	}
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			state := ""
			if inst.State != nil {
				state = string(inst.State.Name)
			}
			return aws.ToString(inst.InstanceId), state, nil
		}
	}
	return "", "", nil
}

// StartInstance starts the given instance.
func (c *Client) StartInstance(ctx context.Context, id string) error {
	_, err := c.api.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{id}})
	if err != nil {
		return fmt.Errorf("start instances: %w", err)
	}
	return nil
}
