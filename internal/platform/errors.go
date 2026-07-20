package platform

import (
	"errors"
	"fmt"
)

// Sentinel errors.
var (
	// ErrNotFound is returned when no provider matches a lookup.
	ErrNotFound = errors.New("platform: provider not found")
	// ErrAlreadyRegistered is returned when a (kind, id) is registered twice.
	ErrAlreadyRegistered = errors.New("platform: provider already registered")
	// ErrIncompatible is returned when a provider targets an incompatible API major.
	ErrIncompatible = errors.New("platform: incompatible API version")
	// ErrNoHealthyProvider is returned when every candidate is unhealthy.
	ErrNoHealthyProvider = errors.New("platform: no healthy provider available")
	// ErrInvalidRegistration is returned when a registration is missing required fields.
	ErrInvalidRegistration = errors.New("platform: invalid registration")
	// ErrUnsupported is returned by reference/stub providers for unimplemented ops.
	ErrUnsupported = errors.New("platform: operation not supported by this provider")
)

// SelectionError explains why selection failed, listing what was tried.
type SelectionError struct {
	Kind       Kind
	Capability Capability
	Tried      []string
	Err        error
}

func (e *SelectionError) Error() string {
	if e.Capability != "" {
		return fmt.Sprintf("platform: no %s provider with capability %q (tried %v): %v", e.Kind, e.Capability, e.Tried, e.Err)
	}
	return fmt.Sprintf("platform: no %s provider (tried %v): %v", e.Kind, e.Tried, e.Err)
}

func (e *SelectionError) Unwrap() error { return e.Err }
