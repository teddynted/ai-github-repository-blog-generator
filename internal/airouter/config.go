package airouter

import "strings"

// AllKinds is the set of content kinds the suite can produce, in generation
// order. It matches the orchestrator's stage names and is used for start-up
// logging and provenance.
var AllKinds = []string{
	"blog", "storyboard", "voiceover", "architecture", "youtube",
	"youtube-shorts", "tiktok", "visual-assets", "seo-metadata",
	"linkedin", "x-thread", "architecture-diagram-spec",
}

// aliases maps convenient short names to canonical stage names, so a caller may
// say "seo" or "shorts" and still normalise correctly.
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
