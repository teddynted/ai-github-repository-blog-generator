package publishing

import "os"

// Credentials reads platform secrets from the environment. Secrets are never
// logged, serialized, or embedded in errors — only their presence is reported.
type Credentials struct {
	lookup func(string) string
}

// EnvCredentials reads from the process environment.
func EnvCredentials() Credentials { return Credentials{lookup: os.Getenv} }

// mapCredentials reads from a map (for tests) — never real secrets.
func mapCredentials(m map[string]string) Credentials {
	return Credentials{lookup: func(k string) string { return m[k] }}
}

func (c Credentials) get(key string) string {
	if c.lookup == nil {
		return ""
	}
	return c.lookup(key)
}

// has reports whether a credential is present (safe to log).
func (c Credentials) has(key string) bool { return c.get(key) != "" }

// Known credential environment variable names.
const (
	envDevToKey      = "DEVTO_API_KEY"
	envMediumToken   = "MEDIUM_TOKEN"
	envMediumUserID  = "MEDIUM_USER_ID"
	envHashnodeToken = "HASHNODE_TOKEN"
	envYouTubeToken  = "YOUTUBE_ACCESS_TOKEN"
	envGitHubToken   = "GITHUB_TOKEN"
)
