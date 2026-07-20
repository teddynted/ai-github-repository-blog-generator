package platform

// selectOptions are the resolved selection filters.
type selectOptions struct {
	id          string
	capability  Capability
	managedOnly bool
	localOnly   bool
	minPriority int
}

// SelectOption customizes provider selection. Options compose, so new selection
// criteria are added without changing Select's signature.
type SelectOption func(*selectOptions)

// WithID forces a specific provider id (still health-checked).
func WithID(id string) SelectOption {
	return func(o *selectOptions) { o.id = id }
}

// WithCapability requires the provider to advertise a capability.
func WithCapability(c Capability) SelectOption {
	return func(o *selectOptions) { o.capability = c }
}

// ManagedOnly restricts selection to managed/cloud providers.
func ManagedOnly() SelectOption {
	return func(o *selectOptions) { o.managedOnly = true }
}

// LocalOnly restricts selection to self-hosted/local providers.
func LocalOnly() SelectOption {
	return func(o *selectOptions) { o.localOnly = true }
}

// WithMinPriority filters out providers below a priority floor.
func WithMinPriority(n int) SelectOption {
	return func(o *selectOptions) { o.minPriority = n }
}

func applyOptions(opts []SelectOption) selectOptions {
	var o selectOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
