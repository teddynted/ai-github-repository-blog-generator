// Package lifecycle contains the EC2 Spot instance lifecycle use cases. The
// Starter ensures the platform's tagged instance is running when a matched
// event arrives; the idle-shutdown use case is added in Milestone 6.
package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Instance is the subset of EC2 instance state the lifecycle use cases need.
// A zero-value ID means no matching instance was found.
type Instance struct {
	ID         string
	State      string // pending|running|stopping|stopped
	LaunchTime time.Time
}

// EC2 is the port for the instance operations the lifecycle use cases need. The
// instance is located by its Project tag rather than a hard-coded ID (see the
// compute stack), which is what breaks the serverless/compute dependency cycle.
type EC2 interface {
	// FindInstance returns the project's instance (empty ID when none exists).
	FindInstance(ctx context.Context, project string) (Instance, error)
	// StartInstance starts the given instance.
	StartInstance(ctx context.Context, id string) error
	// StopInstance stops the given instance.
	StopInstance(ctx context.Context, id string) error
}

// Starter ensures the project's instance is running.
type Starter struct {
	EC2     EC2
	Project string
	Logger  *slog.Logger
}

// EnsureRunning starts the instance unless it is already running/pending. It
// reports whether a start was issued. It is idempotent: concurrent matched
// events simply observe a running instance.
func (s *Starter) EnsureRunning(ctx context.Context) (started bool, err error) {
	inst, err := s.EC2.FindInstance(ctx, s.Project)
	if err != nil {
		return false, fmt.Errorf("find instance: %w", err)
	}
	if inst.ID == "" {
		return false, fmt.Errorf("no instance tagged Project=%s", s.Project)
	}

	switch inst.State {
	case "running", "pending":
		s.log("instance already running", inst.ID, inst.State)
		return false, nil
	default: // stopped, stopping, shutting-down
		if err := s.EC2.StartInstance(ctx, inst.ID); err != nil {
			return false, fmt.Errorf("start instance %s: %w", inst.ID, err)
		}
		s.log("instance start issued", inst.ID, inst.State)
		return true, nil
	}
}

func (s *Starter) log(msg, id, state string) {
	if s.Logger != nil {
		s.Logger.Info(msg, slog.String("instance_id", id), slog.String("prior_state", state))
	}
}
