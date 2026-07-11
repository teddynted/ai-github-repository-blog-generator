package awsec2

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type fakeAPI struct {
	describeIn *ec2.DescribeInstancesInput
	reservs    []ec2types.Reservation
	startedIDs []string
	stoppedIDs []string
}

func (f *fakeAPI) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.describeIn = in
	return &ec2.DescribeInstancesOutput{Reservations: f.reservs}, nil
}
func (f *fakeAPI) StartInstances(_ context.Context, in *ec2.StartInstancesInput, _ ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error) {
	f.startedIDs = append(f.startedIDs, in.InstanceIds...)
	return &ec2.StartInstancesOutput{}, nil
}
func (f *fakeAPI) StopInstances(_ context.Context, in *ec2.StopInstancesInput, _ ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error) {
	f.stoppedIDs = append(f.stoppedIDs, in.InstanceIds...)
	return &ec2.StopInstancesOutput{}, nil
}

func TestFindInstanceReturnsIDAndState(t *testing.T) {
	launch := time.Unix(1700000000, 0)
	f := &fakeAPI{reservs: []ec2types.Reservation{{
		Instances: []ec2types.Instance{{
			InstanceId: aws.String("i-abc"),
			State:      &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
			LaunchTime: &launch,
		}},
	}}}
	inst, err := New(f).FindInstance(context.Background(), "blog-gen")
	if err != nil {
		t.Fatalf("FindInstance: %v", err)
	}
	if inst.ID != "i-abc" || inst.State != "stopped" || !inst.LaunchTime.Equal(launch) {
		t.Errorf("inst=%+v", inst)
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
	inst, err := New(&fakeAPI{}).FindInstance(context.Background(), "blog-gen")
	if err != nil {
		t.Fatalf("FindInstance: %v", err)
	}
	if inst.ID != "" {
		t.Errorf("expected empty id, got %q", inst.ID)
	}
}

func TestInstanceStateReturnsState(t *testing.T) {
	f := &fakeAPI{reservs: []ec2types.Reservation{{
		Instances: []ec2types.Instance{{
			InstanceId: aws.String("i-abc"),
			State:      &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		}},
	}}}
	state, err := New(f).InstanceState(context.Background(), "i-abc")
	if err != nil {
		t.Fatalf("InstanceState: %v", err)
	}
	if state != "running" {
		t.Errorf("state = %q, want running", state)
	}
	if len(f.describeIn.InstanceIds) != 1 || f.describeIn.InstanceIds[0] != "i-abc" {
		t.Errorf("expected lookup by instance id, got %+v", f.describeIn.InstanceIds)
	}
}

func TestInstanceStateEmptyWhenAbsent(t *testing.T) {
	state, err := New(&fakeAPI{}).InstanceState(context.Background(), "i-missing")
	if err != nil {
		t.Fatalf("InstanceState: %v", err)
	}
	if state != "" {
		t.Errorf("expected empty state for absent instance, got %q", state)
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

func TestStopInstance(t *testing.T) {
	f := &fakeAPI{}
	if err := New(f).StopInstance(context.Background(), "i-xyz"); err != nil {
		t.Fatalf("StopInstance: %v", err)
	}
	if len(f.stoppedIDs) != 1 || f.stoppedIDs[0] != "i-xyz" {
		t.Errorf("stoppedIDs = %v", f.stoppedIDs)
	}
}
