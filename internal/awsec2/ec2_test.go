package awsec2

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type fakeAPI struct {
	describeIn *ec2.DescribeInstancesInput
	reservs    []ec2types.Reservation
	startedIDs []string
}

func (f *fakeAPI) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.describeIn = in
	return &ec2.DescribeInstancesOutput{Reservations: f.reservs}, nil
}
func (f *fakeAPI) StartInstances(_ context.Context, in *ec2.StartInstancesInput, _ ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error) {
	f.startedIDs = append(f.startedIDs, in.InstanceIds...)
	return &ec2.StartInstancesOutput{}, nil
}

func TestFindInstanceReturnsIDAndState(t *testing.T) {
	f := &fakeAPI{reservs: []ec2types.Reservation{{
		Instances: []ec2types.Instance{{
			InstanceId: aws.String("i-abc"),
			State:      &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
		}},
	}}}
	id, state, err := New(f).FindInstance(context.Background(), "blog-gen")
	if err != nil {
		t.Fatalf("FindInstance: %v", err)
	}
	if id != "i-abc" || state != "stopped" {
		t.Errorf("id=%q state=%q", id, state)
	}
	// Verify the Project tag filter was applied.
	var hasProjectFilter bool
	for _, filt := range f.describeIn.Filters {
		if aws.ToString(filt.Name) == "tag:Project" && len(filt.Values) == 1 && filt.Values[0] == "blog-gen" {
			hasProjectFilter = true
		}
	}
	if !hasProjectFilter {
		t.Errorf("Project tag filter missing: %+v", f.describeIn.Filters)
	}
}

func TestFindInstanceEmptyWhenNone(t *testing.T) {
	id, _, err := New(&fakeAPI{}).FindInstance(context.Background(), "blog-gen")
	if err != nil {
		t.Fatalf("FindInstance: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty id, got %q", id)
	}
}

func TestStartInstance(t *testing.T) {
	f := &fakeAPI{}
	if err := New(f).StartInstance(context.Background(), "i-xyz"); err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	if len(f.startedIDs) != 1 || f.startedIDs[0] != "i-xyz" {
		t.Errorf("startedIDs = %v", f.startedIDs)
	}
}
