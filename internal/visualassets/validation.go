package visualassets

import (
	"fmt"
	"strings"
)

// Validate checks the collection's structural invariants and returns a list of
// problems (empty when valid). It enforces the Milestone 9 rules: every prompt
// is grounded, names a platform and aspect ratio, carries style guidance, avoids
// unsupported (ungrounded) AWS services, and no two prompts are duplicates.
//
// pkg is the source package, used to confirm grounding and service accuracy.
func (col VisualAssetCollection) Validate(pkg ReleasePackage) []string {
	var problems []string

	if col.SchemaVersion == "" {
		problems = append(problems, "missing schemaVersion")
	}
	if len(col.Assets) == 0 {
		problems = append(problems, "collection has no assets")
	}
	if col.Metadata.AssetCount != len(col.Assets) {
		problems = append(problems, fmt.Sprintf("metadata.assetCount %d != assets %d", col.Metadata.AssetCount, len(col.Assets)))
	}

	grounded := groundedRefs(pkg)
	awsUsed := contextAWSSet(pkg)
	seenType := map[string]bool{}
	seenPrompt := map[string]bool{}

	for i, a := range col.Assets {
		where := fmt.Sprintf("asset %d (%s)", i+1, a.Type)

		if collapse(a.Prompt) == "" {
			problems = append(problems, where+": empty prompt")
		}
		// Every prompt must name a platform and an aspect ratio.
		if collapse(a.Platform) == "" {
			problems = append(problems, where+": missing platform")
		}
		if collapse(a.AspectRatio) == "" {
			problems = append(problems, where+": missing aspect ratio")
		}
		// Every prompt must carry style guidance.
		if collapse(a.Style.Style) == "" && collapse(a.Style.Composition) == "" {
			problems = append(problems, where+": missing style guidance")
		}

		// Every prompt must be grounded in the Release Context.
		var anyGrounded bool
		for _, r := range a.References {
			if grounded[collapse(r)] {
				anyGrounded = true
				break
			}
		}
		if !anyGrounded {
			problems = append(problems, where+": no reference is grounded in the release artifacts")
		}

		// The prompt must not name an AWS service the release does not use.
		for _, svc := range ungroundedAWSMentions(a.Prompt, awsUsed) {
			problems = append(problems, fmt.Sprintf("%s: prompt names unsupported AWS service %q (not in the Release Context)", where, svc))
		}

		// Duplicate detection by (type+platform) and by normalized prompt text.
		key := slug(a.Type + "|" + a.Platform)
		if seenType[key] {
			problems = append(problems, fmt.Sprintf("%s: duplicate asset (type+platform)", where))
		}
		seenType[key] = true

		pkey := slug(a.Prompt)
		if pkey != "" && seenPrompt[pkey] {
			problems = append(problems, where+": duplicate prompt text")
		}
		seenPrompt[pkey] = true
	}

	return problems
}

// ungroundedAWSMentions returns AWS services named in the prompt that are NOT in
// the release's AWS set (case-insensitive, whole-word).
func ungroundedAWSMentions(prompt string, used map[string]bool) []string {
	lc := " " + strings.ToLower(prompt) + " "
	var out []string
	seen := map[string]bool{}
	for _, svc := range knownAWSServices {
		l := strings.ToLower(svc)
		if groundedService(l, used) {
			continue
		}
		// Whole-word-ish match bounded by non-alphanumerics.
		if idx := strings.Index(lc, l); idx >= 0 {
			before := lc[idx-1]
			after := lc[idx+len(l)]
			if !isAlnum(before) && !isAlnum(after) && !seen[l] {
				seen[l] = true
				out = append(out, svc)
			}
		}
	}
	return out
}

// groundedService reports whether a known service (lowercased) is covered by the
// release's AWS set — matching sub/superstrings so "Lambda" is grounded by a
// context entry of "AWS Lambda" and vice versa.
func groundedService(l string, used map[string]bool) bool {
	if used[l] {
		return true
	}
	for u := range used {
		if strings.Contains(u, l) || strings.Contains(l, u) {
			return true
		}
	}
	return false
}

func isAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}
