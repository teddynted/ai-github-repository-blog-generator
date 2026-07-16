// Package lifecycle holds the shared EC2 instance data type and port used by
// the platform's instance-management adapters. Instance power itself is owned
// by the scheduler stack (a fixed weekday window driven by the power package);
// this package only models the instance and the operations the adapters expose.
package lifecycle

import (
	"context"
	"time"
)

// Instance is the subset of EC2 instance state callers need. A zero-value ID
// means no matching instance was found.
type Instance struct {
	ID         string
	State      string // pending|running|stopping|stopped
	LaunchTime time.Time
}

// EC2 is the port for the tag-based instance operations. It is satisfied by
// internal/awsec2.Client. FindInstance locates the platform's instance by its
// Project tag rather than a hard-coded ID, which is what lets the webhook
// handler check host availability without a dependency on the compute stack.
type EC2 interface {
	// FindInstance returns the project's instance (empty ID when none exists).
	FindInstance(ctx context.Context, project string) (Instance, error)
	// StartInstance starts the given instance.
	StartInstance(ctx context.Context, id string) error
	// StopInstance stops the given instance.
	StopInstance(ctx context.Context, id string) error
}
