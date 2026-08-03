package main

import "strings"

// diagramURIFromStoryboard derives the architecture-diagram.svg URI that the
// publisher writes alongside the artifacts for a release, from the storyboard
// artifact URI:
//
//	s3://b/<prefix>/<o>/<n>/releases/<tag>/.artifacts/storyboard.json
//	                                     ↳ architecture-diagram.svg
//
// Returns "" if the storyboard URI is not in the expected shape (the caller
// treats a missing diagram as "no background").
func diagramURIFromStoryboard(storyboardURI string) string {
	const suffix = ".artifacts/storyboard.json"
	base, ok := strings.CutSuffix(storyboardURI, suffix)
	if !ok {
		return ""
	}
	return base + "architecture-diagram.svg"
}
