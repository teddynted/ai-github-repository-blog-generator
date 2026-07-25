package airouter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Provider names used by the default policy and the worker wiring.
const (
	ProviderClaude = "claude"
	ProviderOllama = "ollama"
)

// AllKinds is the set of content kinds the suite can produce, in generation
// order. It matches the orchestrator's stage names and is the vocabulary the
// routing rules key off.
var AllKinds = []string{
	"blog", "storyboard", "voiceover", "architecture", "youtube",
	"youtube-shorts", "tiktok", "visual-assets", "seo-metadata",
	"linkedin", "x-thread",
}

// DefaultRules is the recommended hybrid policy: premium model for the
// high-value, public-facing writing; everything else defaults to the cheap
// local model via the router's default provider. Kinds not listed here fall to
// the default provider, so this map stays small and intent-revealing.
func DefaultRules() map[string]string {
	return map[string]string{
		"blog":         ProviderClaude, // flagship long-form article
		"architecture": ProviderClaude, // technical reasoning + trade-offs
		"linkedin":     ProviderClaude, // professional public communication
		"x-thread":     ProviderClaude, // audience engagement
		// storyboard, voiceover, youtube, youtube-shorts, tiktok, visual-assets,
		// seo-metadata → default provider (Ollama): structured / repetitive / cheap.
	}
}

// aliases maps convenient short names to canonical stage names, so a config may
// say "seo" or "shorts" and still route correctly.
var aliases = map[string]string{
	"seo":          "seo-metadata",
	"seometadata":  "seo-metadata",
	"shorts":       "youtube-shorts",
	"yt-shorts":    "youtube-shorts",
	"xthread":      "x-thread",
	"x":            "x-thread",
	"twitter":      "x-thread",
	"visuals":      "visual-assets",
	"visualassets": "visual-assets",
	"arch":         "architecture",
	"yt":           "youtube",
}

// canonical normalises a kind name (case, spacing, aliases) to a stage name.
func canonical(kind string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	k = strings.ReplaceAll(k, "_", "-")
	k = strings.ReplaceAll(k, " ", "-")
	if c, ok := aliases[strings.ReplaceAll(k, "-", "")]; ok {
		return c
	}
	if c, ok := aliases[k]; ok {
		return c
	}
	return k
}

// ParseRules parses a JSON routing config into a kind->provider map. It accepts
// two equivalent shapes:
//
//	{"claude": ["blog","architecture"], "ollama": ["seo-metadata"]}   (provider -> kinds)
//	{"blog": "claude", "seo-metadata": "ollama"}                       (kind -> provider)
//
// An empty string yields the DefaultRules. Later entries win on conflict.
func ParseRules(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultRules(), nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("airouter: parse routing config: %w", err)
	}
	out := make(map[string]string)
	for key, val := range raw {
		// Try provider -> [kinds] first.
		var kinds []string
		if err := json.Unmarshal(val, &kinds); err == nil {
			provider := strings.ToLower(strings.TrimSpace(key))
			for _, k := range kinds {
				out[canonical(k)] = provider
			}
			continue
		}
		// Else kind -> provider.
		var provider string
		if err := json.Unmarshal(val, &provider); err == nil {
			out[canonical(key)] = strings.ToLower(strings.TrimSpace(provider))
			continue
		}
		return nil, fmt.Errorf("airouter: routing entry %q is neither a provider list nor a provider name", key)
	}
	return out, nil
}
