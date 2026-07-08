package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Queue is the port for reading the events queue depth (visible + in-flight).
type Queue interface {
	Depth(ctx context.Context) (int64, error)
}

// Shutdowner stops the project's instance once it has been up beyond the idle
// timeout and the events queue is empty (no visible or in-flight messages).
// It is invoked periodically by the EventBridge idle timer.
type Shutdowner struct {
	EC2         EC2
	Queue       Queue
	Project     string
	IdleTimeout time.Duration
	Now         func() time.Time
	Logger      *slog.Logger
}

// StopIfIdle stops the instance when it is idle, reporting whether a stop was
// issued. It is idempotent and safe to call repeatedly: a stopped or
// recently-started instance, or a non-empty queue, is a no-op.
func (s *Shutdowner) StopIfIdle(ctx context.Context) (stopped bool, err error) {
	inst, err := s.EC2.FindInstance(ctx, s.Project)
	if err != nil {
		return false, fmt.Errorf("find instance: %w", err)
	}
	if inst.ID == "" || inst.State != "running" {
		// Nothing to stop (absent, already stopped/stopping, or still pending).
		return false, nil
	}

	// Give a freshly-started instance the idle window to pick up and finish work.
	if uptime := s.now().Sub(inst.LaunchTime); uptime < s.IdleTimeout {
		s.log("within idle window; not stopping", inst.ID, uptime)
		return false, nil
	}

	depth, err := s.Queue.Depth(ctx)
	if err != nil {
		return false, fmt.Errorf("queue depth: %w", err)
	}
	if depth > 0 {
		s.log("queue not empty; not stopping", inst.ID, time.Duration(depth))
		return false, nil
	}

	if err := s.EC2.StopInstance(ctx, inst.ID); err != nil {
		return false, fmt.Errorf("stop instance %s: %w", inst.ID, err)
	}
	s.log("instance stop issued (idle)", inst.ID, 0)
	return true, nil
}

func (s *Shutdowner) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Shutdowner) log(msg, id string, d time.Duration) {
	if s.Logger != nil {
		s.Logger.Info(msg, slog.String("instance_id", id), slog.Duration("value", d))
	}
}
