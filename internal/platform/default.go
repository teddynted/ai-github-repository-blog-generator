package platform

// defaultRegistry is the process-wide registry that plug-ins auto-register into
// via package init() — the database/sql-style driver pattern. Business code reads
// providers from Default(), so provider selection is never hard-coded.
var defaultRegistry = NewRegistry()

// Default returns the process-wide registry.
func Default() *Registry { return defaultRegistry }

// Register adds a provider to the default registry.
func Register(reg Registration) error { return defaultRegistry.Register(reg) }

// MustRegister adds a provider to the default registry or panics (init use).
func MustRegister(reg Registration) { defaultRegistry.MustRegister(reg) }
