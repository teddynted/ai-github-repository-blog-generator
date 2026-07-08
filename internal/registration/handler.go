package registration

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
)

// Handler adapts the Service to a transport-agnostic JSON request/response,
// so the Lambda entry point stays a thin wrapper over API Gateway types.
type Handler struct {
	Service *Service
	Logger  *slog.Logger
}

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// HandleJSON processes a registration request body and returns an HTTP status
// and a JSON response body. It never returns Go errors — failures are encoded
// into the response — so the caller can respond uniformly.
func (h *Handler) HandleJSON(ctx context.Context, body []byte) (int, []byte) {
	var in Input
	if err := json.Unmarshal(body, &in); err != nil {
		return h.fail(apperror.New(apperror.CodeInvalidInput, "request body must be valid JSON"))
	}

	out, err := h.Service.Register(ctx, in)
	if err != nil {
		if h.Logger != nil {
			// Never log the PAT; log the outcome and code only.
			h.Logger.Warn("registration failed",
				slog.String("code", string(apperror.CodeOf(err))),
				slog.String("error", err.Error()))
		}
		return h.fail(err)
	}

	if h.Logger != nil {
		h.Logger.Info("repository registered",
			slog.String("repo", out.RepoFullName),
			slog.Int64("webhook_id", out.WebhookID))
	}
	resp, _ := json.Marshal(out)
	return 200, resp
}

func (h *Handler) fail(err error) (int, []byte) {
	body, _ := json.Marshal(errorBody{
		Error:   string(apperror.CodeOf(err)),
		Message: clientMessage(err),
	})
	return apperror.HTTPStatusOf(err), body
}

// clientMessage returns the safe, client-facing message for an error.
func clientMessage(err error) string {
	var e *apperror.Error
	if errors.As(err, &e) {
		return e.Message
	}
	return "internal error"
}
