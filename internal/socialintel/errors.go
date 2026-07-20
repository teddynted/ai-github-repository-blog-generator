package socialintel

import "errors"

// CollectError is a classified collection error. Recoverable errors are retried.
type CollectError struct {
	Code        string // network | timeout | rate-limit | quota | auth | partial | platform-down | missing
	Message     string
	Recoverable bool
	Err         error
}

func (e *CollectError) Error() string {
	if e.Err != nil {
		return "socialintel: " + e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return "socialintel: " + e.Code + ": " + e.Message
}

func (e *CollectError) Unwrap() error { return e.Err }

func newErr(code, msg string, recoverable bool, cause error) *CollectError {
	return &CollectError{Code: code, Message: msg, Recoverable: recoverable, Err: cause}
}

func errNetwork(c error) *CollectError       { return newErr("network", "network failure", true, c) }
func errTimeout(c error) *CollectError       { return newErr("timeout", "request timed out", true, c) }
func errRateLimit(m string) *CollectError    { return newErr("rate-limit", m, true, nil) }
func errPlatformDown(m string) *CollectError { return newErr("platform-down", m, true, nil) }
func errPartial(m string) *CollectError      { return newErr("partial", m, true, nil) }
func errAuth(m string) *CollectError         { return newErr("auth", m, false, nil) }
func errQuota(m string) *CollectError        { return newErr("quota", m, false, nil) }
func errMissing(m string) *CollectError      { return newErr("missing", m, false, nil) }

// Sentinel errors.
var (
	ErrDuplicateSnapshot = errors.New("socialintel: snapshot already exists for this platform/date (immutable)")
	ErrNoData            = errors.New("socialintel: no data available")
	ErrNoProvider        = errors.New("socialintel: no provider registered for platform")
)

func isRecoverable(err error) bool {
	var ce *CollectError
	if errors.As(err, &ce) {
		return ce.Recoverable
	}
	return true
}

func errCode(err error) string {
	var ce *CollectError
	if errors.As(err, &ce) {
		return ce.Code
	}
	if err == nil {
		return ""
	}
	return "unknown"
}
