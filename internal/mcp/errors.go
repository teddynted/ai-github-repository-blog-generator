package mcp

import "errors"

// Sentinel errors (callers use errors.Is).
var (
	// ErrServerNotFound — no server registered for an id.
	ErrServerNotFound = errors.New("mcp: server not found")
	// ErrToolNotFound — the server has no such tool.
	ErrToolNotFound = errors.New("mcp: tool not found")
	// ErrResourceNotFound — the server has no such resource.
	ErrResourceNotFound = errors.New("mcp: resource not found")
	// ErrPromptNotFound — the server has no such prompt.
	ErrPromptNotFound = errors.New("mcp: prompt not found")
	// ErrForbidden — the caller lacks the required permission (authorization).
	ErrForbidden = errors.New("mcp: forbidden")
	// ErrUnauthenticated — a required credential is missing/invalid.
	ErrUnauthenticated = errors.New("mcp: unauthenticated")
	// ErrInvalidArgument — a tool argument failed validation.
	ErrInvalidArgument = errors.New("mcp: invalid argument")
	// ErrAlreadyRegistered — a server id was registered twice.
	ErrAlreadyRegistered = errors.New("mcp: server already registered")
	// ErrTimeout — a tool call exceeded its deadline.
	ErrTimeout = errors.New("mcp: tool call timed out")
	// ErrRateLimited — the caller exceeded the rate limit.
	ErrRateLimited = errors.New("mcp: rate limited")
	// ErrUnsupported — the server does not support this capability.
	ErrUnsupported = errors.New("mcp: capability not supported")
)

// InvocationError classifies a failed tool invocation for retry decisions.
type InvocationError struct {
	Server      string
	Tool        string
	Recoverable bool
	Err         error
}

func (e *InvocationError) Error() string {
	return "mcp: " + e.Server + "/" + e.Tool + ": " + e.Err.Error()
}

func (e *InvocationError) Unwrap() error { return e.Err }

// recoverable reports whether an error should be retried.
func recoverable(err error) bool {
	var ie *InvocationError
	if errors.As(err, &ie) {
		return ie.Recoverable
	}
	// Timeouts and rate limits are transient.
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrRateLimited)
}
