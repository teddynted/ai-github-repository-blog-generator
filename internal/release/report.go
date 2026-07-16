package release

// Severity classifies a validation result.
type Severity int

const (
	// SeverityOK: the check passed.
	SeverityOK Severity = iota
	// SeverityWarning: non-blocking; noted but does not fail a dry-run.
	SeverityWarning
	// SeverityError: release-blocking; fails both dry-run and a real release.
	SeverityError
)

func (s Severity) Symbol() string {
	switch s {
	case SeverityOK:
		return "✓"
	case SeverityWarning:
		return "⚠"
	default:
		return "✗"
	}
}

// Result is one validation outcome. Detail holds optional extra lines (e.g. the
// current vs expected branch) rendered under the summary.
type Result struct {
	Name     string
	Severity Severity
	Message  string
	Detail   []string
}

// Report is the ordered set of validation results for a release plan.
type Report struct {
	Results []Result
}

func (r *Report) add(name string, sev Severity, msg string, detail ...string) {
	r.Results = append(r.Results, Result{Name: name, Severity: sev, Message: msg, Detail: detail})
}

// HasError reports whether any result is release-blocking.
func (r Report) HasError() bool {
	for _, res := range r.Results {
		if res.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Counts returns the number of OK, warning, and error results.
func (r Report) Counts() (ok, warn, err int) {
	for _, res := range r.Results {
		switch res.Severity {
		case SeverityOK:
			ok++
		case SeverityWarning:
			warn++
		case SeverityError:
			err++
		}
	}
	return
}
