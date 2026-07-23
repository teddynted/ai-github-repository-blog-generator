package mcp

import (
	"context"
	"os"
)

// AuthKind is how a server authenticates a caller.
type AuthKind string

const (
	AuthNone   AuthKind = "none"    // no credential needed (local/in-process servers)
	AuthBearer AuthKind = "bearer"  // OAuth/JWT bearer token
	AuthAPIKey AuthKind = "api-key" // static API key
	AuthIAM    AuthKind = "iam"     // AWS SigV4 / instance role (resolved by the AWS adapter)
)

// NoAuth is an AuthProvider for servers that need no credential (the in-process
// default). It never returns a secret.
type NoAuth struct{}

func (NoAuth) Credential(context.Context, string) (Credential, error) {
	return Credential{Kind: AuthNone}, nil
}

// EnvAuth resolves a per-server credential from an environment variable, keyed by
// a prefix + upper-cased server id (e.g. MCP_GITHUB_TOKEN). Secrets are read on
// demand and never logged, supporting rotation.
type EnvAuth struct {
	Prefix string   // default "MCP_"
	Kind   AuthKind // credential kind these env values represent (default bearer)
	lookup func(string) string
}

// NewEnvAuth builds an environment-backed auth provider.
func NewEnvAuth(prefix string, kind AuthKind) *EnvAuth {
	if prefix == "" {
		prefix = "MCP_"
	}
	if kind == "" {
		kind = AuthBearer
	}
	return &EnvAuth{Prefix: prefix, Kind: kind, lookup: os.Getenv}
}

func (e *EnvAuth) envName(serverID string) string {
	up := make([]byte, 0, len(serverID))
	for i := 0; i < len(serverID); i++ {
		c := serverID[i]
		switch {
		case c >= 'a' && c <= 'z':
			up = append(up, c-32)
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			up = append(up, c)
		default:
			up = append(up, '_')
		}
	}
	return e.Prefix + string(up) + "_TOKEN"
}

func (e *EnvAuth) Credential(_ context.Context, serverID string) (Credential, error) {
	lookup := e.lookup
	if lookup == nil {
		lookup = os.Getenv
	}
	return Credential{Kind: e.Kind, value: lookup(e.envName(serverID))}, nil
}

// MapAuth resolves credentials from an in-memory map (tests only — never real
// secrets in source).
type MapAuth struct {
	Kind    AuthKind
	Secrets map[string]string // serverID -> secret
}

func (m MapAuth) Credential(_ context.Context, serverID string) (Credential, error) {
	kind := m.Kind
	if kind == "" {
		kind = AuthBearer
	}
	return Credential{Kind: kind, value: m.Secrets[serverID]}, nil
}

// SecretResolver is the port a production AuthProvider uses to fetch a secret
// value by id (satisfied by an AWS Secrets Manager adapter). Keeping it a port
// means this package never imports the AWS SDK and stays offline-testable.
type SecretResolver interface {
	Resolve(ctx context.Context, secretID string) (string, error)
}

// SecretsManagerAuth resolves a per-server credential from a secret store via the
// SecretResolver port (e.g. AWS Secrets Manager). Secret ids are namespaced per
// server; values are fetched on demand and never logged.
type SecretsManagerAuth struct {
	Resolver SecretResolver
	Prefix   string   // secret-id prefix, e.g. "mcp/"
	Kind     AuthKind // default bearer
}

func (s SecretsManagerAuth) Credential(ctx context.Context, serverID string) (Credential, error) {
	if s.Resolver == nil {
		return Credential{Kind: AuthNone}, nil
	}
	prefix := s.Prefix
	if prefix == "" {
		prefix = "mcp/"
	}
	kind := s.Kind
	if kind == "" {
		kind = AuthBearer
	}
	v, err := s.Resolver.Resolve(ctx, prefix+serverID)
	if err != nil {
		return Credential{}, err
	}
	return Credential{Kind: kind, value: v}, nil
}
