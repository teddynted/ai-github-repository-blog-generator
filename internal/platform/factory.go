package platform

import "context"

// Typed asserts a Provider to a concrete domain interface T, threading any prior
// error. It is the bridge between the generic registry and the rich domain
// interfaces, so callers select generically and use specifically:
//
//	llm, err := platform.Typed[LLMProvider](reg.Select(ctx, KindLLM, cfg, WithCapability("text-generation")))
func Typed[T any](p Provider, err error) (T, error) {
	var zero T
	if err != nil {
		return zero, err
	}
	t, ok := any(p).(T)
	if !ok {
		return zero, ErrUnsupported
	}
	return t, nil
}

// SelectTyped selects a provider for a kind and asserts it to T in one call.
func SelectTyped[T any](ctx context.Context, r *Registry, kind Kind, cfg ConfigSource, opts ...SelectOption) (T, error) {
	return Typed[T](r.Select(ctx, kind, cfg, opts...))
}

// staticFactory adapts an already-constructed provider into a Factory (handy for
// reference providers and tests that need no configuration).
func staticFactory(p Provider) Factory {
	return func(ConfigSource) (Provider, error) { return p, nil }
}
