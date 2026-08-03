package main

import (
	"path"
	"strings"
)

// artifactURI builds the s3:// URI of a generated artifact's JSON sidecar. The
// layout mirrors internal/artifactstore exactly so the URIs match what the
// worker wrote: <prefix>/<owner>/<name>/releases/<tag>/.artifacts/<stage>.json.
func artifactURI(bucket, prefix, owner, name, tag, stage string) string {
	key := path.Join(strings.Trim(prefix, "/"), safe(owner), safe(name), "releases", safe(tag), ".artifacts", stage+".json")
	return "s3://" + bucket + "/" + key
}

// outputURI is where the rendered MP4 for a format lands in the video bucket:
// <owner>/<name>/<tag>/<stage>/final-<stage>.mp4.
func outputURI(videoBucket, owner, name, tag, stage string) string {
	key := path.Join(safe(owner), safe(name), safe(tag), stage, "final-"+stage+".mp4")
	return "s3://" + videoBucket + "/" + key
}

// safe sanitizes a path segment identically to internal/artifactstore.safe.
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
