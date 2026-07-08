// Package lifecycle contains the EC2 Spot instance lifecycle use cases. The
// Starter ensures the platform's tagged instance is running when a matched
// event arrives; the idle-shutdown use case is added in Milestone 6.
package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
)

// EC2 is the port for the instance operations the Starter needs. The instance
// is located by its Project tag rather than a hard-coded ID (see the compute
// stack), which is what breaks the serverless/compute dependency cycle.
type EC2 interface {
	// FindInstance returns the id and state name of the project's instance, or
	// an empty id when none exists.
	FindInstance(ctx context.Context, project string) (id, state string, err error)
	// StartInstance starts the given instance.
	StartInstance(ctx context.Context, id string) error
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
	id, state, err := s.EC2.FindInstance(ctx, s.Project)
	if err != nil {
		return false, fmt.Errorf("find instance: %w", err)
	}
	if id == "" {
		return false, fmt.Errorf("no instance tagged Project=%s", s.Project)
	}

	switch state {
	case "running", "pending":
		s.log("instance already running", id, state)
		return false, nil
	default: // stopped, stopping, shutting-down
		if err := s.EC2.StartInstance(ctx, id); err != nil {
			return false, fmt.Errorf("start instance %s: %w", id, err)
		}
		s.log("instance start issued", id, state)
		return true, nil
	}
}

func (s *Starter) log(msg, id, state string) {
	if s.Logger != nil {
		s.Logger.Info(msg, slog.String("instance_id", id), slog.String("prior_state", state))
	}
}
