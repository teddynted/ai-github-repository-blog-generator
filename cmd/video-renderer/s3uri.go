package main

import (
	"fmt"
	"strings"
)

// parseS3URI splits an s3://bucket/key URI into its bucket and key.
func parseS3URI(uri string) (bucket, key string, err error) {
	rest, ok := strings.CutPrefix(uri, "s3://")
	if !ok {
		return "", "", fmt.Errorf("not an s3 uri: %q", uri)
	}
	bucket, key, ok = strings.Cut(rest, "/")
	if !ok || bucket == "" || key == "" {
		return "", "", fmt.Errorf("s3 uri missing bucket or key: %q", uri)
	}
	return bucket, key, nil
}
