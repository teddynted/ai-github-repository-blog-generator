package socialintel

import "os"

// Credentials reads platform secrets from the environment. Secrets are never
// logged, serialized, or embedded in errors — only their presence is reported.
// Rotation is supported transparently: each call re-reads the environment.
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
	envYouTubeToken   = "YOUTUBE_ACCESS_TOKEN"
	envInstagramToken = "INSTAGRAM_ACCESS_TOKEN"
	envInstagramUser  = "INSTAGRAM_USER_ID"
	envXBearer        = "X_BEARER_TOKEN"
	envXUserID        = "X_USER_ID"
	envTikTokToken    = "TIKTOK_ACCESS_TOKEN"
)
