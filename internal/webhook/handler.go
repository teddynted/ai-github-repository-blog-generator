// Package webhook implements the lightweight GitHub webhook receiver: resolve
// the repository's metadata, verify the HMAC signature with that repo's secret,
// and evaluate the commit-message trigger. On a match it publishes an event via
// the Publisher port; it performs no cloning, analysis, or AI work, and never
// reads the repository PAT.
package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/githubsig"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/repo"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/trigger"
)

// RepoLookup resolves repository metadata by "owner/name".
type RepoLookup interface {
	Get(ctx context.Context, fullName string) (repo.Repository, bool, error)
}

// SecretGetter retrieves a repository's webhook signing secret by reference.
type SecretGetter interface {
	WebhookSecret(ctx context.Context, ref string) (string, error)
}

// Publisher publishes a matched event downstream (EventBridge in production).
type Publisher interface {
	Publish(ctx context.Context, ev Event) error
}

// Counter emits a named count metric. *metrics.Emitter satisfies it. Optional.
type Counter interface {
	Count(name string)
}

// Event is the payload published when a commit matches the trigger.
type Event struct {
	RepoFullName   string `json:"repo_full_name"`
	Owner          string `json:"owner"`
	Name           string `json:"name"`
	Ref            string `json:"ref"`
	CommitSHA      string `json:"commit_sha"`
	CommitMessage  string `json:"commit_message"`
	TriggerPattern string `json:"trigger_pattern"`
}

// Handler processes webhook deliveries.
type Handler struct {
	Repos          RepoLookup
	Secrets        SecretGetter
	Publisher      Publisher
	Metrics        Counter // optional
	DefaultTrigger string
	Logger         *slog.Logger
}

func (h *Handler) count(name string) {
	if h.Metrics != nil {
		h.Metrics.Count(name)
	}
}

// pushPayload is the minimal subset of the GitHub push event we parse.
type pushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	HeadCommit *struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"head_commit"`
}

// releasePayload is the minimal subset of the GitHub release event we parse.
type releasePayload struct {
	Action  string `json:"action"`
	Release struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
	} `json:"release"`
}

type result struct {
	Status string `json:"status"`
	Repo   string `json:"repo,omitempty"`
}

// Handle processes one delivery and returns an HTTP status and JSON body. It
// never returns a Go error: outcomes (accepted, ignored, rejected) are encoded
// in the response so the Lambda wrapper can reply uniformly.
func (h *Handler) Handle(ctx context.Context, headers map[string]string, body []byte) (int, []byte) {
	hdr := normalizeHeaders(headers)
	eventType := hdr["x-github-event"]
	deliveryID := hdr["x-github-delivery"]
	h.count("WebhookReceived")

	// Parse just enough to identify the repository and select its secret.
	var p pushPayload
	if err := json.Unmarshal(body, &p); err != nil || p.Repository.FullName == "" {
		return jsonResp(apperror.New(apperror.CodeInvalidInput, "unrecognised webhook payload"))
	}
	full := p.Repository.FullName

	r, ok, err := h.Repos.Get(ctx, full)
	if err != nil {
		return jsonResp(apperror.Wrap(err, apperror.CodeInternal, "metadata lookup failed"))
	}
	if !ok {
		return jsonResp(apperror.New(apperror.CodeNotFound, "repository is not registered"))
	}

	secret, err := h.Secrets.WebhookSecret(ctx, r.SecretRef)
	if err != nil {
		return jsonResp(apperror.Wrap(err, apperror.CodeInternal, "secret lookup failed"))
	}

	// Verify the signature over the raw body before trusting the payload.
	if !githubsig.Verify(secret, body, hdr["x-hub-signature-256"]) {
		h.log("rejected", deliveryID, full, "invalid signature")
		h.count("WebhookRejected")
		return jsonResp(apperror.New(apperror.CodeUnauthorized, "invalid signature"))
	}

	// Signature valid. Build a trigger event for supported event types; anything
	// we choose not to process is a 200 "ignored".
	if !r.Enabled {
		return h.ignore(deliveryID, full, "repository disabled")
	}

	var ev Event
	var skip string
	switch eventType {
	case "push":
		ev, skip = h.pushEvent(full, p, r)
	case "release":
		ev, skip = h.releaseEvent(full, body, r)
	default:
		skip = "unsupported event: " + eventType
	}
	if skip != "" {
		return h.ignore(deliveryID, full, skip)
	}

	if err := h.Publisher.Publish(ctx, ev); err != nil {
		return jsonResp(apperror.Wrap(err, apperror.CodeInternal, "publish failed"))
	}
	h.log("published", deliveryID, full, "triggered by "+eventType)
	h.count("TriggerMatched")
	return okResp("accepted", full)
}

// pushEvent builds a trigger event for a push, gated on the commit-message
// trigger. A non-empty second return value is the reason to ignore.
func (h *Handler) pushEvent(full string, p pushPayload, r repo.Repository) (Event, string) {
	if p.HeadCommit == nil {
		return Event{}, "no head commit"
	}
	pattern := r.TriggerPattern
	if strings.TrimSpace(pattern) == "" {
		pattern = h.DefaultTrigger
	}
	if !trigger.Matches(p.HeadCommit.Message, pattern) {
		return Event{}, "commit does not match trigger"
	}
	return Event{
		RepoFullName:   full,
		Owner:          r.Owner,
		Name:           r.Name,
		Ref:            p.Ref,
		CommitSHA:      p.HeadCommit.ID,
		CommitMessage:  p.HeadCommit.Message,
		TriggerPattern: pattern,
	}, ""
}

// releaseEvent builds a trigger event for a published release. A release is
// itself an intentional event, so it always triggers (no commit-message gate).
func (h *Handler) releaseEvent(full string, body []byte, r repo.Repository) (Event, string) {
	var rp releasePayload
	if err := json.Unmarshal(body, &rp); err != nil {
		return Event{}, "unparseable release payload"
	}
	if rp.Action != "published" {
		return Event{}, "release action not published: " + rp.Action
	}
	name := rp.Release.Name
	if name == "" {
		name = rp.Release.TagName
	}
	return Event{
		RepoFullName:   full,
		Owner:          r.Owner,
		Name:           r.Name,
		Ref:            "refs/tags/" + rp.Release.TagName,
		CommitSHA:      rp.Release.TagName, // memory dedup key for releases
		CommitMessage:  "Release " + name,
		TriggerPattern: "release",
	}, ""
}

// ignore logs, counts, and returns the ignored response for a delivery.
func (h *Handler) ignore(deliveryID, repoName, reason string) (int, []byte) {
	h.log("ignored", deliveryID, repoName, reason)
	h.count("WebhookIgnored")
	return okResp("ignored", repoName)
}

func (h *Handler) log(outcome, deliveryID, repoName, detail string) {
	if h.Logger == nil {
		return
	}
	h.Logger.Info("webhook processed",
		slog.String("outcome", outcome),
		slog.String("delivery_id", deliveryID),
		slog.String("repo", repoName),
		slog.String("detail", detail))
}

func normalizeHeaders(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[strings.ToLower(k)] = v
	}
	return out
}

func okResp(status, repoName string) (int, []byte) {
	b, _ := json.Marshal(result{Status: status, Repo: repoName})
	return 200, b
}

func jsonResp(err error) (int, []byte) {
	code := apperror.CodeOf(err)
	// Do not leak internal detail to clients.
	msg := "internal error"
	if code != apperror.CodeInternal {
		var e *apperror.Error
		if errors.As(err, &e) {
			msg = e.Message
		}
	}
	b, _ := json.Marshal(map[string]string{"error": string(code), "message": msg})
	return apperror.HTTPStatusOf(err), b
}
