package publishing

import "errors"

// PubError is a classified publishing error. Recoverable errors are retried;
// permanent ones fail immediately.
type PubError struct {
	Code        string // network | timeout | rate-limit | quota | auth | duplicate | invalid | missing-asset | platform-down | unsupported | not-approved
	Message     string
	Recoverable bool
	Err         error // underlying cause (never exposes secrets)
}

func (e *PubError) Error() string {
	if e.Err != nil {
		return "publishing: " + e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return "publishing: " + e.Code + ": " + e.Message
}

func (e *PubError) Unwrap() error { return e.Err }

// newErr builds a PubError.
func newErr(code, msg string, recoverable bool, cause error) *PubError {
	return &PubError{Code: code, Message: msg, Recoverable: recoverable, Err: cause}
}

// Recoverable-error constructors.
func errNetwork(cause error) *PubError   { return newErr("network", "network failure", true, cause) }
func errTimeout(cause error) *PubError   { return newErr("timeout", "request timed out", true, cause) }
func errRateLimit(msg string) *PubError  { return newErr("rate-limit", msg, true, nil) }
func errPlatformDown(m string) *PubError { return newErr("platform-down", m, true, nil) }

// Permanent-error constructors.
func errAuth(msg string) *PubError       { return newErr("auth", msg, false, nil) }
func errQuota(msg string) *PubError      { return newErr("quota", msg, false, nil) }
func errDuplicate(msg string) *PubError  { return newErr("duplicate", msg, false, nil) }
func errInvalid(msg string) *PubError    { return newErr("invalid", msg, false, nil) }
func errMissingAsset(m string) *PubError { return newErr("missing-asset", m, false, nil) }
func errUnsupported(m string) *PubError  { return newErr("unsupported", m, false, nil) }

// Sentinel errors for the engine layer.
var (
	ErrNotApproved       = errors.New("publishing: content has not passed the review & approval workflow")
	ErrNotFound          = errors.New("publishing: publication not found")
	ErrNoPublisher       = errors.New("publishing: no publisher registered for platform")
	ErrPublisherDisabled = errors.New("publishing: publisher is disabled")
	ErrInvalidTransition = errors.New("publishing: invalid status transition")
)

// isRecoverable reports whether an error should be retried.
func isRecoverable(err error) bool {
	var pe *PubError
	if errors.As(err, &pe) {
		return pe.Recoverable
	}
	// Unclassified errors are treated as recoverable transient failures.
	return true
}

// errCode returns the classification code for an error (or "unknown").
func errCode(err error) string {
	var pe *PubError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return "unknown"
}
