package contentanalytics

import "os"

// Credentials reads platform secrets from the environment. Secrets are never
// logged, serialized, or embedded in errors — only their presence is reported.
// Rotation is supported transparently: each call re-reads the environment, so a
// rotated token takes effect on the next collection with no code change.
type Credentials struct {
	lookup func(string) string
}

// EnvCredentials reads from the process environment.
func EnvCredentials() Credentials { return Credentials{lookup: os.Getenv} }

// mapCredentials reads from a map (tests only — never real secrets).
func mapCredentials(m map[string]string) Credentials {
	return Credentials{lookup: func(k string) string { return m[k] }}
}

func (c Credentials) get(key string) string {
	if c.lookup == nil {
		return ""
	}
	return c.lookup(key)
}

func (c Credentials) has(key string) bool { return c.get(key) != "" }

// Known credential environment variable names.
const (
	envDevToKey       = "DEVTO_API_KEY"
	envMediumToken    = "MEDIUM_INTEGRATION_TOKEN"
	envMediumStatsURL = "MEDIUM_STATS_URL" // Medium has no public analytics API; optional stats feed
	envHashnodeToken  = "HASHNODE_TOKEN"
	envYouTubeKey     = "YOUTUBE_API_KEY"      // Data API v3
	envYouTubeOAuth   = "YOUTUBE_ACCESS_TOKEN" // Analytics API (OAuth)
	envYouTubeChannel = "YOUTUBE_CHANNEL_ID"
)
