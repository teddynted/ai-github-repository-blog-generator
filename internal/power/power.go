// Package power contains the scheduled start/stop use case for a single,
// explicitly identified EC2 instance. It is deliberately separate from the
// tag-based lifecycle package: the scheduler drives a specific instance by ID
// (from the INSTANCE_ID environment variable), not by locating the platform's
// tagged host.
//
// Both operations are idempotent — they inspect the current instance state and
// issue an API call only when a transition is actually required — so a schedule
// that fires against an already-running (or already-stopped) instance is a
// successful no-op rather than an error.
package power

import (
	"context"
	"fmt"
	"log/slog"
)

// EC2 instance state names, as returned by the EC2 API. Kept here so the use
// case reads intent rather than magic strings.
const (
	statePending      = "pending"
	stateRunning      = "running"
	stateStopping     = "stopping"
	stateStopped      = "stopped"
	stateShuttingDown = "shutting-down"
	stateTerminated   = "terminated"
)

// Instances is the port for the EC2 operations the scheduler needs. It is
// satisfied by internal/awsec2.Client, which the Lambda entry points wire in.
type Instances interface {
	// InstanceState returns the current state name of the instance (e.g.
	// "running", "stopped"). It returns an empty string when the instance does
	// not exist.
	InstanceState(ctx context.Context, id string) (string, error)
	// StartInstance starts the given instance.
	StartInstance(ctx context.Context, id string) error
	// StopInstance stops the given instance.
	StopInstance(ctx context.Context, id string) error
}

// Switch turns a specific EC2 instance on or off on a schedule.
type Switch struct {
	Instances  Instances
	InstanceID string
	Logger     *slog.Logger
}

// EnsureStarted starts the instance unless it is already running or pending.
// It reports whether a start was actually issued.
func (s *Switch) EnsureStarted(ctx context.Context) (changed bool, err error) {
	state, err := s.currentState(ctx)
	if err != nil {
		return false, err
	}

	switch state {
	case stateRunning, statePending:
		s.log("instance already running; no action", state, false)
		return false, nil
	case stateTerminated:
		// A terminated instance cannot be started — surface it rather than
		// silently succeeding, since the schedule now points at a dead ID.
		return false, fmt.Errorf("instance %s is terminated and cannot be started", s.InstanceID)
	default: // stopped, stopping, shutting-down
		if err := s.Instances.StartInstance(ctx, s.InstanceID); err != nil {
			return false, fmt.Errorf("start instance %s: %w", s.InstanceID, err)
		}
		s.log("instance start issued", state, true)
		return true, nil
	}
}

// EnsureStopped stops the instance unless it is already stopped or stopping.
// It reports whether a stop was actually issued.
func (s *Switch) EnsureStopped(ctx context.Context) (changed bool, err error) {
	state, err := s.currentState(ctx)
	if err != nil {
		return false, err
	}

	switch state {
	case stateStopped, stateStopping, stateShuttingDown, stateTerminated:
		s.log("instance already stopped; no action", state, false)
		return false, nil
	default: // running, pending
		if err := s.Instances.StopInstance(ctx, s.InstanceID); err != nil {
			return false, fmt.Errorf("stop instance %s: %w", s.InstanceID, err)
		}
		s.log("instance stop issued", state, true)
		return true, nil
	}
}

// currentState fetches the instance state and rejects a missing instance, which
// almost always means a misconfigured INSTANCE_ID.
func (s *Switch) currentState(ctx context.Context) (string, error) {
	state, err := s.Instances.InstanceState(ctx, s.InstanceID)
	if err != nil {
		return "", fmt.Errorf("describe instance %s: %w", s.InstanceID, err)
	}
	if state == "" {
		return "", fmt.Errorf("instance %s not found", s.InstanceID)
	}
	return state, nil
}

func (s *Switch) log(msg, priorState string, changed bool) {
	if s.Logger != nil {
		s.Logger.Info(msg,
			slog.String("instance_id", s.InstanceID),
			slog.String("prior_state", priorState),
			slog.Bool("changed", changed),
		)
	}
}
