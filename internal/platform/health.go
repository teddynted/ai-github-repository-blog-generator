package platform

// HealthStatus is a provider's coarse availability.
type HealthStatus string

const (
	HealthOK       HealthStatus = "ok"
	HealthDegraded HealthStatus = "degraded"
	HealthUnknown  HealthStatus = "unknown"
	HealthDown     HealthStatus = "down"
)

// Health is a provider's reported health.
type Health struct {
	Status HealthStatus `json:"status"`
	Detail string       `json:"detail,omitempty"`
}

// Healthy reports whether the provider is usable (ok or degraded).
func (h Health) Healthy() bool {
	return h.Status == HealthOK || h.Status == HealthDegraded
}

// OK is a convenience constructor for a healthy result.
func OK() Health { return Health{Status: HealthOK} }

// Down is a convenience constructor for an unavailable result.
func Down(detail string) Health { return Health{Status: HealthDown, Detail: detail} }
