package awsec2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// IdleAPI is the EC2 subset the idle-stop path uses: read an instance's state,
// launch time and tags, and set or clear the idle-streak tag. It is separate
// from API so the shared start/stop adapter keeps its narrow surface.
type IdleAPI interface {
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	CreateTags(ctx context.Context, in *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DeleteTags(ctx context.Context, in *ec2.DeleteTagsInput, optFns ...func(*ec2.Options)) (*ec2.DeleteTagsOutput, error)
}

// Snapshot is an instance's current state, launch time and tags.
type Snapshot struct {
	State      string
	LaunchTime time.Time
	Tags       map[string]string
}

// IdleClient reads snapshots and mutates a single tag for the idle detector.
type IdleClient struct {
	api IdleAPI
}

// NewIdle builds an IdleClient. The real *ec2.Client satisfies IdleAPI.
func NewIdle(api IdleAPI) *IdleClient { return &IdleClient{api: api} }

// Snapshot returns the instance's state, launch time and tags. State is empty
// when the instance is not found.
func (c *IdleClient) Snapshot(ctx context.Context, id string) (Snapshot, error) {
	out, err := c.api.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{id}})
	if err != nil {
		return Snapshot{}, fmt.Errorf("describe instances: %w", err)
	}
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			s := Snapshot{Tags: map[string]string{}}
			if inst.State != nil {
				s.State = string(inst.State.Name)
			}
			if inst.LaunchTime != nil {
				s.LaunchTime = *inst.LaunchTime
			}
			for _, t := range inst.Tags {
				s.Tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
			}
			return s, nil
		}
	}
	return Snapshot{}, nil
}

// SetTag sets (creates or overwrites) a tag on the instance.
func (c *IdleClient) SetTag(ctx context.Context, id, key, value string) error {
	_, err := c.api.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{id},
		Tags:      []ec2types.Tag{{Key: aws.String(key), Value: aws.String(value)}},
	})
	if err != nil {
		return fmt.Errorf("create tag %s: %w", key, err)
	}
	return nil
}

// DeleteTag removes a tag from the instance. Deleting an absent tag is a no-op.
func (c *IdleClient) DeleteTag(ctx context.Context, id, key string) error {
	_, err := c.api.DeleteTags(ctx, &ec2.DeleteTagsInput{
		Resources: []string{id},
		Tags:      []ec2types.Tag{{Key: aws.String(key)}},
	})
	if err != nil {
		return fmt.Errorf("delete tag %s: %w", key, err)
	}
	return nil
}
