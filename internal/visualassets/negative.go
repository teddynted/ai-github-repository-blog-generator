package visualassets

import "strings"

// universalNegatives apply to every asset — they keep AI output clean and
// on-brand.
var universalNegatives = []string{
	"no gibberish text", "no misspelled words", "no watermarks", "no signatures",
	"no logos", "no clutter", "no excessive visual noise", "no distorted shapes",
	"no low-resolution artifacts",
}

// planNegative builds the negative prompt for a candidate: the universal set
// plus category- and grounding-specific exclusions (e.g. avoid unrelated AWS
// services so the imagery stays accurate to the Release Context).
func planNegative(c candidate, pkg ReleasePackage) string {
	negs := append([]string{}, universalNegatives...)

	switch c.Category {
	case catThumbnail, catBanner, catSocial, catCover:
		negs = append(negs, "no photorealistic human faces", "no busy backgrounds that fight the headline")
	case catIllustration:
		negs = append(negs, "no distorted diagrams", "no tangled or crossing connectors", "no unreadable component shapes")
	case catBlog:
		negs = append(negs, "no stock-photo clichés", "no photorealistic people")
	}

	// Keep AWS imagery accurate: exclude services the release does not use.
	if c.Category == catIllustration || strings.Contains(strings.ToLower(c.Focus), "aws") {
		if excl := excludedAWS(pkg); excl != "" {
			negs = append(negs, "no unrelated AWS services ("+excl+")")
		}
	}

	return strings.Join(dedupe(negs), ", ")
}

// excludedAWS names a few common AWS services NOT used by this release, so the
// image model doesn't sprinkle in irrelevant ones.
func excludedAWS(pkg ReleasePackage) string {
	used := contextAWSSet(pkg)
	common := []string{"Amazon RDS", "Amazon EKS", "Amazon Redshift", "Amazon SageMaker", "AWS Glue", "Amazon Aurora"}
	var out []string
	for _, svc := range common {
		if !used[strings.ToLower(svc)] {
			out = append(out, svc)
		}
		if len(out) >= 3 {
			break
		}
	}
	return strings.Join(out, ", ")
}
