package publishing

// PublicationStatus is a state in the publication lifecycle.
type PublicationStatus string

const (
	StatusPending    PublicationStatus = "Pending"
	StatusScheduled  PublicationStatus = "Scheduled"
	StatusPublishing PublicationStatus = "Publishing"
	StatusPublished  PublicationStatus = "Published"
	StatusFailed     PublicationStatus = "Failed"
	StatusRetrying   PublicationStatus = "Retrying"
	StatusCancelled  PublicationStatus = "Cancelled"
	StatusArchived   PublicationStatus = "Archived"
)

// pubTransitions is the legal publication state machine.
var pubTransitions = map[PublicationStatus][]PublicationStatus{
	StatusPending:    {StatusScheduled, StatusPublishing, StatusCancelled, StatusFailed},
	StatusScheduled:  {StatusPublishing, StatusCancelled, StatusFailed},
	StatusPublishing: {StatusPublished, StatusRetrying, StatusFailed},
	StatusRetrying:   {StatusPublishing, StatusFailed},
	StatusPublished:  {StatusArchived},
	StatusFailed:     {StatusRetrying, StatusCancelled, StatusArchived},
	StatusCancelled:  {StatusArchived},
	StatusArchived:   {},
}

// canPubTransition reports whether from → to is legal.
func canPubTransition(from, to PublicationStatus) bool {
	for _, s := range pubTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}
