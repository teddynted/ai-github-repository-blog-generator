package registration

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// Handler adapts the Service to transport-agnostic JSON request/response, so the
// Lambda entry point stays a thin wrapper over API Gateway types.
type Handler struct {
	Service *Service
	Logger  *slog.Logger
}

// response is the uniform API body: {status, message, repository?}.
type response struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	Repository string `json:"repository,omitempty"`
}

// Register handles a POST /repositories body: validate, register or update, and
// return {status,message,repository}. It never returns a Go error — outcomes are
// encoded in the response.
func (h *Handler) Register(ctx context.Context, body []byte) (int, []byte) {
	var in Input
	if err := json.Unmarshal(body, &in); err != nil {
		return h.fail("register", "", "", apperror.New(apperror.CodeInvalidInput, "request body must be valid JSON"))
	}

	out, err := h.Service.Register(ctx, in)
	if err != nil {
		return h.fail("register", in.Owner, in.Repository, err)
	}

	action, msg := "registered", "Repository registered successfully."
	if out.Updated {
		action, msg = "updated", "Repository credentials updated successfully."
	}
	h.log("success", action, in.Owner, in.Repository, "secret upserted")
	return ok(response{Status: "success", Message: msg, Repository: out.RepoFullName})
}

// Delete handles a DELETE /repositories body: {owner, repository}.
func (h *Handler) Delete(ctx context.Context, body []byte) (int, []byte) {
	var in DeleteInput
	if err := json.Unmarshal(body, &in); err != nil {
		return h.fail("delete", "", "", apperror.New(apperror.CodeInvalidInput, "request body must be valid JSON"))
	}

	full, err := h.Service.Delete(ctx, in)
	if err != nil {
		return h.fail("delete", in.Owner, in.Repository, err)
	}
	h.log("success", "deleted", in.Owner, in.Repository, "secret entry removed")
	return ok(response{Status: "success", Message: "Repository deleted successfully.", Repository: full})
}

func (h *Handler) fail(action, owner, repository string, err error) (int, []byte) {
	// Log the outcome and error code only — never the PAT, webhook secret, or body.
	h.log("failure", action, owner, repository, string(apperror.CodeOf(err)))
	b, _ := json.Marshal(response{Status: "error", Message: clientMessage(err)})
	return apperror.HTTPStatusOf(err), b
}

func (h *Handler) log(outcome, action, owner, repository, secretStatus string) {
	if h.Logger == nil {
		return
	}
	h.Logger.Info("repository registration",
		slog.String("outcome", outcome),
		slog.String("action", action),
		slog.String("owner", owner),
		slog.String("repository", repository),
		slog.String("secret_status", secretStatus))
}

func ok(r response) (int, []byte) {
	b, _ := json.Marshal(r)
	return 200, b
}

// clientMessage returns the safe, client-facing message for an error.
func clientMessage(err error) string {
	var e *apperror.Error
	if errors.As(err, &e) && apperror.CodeOf(err) != apperror.CodeInternal {
		return e.Message
	}
	return "internal error"
}
