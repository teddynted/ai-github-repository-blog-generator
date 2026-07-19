package visualassets

// category groups asset types by the planner that styles them.
type category string

const (
	catThumbnail    category = "thumbnail"
	catCover        category = "cover"
	catSocial       category = "social"
	catBlog         category = "blog"
	catBanner       category = "banner"
	catIllustration category = "illustration"
)

// candidate is a discovered visual asset to produce, grounded in the release.
type candidate struct {
	Type        string
	Platform    string
	AspectRatio string
	Dimensions  string
	Category    category
	Purpose     string
	Subject     string   // grounded subject the imagery depicts
	Focus       string   // technical focus
	References  []string // grounded artifact references
}

// discover returns the visual assets a release needs, most-important first. Each
// candidate is grounded (subject + references come from the Release Context and
// downstream artifacts). Covers for Shorts/TikTok are only added when those
// artifacts are present. Capped at max.
func discover(pkg ReleasePackage, max int) []candidate {
	if max <= 0 {
		max = defaultMaxAssets
	}
	repo := repoShortName(pkg)
	tag := releaseTag(pkg)
	subject := subjectSeed(pkg)
	arch := archSubject(pkg)
	svcRefs := awsServices(pkg, 4)
	baseRefs := groundedRefList(pkg)

	var cands []candidate
	add := func(c candidate) {
		if len(c.References) == 0 {
			c.References = baseRefs
		}
		cands = append(cands, c)
	}

	// Core video + social set (always).
	add(candidate{
		Type: "YouTube Thumbnail", Platform: "YouTube", AspectRatio: "16:9", Dimensions: "1280x720",
		Category: catThumbnail, Purpose: "Drive clicks on the long-form release deep dive.",
		Subject: arch, Focus: "release architecture at a glance",
	})
	add(candidate{
		Type: "Repository Hero Image", Platform: "GitHub", AspectRatio: "16:9", Dimensions: "1600x900",
		Category: catBanner, Purpose: "Hero image for the repository README / landing.",
		Subject: subject, Focus: "what the repository does",
	})
	add(candidate{
		Type: "GitHub Social Card", Platform: "GitHub", AspectRatio: "1.91:1", Dimensions: "1280x640",
		Category: catSocial, Purpose: "GitHub social preview when the repo is shared.",
		Subject: subject, Focus: repo + " " + tag,
	})
	add(candidate{
		Type: "LinkedIn Banner", Platform: "LinkedIn", AspectRatio: "1.91:1", Dimensions: "1200x627",
		Category: catSocial, Purpose: "Professional announcement graphic for LinkedIn.",
		Subject: subject, Focus: "release announcement",
	})
	add(candidate{
		Type: "X Image", Platform: "X", AspectRatio: "16:9", Dimensions: "1600x900",
		Category: catSocial, Purpose: "Shareable image for an X (Twitter) post.",
		Subject: subject, Focus: "release highlight",
	})

	// Blog headers (when a blog exists — it always does in the pipeline).
	add(candidate{
		Type: "Blog Header", Platform: "Blog", AspectRatio: "16:9", Dimensions: "1600x900",
		Category: catBlog, Purpose: "Header image for the technical blog post.",
		Subject: subject, Focus: "the technical story of the release",
	})
	add(candidate{
		Type: "Dev.to Cover", Platform: "Dev.to", AspectRatio: "1000:420", Dimensions: "1000x420",
		Category: catBlog, Purpose: "Cover image for the Dev.to cross-post.",
		Subject: subject, Focus: "the technical story of the release",
	})
	add(candidate{
		Type: "Medium Cover", Platform: "Medium", AspectRatio: "3:2", Dimensions: "1500x1000",
		Category: catBlog, Purpose: "Cover image for the Medium cross-post.",
		Subject: subject, Focus: "the technical story of the release",
	})

	// Architecture-driven illustrations (only when there is architecture to show).
	if arch != "" {
		add(candidate{
			Type: "Architecture Illustration", Platform: "Docs", AspectRatio: "16:9", Dimensions: "1920x1080",
			Category: catIllustration, Purpose: "Explain the system architecture in documentation.",
			Subject: arch, Focus: "component relationships", References: archRefs(pkg),
		})
	}
	if len(svcRefs) > 0 {
		add(candidate{
			Type: "AWS Workflow Diagram", Platform: "Docs", AspectRatio: "16:9", Dimensions: "1920x1080",
			Category: catIllustration, Purpose: "Illustrate the AWS event flow.",
			Subject: "an event-driven AWS workflow using " + joinAnd(svcRefs),
			Focus:   "AWS service flow", References: append([]string{}, svcRefs...),
		})
	}

	// Promotional set (always).
	add(candidate{
		Type: "Release Card", Platform: "Generic", AspectRatio: "1:1", Dimensions: "1080x1080",
		Category: catBanner, Purpose: "Square release-announcement card for feeds.",
		Subject: subject, Focus: repo + " " + tag + " release",
	})
	add(candidate{
		Type: "Promotional Graphic", Platform: "Generic", AspectRatio: "1:1", Dimensions: "1080x1080",
		Category: catBanner, Purpose: "Promote the feature / open-source project.",
		Subject: featureName(pkg), Focus: "the headline feature",
	})

	// Short-form covers (only when those artifacts exist).
	if len(pkg.Shorts.Shorts) > 0 {
		add(candidate{
			Type: "YouTube Shorts Cover", Platform: "YouTube Shorts", AspectRatio: "9:16", Dimensions: "1080x1920",
			Category: catCover, Purpose: "Vertical cover for the YouTube Short.",
			Subject: arch, Focus: shortsFocus(pkg),
		})
	}
	if len(pkg.TikTok.Videos) > 0 {
		add(candidate{
			Type: "TikTok Cover", Platform: "TikTok", AspectRatio: "9:16", Dimensions: "1080x1920",
			Category: catCover, Purpose: "Vertical cover for the TikTok video.",
			Subject: arch, Focus: tiktokFocus(pkg),
		})
	}

	if len(cands) > max {
		cands = cands[:max]
	}
	return cands
}

const defaultMaxAssets = 14

// archSubject returns a grounded architecture subject, or "" when none exists.
func archSubject(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		if s := firstSentences(c.Architecture.Overview, 1); s != "" {
			return s
		}
		if len(c.Architecture.AWSServices) > 0 {
			return "an event-driven architecture using " + joinAnd(awsServices(pkg, 3))
		}
		if diagramSource(pkg) != "" {
			return "the system architecture diagram"
		}
	}
	if diagramSource(pkg) != "" {
		return "the system architecture diagram"
	}
	return ""
}

func archRefs(pkg ReleasePackage) []string {
	var refs []string
	if src := diagramSource(pkg); src != "" {
		refs = append(refs, src)
	}
	refs = append(refs, awsServices(pkg, 4)...)
	if len(refs) == 0 {
		refs = groundedRefList(pkg)
	}
	return refs
}

func shortsFocus(pkg ReleasePackage) string {
	if len(pkg.Shorts.Shorts) > 0 {
		return lowerFirst(pkg.Shorts.Shorts[0].Angle)
	}
	return "a single technical idea"
}

func tiktokFocus(pkg ReleasePackage) string {
	if len(pkg.TikTok.Videos) > 0 {
		return lowerFirst(pkg.TikTok.Videos[0].Topic)
	}
	return "a single technical idea"
}

// groundedRefList is the default reference set (repo + tag + top services).
func groundedRefList(pkg ReleasePackage) []string {
	var refs []string
	if n := repoShortName(pkg); n != "" {
		refs = append(refs, n)
	}
	if t := releaseTag(pkg); t != "" {
		refs = append(refs, t)
	}
	refs = append(refs, awsServices(pkg, 3)...)
	return dedupe(refs)
}
