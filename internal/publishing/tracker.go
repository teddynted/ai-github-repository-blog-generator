package publishing

import "time"

// Tracker manages a publication's status lifecycle, appends the audit trail, and
// persists every change. It enforces the status state machine.
type Tracker struct {
	Repo Repository
	Now  Clock
}

func (t Tracker) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

// transition applies a status change if legal, records an audit entry, and
// persists. It returns ErrInvalidTransition for an illegal move.
func (t Tracker) transition(p *Publication, to PublicationStatus, action, detail string) error {
	from := p.Status
	if from != to && !canPubTransition(from, to) {
		return ErrInvalidTransition
	}
	p.Status = to
	p.UpdatedAt = t.now()
	p.Audit = append(p.Audit, AuditEntry{Timestamp: t.now(), Action: action, From: from, To: to, Detail: detail})
	return t.Repo.Save(p)
}
