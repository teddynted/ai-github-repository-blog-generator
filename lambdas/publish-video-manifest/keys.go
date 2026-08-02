package main

import (
	"path"
	"strings"
)

// manifestKey is where the manifest lands in the video bucket:
// <owner>/<name>/<tag>/videos/manifest.json (segments sanitized like the
// artifact layout in internal/artifactstore).
func manifestKey(owner, name, tag string) string {
	return path.Join(safe(owner), safe(name), safe(tag), "videos", "manifest.json")
}

func safe(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
