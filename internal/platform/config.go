package platform

import "os"

// ConfigSource is the configuration port. Business services and factories read
// configuration through it, so no provider-specific configuration logic (env var
// names, file paths, Parameter Store keys, Secrets Manager ARNs) leaks into
// business code. New configuration backends (AWS Parameter Store, Secrets
// Manager, Consul, …) are added by implementing this interface — nothing else
// changes.
type ConfigSource interface {
	// Get returns the value for a key and whether it was present.
	Get(key string) (string, bool)
}

// Lookup returns a key's value or "" if absent (convenience over Get).
func Lookup(c ConfigSource, key string) string {
	if c == nil {
		return ""
	}
	v, _ := c.Get(key)
	return v
}

// GetDefault returns a key's value or a fallback when absent/empty.
func GetDefault(c ConfigSource, key, fallback string) string {
	if c == nil {
		return fallback
	}
	if v, ok := c.Get(key); ok && v != "" {
		return v
	}
	return fallback
}

// EnvConfig reads configuration from the process environment. Secrets are never
// logged — only read on demand, supporting transparent rotation.
type EnvConfig struct{}

func (EnvConfig) Get(key string) (string, bool) { return os.LookupEnv(key) }

// MapConfig reads from an in-memory map (tests, defaults, or a parsed file).
type MapConfig map[string]string

func (m MapConfig) Get(key string) (string, bool) {
	v, ok := m[key]
	return v, ok
}

// ChainConfig resolves a key against an ordered list of sources, first hit wins.
// This is the fallback pattern for configuration: e.g. explicit map → env →
// Parameter Store → Secrets Manager. Adding a backend is appending a source.
type ChainConfig struct {
	Sources []ConfigSource
}

// NewChainConfig builds a chain (earlier sources take precedence).
func NewChainConfig(sources ...ConfigSource) ChainConfig {
	return ChainConfig{Sources: sources}
}

func (c ChainConfig) Get(key string) (string, bool) {
	for _, s := range c.Sources {
		if s == nil {
			continue
		}
		if v, ok := s.Get(key); ok {
			return v, true
		}
	}
	return "", false
}

// PrefixConfig namespaces keys with a prefix before delegating (e.g. per-provider
// configuration under "BEDROCK_"). It keeps provider config isolated.
type PrefixConfig struct {
	Prefix string
	Source ConfigSource
}

func (p PrefixConfig) Get(key string) (string, bool) {
	if p.Source == nil {
		return "", false
	}
	return p.Source.Get(p.Prefix + key)
}
