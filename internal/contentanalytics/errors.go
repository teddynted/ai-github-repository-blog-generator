package contentanalytics

import "errors"

// Sentinel errors.
var (
	// ErrDuplicateSnapshot is returned when a snapshot already exists for a
	// (publication, period, date) — snapshots are immutable and never overwritten.
	ErrDuplicateSnapshot = errors.New("contentanalytics: snapshot already exists (immutable)")
	// ErrNoData indicates there is nothing to analyze yet.
	ErrNoData = errors.New("contentanalytics: no data")
	// ErrUnsupported indicates a provider does not report a capability.
	ErrUnsupported = errors.New("contentanalytics: capability not supported by provider")
	// ErrNotFound indicates a publication/snapshot lookup missed.
	ErrNotFound = errors.New("contentanalytics: not found")
)

// CollectError is a classified provider/collection error. Recoverable errors are
// retried with exponential backoff; permanent errors fail fast.
type CollectError struct {
	Code        string // network | timeout | rate_limit | server | auth | quota | not_found | partial
	Message     string
	Recoverable bool
	Err         error
}

func (e *CollectError) Error() string {
	if e.Err != nil {
		return "contentanalytics: " + e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return "contentanalytics: " + e.Code + ": " + e.Message
}

func (e *CollectError) Unwrap() error { return e.Err }

// Recoverable constructors.
func errNetwork(msg string, err error) *CollectError {
	return &CollectError{Code: "network", Message: msg, Recoverable: true, Err: err}
}
func errTimeout(msg string) *CollectError {
	return &CollectError{Code: "timeout", Message: msg, Recoverable: true}
}
func errRateLimit(msg string) *CollectError {
	return &CollectError{Code: "rate_limit", Message: msg, Recoverable: true}
}
func errServer(msg string) *CollectError {
	return &CollectError{Code: "server", Message: msg, Recoverable: true}
}
func errPartial(msg string) *CollectError {
	return &CollectError{Code: "partial", Message: msg, Recoverable: true}
}

// Permanent constructors.
func errAuth(msg string) *CollectError {
	return &CollectError{Code: "auth", Message: msg, Recoverable: false}
}
func errQuota(msg string) *CollectError {
	return &CollectError{Code: "quota", Message: msg, Recoverable: false}
}
func errMissing(msg string) *CollectError {
	return &CollectError{Code: "not_found", Message: msg, Recoverable: false}
}

// isRecoverable reports whether an error should be retried.
func isRecoverable(err error) bool {
	var ce *CollectError
	if errors.As(err, &ce) {
		return ce.Recoverable
	}
	return false
}

// isAuthError reports whether an error is an authentication failure (for the
// CloudWatch auth-failure alarm signal).
func isAuthError(err error) bool {
	var ce *CollectError
	if errors.As(err, &ce) {
		return ce.Code == "auth"
	}
	return false
}

// errCode extracts the classification code (or "" for unclassified).
func errCode(err error) string {
	var ce *CollectError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}
