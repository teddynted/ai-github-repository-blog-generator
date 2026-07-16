// Package intake is the trigger-agnostic core that accepts a processing request
// from any source (GitHub webhook, manual REST call, and future CLI/Slack/cron
// triggers) and submits it onto the shared pipeline. It owns the canonical
// Event contract published to EventBridge and the window policy that decides
// whether a request is accepted now, deferred to the next scheduled window, or
// rejected — so no trigger source duplicates that orchestration logic.
//
// The processing engine itself (the worker that drains SQS) is independent of
// how a request was triggered: every trigger publishes the same Event, and the
// worker treats them identically.
package intake

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Event is the payload published to EventBridge (detail "blog.publish.requested")
// and consumed by the worker. Its JSON shape is the stable cross-service
// contract, so field tags must not change lightly. Fields added for newer
// trigger sources are omitempty, keeping older producers/consumers compatible.
type Event struct {
	RepoFullName   string `json:"repo_full_name"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	Ref            string `json:"ref"`
	CommitSHA      string `json:"commit_sha"`
	CommitMessage  string `json:"commit_message"`
	TriggerPattern string `json:"trigger_pattern"`
	// Source identifies the trigger that produced the event (push, release,
	// manual, …) for logging and future routing. Optional for back-compat.
	Source string `json:"source,omitempty"`
	// Force asks the pipeline to process even if the commit was already
	// published (manual re-runs). Carried through for the worker to honour.
	Force bool `json:"force,omitempty"`
	// Provider is an optional AI-provider hint (e.g. "bedrock"); the platform
	// defaults to local Ollama when unset. Carried through for future use.
	Provider string `json:"provider,omitempty"`
}

// Publisher publishes an Event downstream (EventBridge in production). It is
// satisfied by internal/eventbus adapters and reused by every trigger source.
type Publisher interface {
	Publish(ctx context.Context, ev Event) error
}

// Window reports whether the platform is currently inside its operating window,
// i.e. the compute host is running (the scheduler owns that lifecycle). It is
// optional: a nil Window is treated as always-open.
type Window interface {
	Open(ctx context.Context) (bool, error)
}

// Starter ensures the compute host is running, starting it if stopped
// (idempotent). Used by interactive triggers that override the schedule (the
// manual endpoint) to run on demand. Optional.
type Starter interface {
	Start(ctx context.Context) error
}

// Decision is the outcome of submitting a request.
type Decision string

const (
	// Accepted: published and the host is up, so it is processed in this window.
	Accepted Decision = "accepted"
	// Deferred: published while the host is down; it waits in SQS for the next
	// scheduled start (used by buffered triggers such as webhooks).
	Deferred Decision = "deferred"
	// Rejected: not published because the platform is outside its window and the
	// trigger's policy is to reject.
	Rejected Decision = "rejected"
	// Started: the host was stopped, so it was started on demand and the event
	// published; processing begins once the host is healthy.
	Started Decision = "started"
)

// Policy selects what happens when the platform is outside its operating window.
type Policy int

const (
	// BufferOutsideWindow publishes regardless; the event is retained in SQS and
	// drained at the next scheduled start. Used by push/release webhooks.
	BufferOutsideWindow Policy = iota
	// RejectOutsideWindow refuses the request and publishes nothing.
	RejectOutsideWindow
	// StartOutsideWindow starts the host on demand (overriding the schedule),
	// then publishes. Used by the manual trigger so a run can be forced any time.
	StartOutsideWindow
)

// Service submits requests onto the shared pipeline under a window policy. It
// never starts the instance — the scheduler stack is the authority for
// instance power; Service only publishes (or refuses to).
type Service struct {
	Publisher Publisher
	Window    Window  // optional; nil => always open
	Starter   Starter // optional; required for StartOutsideWindow
	Logger    *slog.Logger
}

// Submit applies the window policy and publishes the event unless it must be
// rejected. It reports the decision. A Window error fails open (buffer and
// report Accepted) so a transient describe failure never drops work.
func (s *Service) Submit(ctx context.Context, ev Event, policy Policy) (Decision, error) {
	open := true
	if s.Window != nil {
		o, err := s.Window.Open(ctx)
		switch {
		case err != nil:
			s.log(ev, "accepted", "window check failed, treating as open: "+err.Error())
			open = true
		default:
			open = o
		}
	}

	if open {
		if err := s.Publisher.Publish(ctx, ev); err != nil {
			return "", fmt.Errorf("publish: %w", err)
		}
		s.log(ev, "accepted", "instance running, processing in this window")
		return Accepted, nil
	}

	// Outside the operating window (host stopped).
	switch policy {
	case RejectOutsideWindow:
		s.log(ev, "rejected", "outside operational window")
		return Rejected, nil
	case StartOutsideWindow:
		// Override the schedule: start the host on demand, then publish. A start
		// failure is not fatal — the event still buffers in SQS and runs at the
		// next scheduled start — but it is logged and reported as deferred.
		if s.Starter == nil {
			return "", fmt.Errorf("StartOutsideWindow requires a Starter")
		}
		startErr := s.Starter.Start(ctx)
		if err := s.Publisher.Publish(ctx, ev); err != nil {
			return "", fmt.Errorf("publish: %w", err)
		}
		if startErr != nil {
			s.log(ev, "deferred", "instance start failed, buffered for next scheduled runtime: "+startErr.Error())
			return Deferred, nil
		}
		s.log(ev, "started", "instance started on demand; processing will begin when healthy")
		return Started, nil
	default: // BufferOutsideWindow
		if err := s.Publisher.Publish(ctx, ev); err != nil {
			return "", fmt.Errorf("publish: %w", err)
		}
		s.log(ev, "deferred", "outside operational window, buffered for next scheduled runtime")
		return Deferred, nil
	}
}

func (s *Service) log(ev Event, decision, detail string) {
	if s.Logger == nil {
		return
	}
	s.Logger.Info("intake submit",
		slog.String("decision", decision),
		slog.String("source", ev.Source),
		slog.String("repo", ev.RepoFullName),
		slog.String("ref", ev.Ref),
		slog.String("provider", ev.Provider),
		slog.String("detail", detail))
}

// Request is a trigger-agnostic processing request as supplied by an
// interactive source (the manual REST endpoint today; CLI/Slack later). It is
// validated and mapped to an Event via ToEvent, so new sources reuse the same
// validation and contract.
type Request struct {
	Repository string `json:"repository"`
	Owner      string `json:"owner"`
	Branch     string `json:"branch,omitempty"`
	Commit     string `json:"commit,omitempty"`
	Force      bool   `json:"force,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

// Validate reports the required fields that are missing. Repository and owner
// are required; branch defaults to "main" and the rest are optional.
func (r Request) Validate() error {
	var missing []string
	if strings.TrimSpace(r.Repository) == "" {
		missing = append(missing, "repository")
	}
	if strings.TrimSpace(r.Owner) == "" {
		missing = append(missing, "owner")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// ToEvent maps a validated Request to an Event, tagging it with the trigger
// source. Branch defaults to main; an unset commit is left empty for the
// pipeline to resolve.
func (r Request) ToEvent(source string) Event {
	branch := strings.TrimSpace(r.Branch)
	if branch == "" {
		branch = "main"
	}
	return Event{
		RepoFullName:   r.Owner + "/" + r.Repository,
		Owner:          r.Owner,
		Name:           r.Repository,
		Ref:            "refs/heads/" + branch,
		CommitSHA:      strings.TrimSpace(r.Commit),
		CommitMessage:  source + " trigger",
		TriggerPattern: source,
		Source:         source,
		Force:          r.Force,
		Provider:       strings.TrimSpace(r.Provider),
	}
}
