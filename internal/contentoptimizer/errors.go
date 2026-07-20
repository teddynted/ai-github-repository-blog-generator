package contentoptimizer

import "errors"

// Sentinel errors.
var (
	// ErrNoData indicates there is not enough analytics to optimize.
	ErrNoData = errors.New("contentoptimizer: insufficient analytics data")
	// ErrDuplicateReport is returned when a report already exists for a RunID.
	ErrDuplicateReport = errors.New("contentoptimizer: report already exists (immutable)")
	// ErrNotFound indicates a lookup missed.
	ErrNotFound = errors.New("contentoptimizer: not found")
	// ErrReasoningUnavailable indicates the AI reasoning provider failed and the
	// engine fell back to deterministic reasoning.
	ErrReasoningUnavailable = errors.New("contentoptimizer: reasoning provider unavailable")
	// ErrInvalidTransition rejects an illegal prompt approval transition.
	ErrInvalidTransition = errors.New("contentoptimizer: invalid approval transition")
)

// OptimizeError is a classified optimization error supporting retry/fallback.
type OptimizeError struct {
	Code        string // reasoning | timeout | rate_limit | parse | partial
	Message     string
	Recoverable bool
	Err         error
}

func (e *OptimizeError) Error() string {
	if e.Err != nil {
		return "contentoptimizer: " + e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return "contentoptimizer: " + e.Code + ": " + e.Message
}

func (e *OptimizeError) Unwrap() error { return e.Err }

func errReasoning(msg string, err error) *OptimizeError {
	return &OptimizeError{Code: "reasoning", Message: msg, Recoverable: true, Err: err}
}
func errParse(msg string) *OptimizeError {
	return &OptimizeError{Code: "parse", Message: msg, Recoverable: false}
}

// isRecoverable reports whether an error should be retried.
func isRecoverable(err error) bool {
	var oe *OptimizeError
	if errors.As(err, &oe) {
		return oe.Recoverable
	}
	return false
}
