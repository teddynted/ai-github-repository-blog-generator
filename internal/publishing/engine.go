package publishing

import (
	"context"
	"log/slog"
	"time"
)

// Engine is the application layer: it distributes approved content to multiple
// platforms, honouring scheduling, retry, tracking, and notifications. It
// depends only on the Publisher, Repository, and Notifier ports.
type Engine struct {
	Config     Config
	Repo       Repository
	Notifier   Notifier
	Metadata   MetadataEngine
	Validation ValidationEngine
	Scheduler  Scheduler
	Tracker    Tracker
	Now        Clock
	Sleep      Sleeper
	Logger     *slog.Logger

	publishers map[Platform]Publisher
}

// NewEngine wires an Engine. A nil repository/notifier get in-memory/no-op
// defaults; a nil clock uses time.Now.
func NewEngine(cfg Config, repo Repository, publishers []Publisher, now Clock) *Engine {
	if now == nil {
		now = time.Now
	}
	if repo == nil {
		repo = NewMemoryRepository()
	}
	e := &Engine{
		Config:     cfg,
		Repo:       repo,
		Notifier:   nopNotifier{},
		Metadata:   MetadataEngine{Config: cfg},
		Validation: ValidationEngine{},
		Scheduler:  Scheduler{Now: now},
		Tracker:    Tracker{Repo: repo, Now: now},
		Now:        now,
		Sleep:      realSleeper,
		publishers: map[Platform]Publisher{},
	}
	for _, p := range publishers {
		e.publishers[p.Name()] = p
	}
	return e
}

// Register adds or replaces a publisher (a new platform, no core changes).
func (e *Engine) Register(p Publisher) { e.publishers[p.Name()] = p }

// DistributeRequest configures a multi-platform distribution.
type DistributeRequest struct {
	Targets  []Platform // empty → all enabled publishers that support the type
	Schedule Schedule
}

// Distribute publishes approved content to the requested platforms in the
// configured order. It never publishes unapproved content. Each platform gets an
// independent, tracked Publication; a failure on one does not stop the others
// (when ContinueOnError). Returns every Publication created.
func (e *Engine) Distribute(ctx context.Context, c Content, req DistributeRequest) ([]*Publication, error) {
	if !c.Approved {
		e.notify(ctx, Event{Kind: EventApprovalMissing, Content: c.ID, Message: "content is not approved"})
		return nil, ErrNotApproved
	}

	targets := e.orderedTargets(c, req.Targets)
	if len(targets) == 0 {
		return nil, ErrNoPublisher
	}

	var pubs []*Publication
	for _, platform := range targets {
		pub := e.distributeOne(ctx, c, platform, req.Schedule)
		pubs = append(pubs, pub)
		if pub.Status == StatusFailed && !e.Config.ContinueOnError {
			break
		}
	}
	return pubs, nil
}

// distributeOne creates and runs a single-platform publication.
func (e *Engine) distributeOne(ctx context.Context, c Content, platform Platform, sc Schedule) *Publication {
	meta := e.Metadata.For(platform, c)
	pub := &Publication{
		ID:             c.ID + ":" + string(platform),
		ContentID:      c.ID,
		ContentType:    c.Type,
		Platform:       platform,
		Status:         StatusPending,
		ApprovalRef:    c.ApprovalRef,
		ReleaseVersion: c.ReleaseVersion,
		Author:         c.Author,
		Metadata:       meta,
		CreatedAt:      e.Now(),
		UpdatedAt:      e.Now(),
	}
	_ = e.Repo.Save(pub)

	publisher := e.publishers[platform]

	// Validate before doing anything.
	if err := e.Validation.Validate(c, meta, publisher); err != nil {
		e.fail(ctx, pub, err)
		return pub
	}
	if !e.Config.isEnabled(platform) {
		e.fail(ctx, pub, ErrPublisherDisabled)
		return pub
	}

	// Scheduling: future publications are parked as Scheduled for RunDue.
	next := e.Scheduler.NextRun(sc, e.Now())
	if sc.Mode != ScheduleImmediate && sc.Mode != "" && next.After(e.Now()) {
		pub.ScheduledFor = &next
		_ = e.Tracker.transition(pub, StatusScheduled, "schedule", "scheduled for "+next.Format(time.RFC3339))
		e.notify(ctx, Event{Kind: EventScheduled, Platform: platform, Content: c.ID, Message: "scheduled for " + next.Format(time.RFC3339)})
		return pub
	}

	e.runPublish(ctx, c, pub, publisher)
	return pub
}

// RunDue publishes every scheduled publication whose time has arrived. A
// scheduler/cron (or n8n) calls this periodically.
func (e *Engine) RunDue(ctx context.Context, contentByID func(id string) (Content, bool)) (int, error) {
	scheduled, err := e.Repo.ByStatus(StatusScheduled)
	if err != nil {
		return 0, err
	}
	ran := 0
	for _, pub := range scheduled {
		if pub.ScheduledFor == nil || !e.Scheduler.Due(*pub.ScheduledFor) {
			continue
		}
		c, ok := contentByID(pub.ContentID)
		if !ok {
			e.fail(ctx, pub, errMissingAsset("content no longer available for scheduled publication"))
			continue
		}
		_ = e.Tracker.transition(pub, StatusPublishing, "run-due", "scheduled time reached")
		e.runPublish(ctx, c, pub, e.publishers[pub.Platform])
		ran++
	}
	return ran, nil
}

// runPublish executes the publish with retry + tracking.
func (e *Engine) runPublish(ctx context.Context, c Content, pub *Publication, publisher Publisher) {
	if publisher == nil {
		e.fail(ctx, pub, ErrNoPublisher)
		return
	}
	if pub.Status != StatusPublishing {
		if err := e.Tracker.transition(pub, StatusPublishing, "publish", "starting publication"); err != nil {
			e.fail(ctx, pub, err)
			return
		}
	}

	retry := RetryEngine{
		Policy: e.Config.Retry,
		Sleep:  e.Sleep,
		OnRetry: func(attempt int, err error, delay time.Duration) {
			pub.RetryCount = attempt
			_ = e.Tracker.transition(pub, StatusRetrying, "retry", "attempt "+itoa(attempt)+" ("+errCode(err)+"), backoff "+delay.String())
			_ = e.Tracker.transition(pub, StatusPublishing, "retry-run", "retrying")
			e.notify(ctx, Event{Kind: EventRetry, Platform: pub.Platform, Content: c.ID, Message: errCode(err)})
		},
	}

	var result PublicationResult
	start := e.Now()
	attempts, err := retry.Do(ctx, func(ctx context.Context) error {
		aStart := e.Now()
		cctx := ctx
		if e.Config.Timeout > 0 {
			var cancel context.CancelFunc
			cctx, cancel = context.WithTimeout(ctx, e.Config.Timeout)
			defer cancel()
		}
		r, perr := publisher.Publish(cctx, c, pub.Metadata)
		pub.Attempts = append(pub.Attempts, PublicationAttempt{
			Number: len(pub.Attempts) + 1, StartedAt: aStart, FinishedAt: e.Now(),
			Success: perr == nil, Error: errString(perr), Recoverable: perr != nil && isRecoverable(perr),
			Duration: e.Now().Sub(aStart),
		})
		if perr != nil {
			return perr
		}
		result = r
		return nil
	})
	pub.DurationMs = e.Now().Sub(start).Milliseconds()
	pub.RetryCount = attempts - 1

	if err != nil {
		e.fail(ctx, pub, err)
		return
	}

	now := e.Now()
	pub.PlatformID = result.PlatformID
	pub.URL = result.URL
	pub.PublishedAt = &now
	_ = e.Tracker.transition(pub, StatusPublished, "published", result.URL)
	e.notify(ctx, Event{Kind: EventPublished, Platform: pub.Platform, Content: c.ID, URL: result.URL})
}

// fail records a terminal failure with a safe, meaningful message.
func (e *Engine) fail(ctx context.Context, pub *Publication, err error) {
	pub.Errors = append(pub.Errors, err.Error())
	_ = e.Tracker.transition(pub, StatusFailed, "failed", errCode(err)+": "+err.Error())
	kind := EventFailed
	if errCode(err) == "quota" {
		kind = EventQuotaExceeded
	}
	e.notify(ctx, Event{Kind: kind, Platform: pub.Platform, Content: pub.ContentID, Message: err.Error()})
}

// Cancel cancels a pending/scheduled/failed publication.
func (e *Engine) Cancel(id string) (*Publication, error) {
	pub, err := e.Repo.Get(id)
	if err != nil {
		return nil, err
	}
	if err := e.Tracker.transition(pub, StatusCancelled, "cancel", "cancelled by request"); err != nil {
		return nil, err
	}
	return pub, nil
}

// Get / List expose publications.
func (e *Engine) Get(id string) (*Publication, error) { return e.Repo.Get(id) }
func (e *Engine) List() ([]*Publication, error)       { return e.Repo.List() }

// orderedTargets resolves and orders the target platforms.
func (e *Engine) orderedTargets(c Content, requested []Platform) []Platform {
	// Candidate set.
	want := map[Platform]bool{}
	if len(requested) > 0 {
		for _, p := range requested {
			want[p] = true
		}
	} else {
		for p, pub := range e.publishers {
			if e.Config.isEnabled(p) && pub.Supports(c.Type) {
				want[p] = true
			}
		}
	}
	// Order by config priority, then any remaining.
	var out []Platform
	seen := map[Platform]bool{}
	for _, p := range e.Config.Priority {
		if want[p] && !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	for p := range want {
		if !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	return out
}

func (e *Engine) notify(ctx context.Context, ev Event) {
	if e.Notifier != nil {
		_ = e.Notifier.Notify(ctx, ev)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
