package publishing

// Platform limits (characters / counts). Publishing normalizes to these so a
// platform never rejects content for an over-limit field.
const (
	devtoTitleMax    = 128
	devtoTagMax      = 4
	mediumTitleMax   = 100
	mediumTagMax     = 5
	hashnodeTitleMax = 250
	hashnodeTagMax   = 5
	ytTitleMax       = 100
	ytDescMax        = 5000
	ytTagsCharBudget = 460 // YouTube caps total tag chars ~500; leave headroom
)

// MetadataEngine produces platform-optimized metadata from neutral content.
type MetadataEngine struct {
	Config Config
}

// For builds the metadata for one platform, applying that platform's limits and
// defaults.
func (e MetadataEngine) For(p Platform, c Content) PlatformMetadata {
	vis := firstNonEmpty(c.Metadata["visibility"], e.Config.DefaultVisibility, "public")
	m := PlatformMetadata{
		Platform:     p,
		Title:        c.Title,
		Description:  firstNonEmpty(c.Description, c.Metadata["metaDescription"]),
		Body:         c.Body,
		Tags:         dedupe(c.Tags),
		CanonicalURL: c.CanonicalURL,
		CoverImage:   c.CoverImage,
		Visibility:   vis,
		Extra:        map[string]string{},
	}

	switch p {
	case PlatformDevTo:
		m.Title = truncateChars(m.Title, devtoTitleMax)
		m.Tags = normalizeTags(m.Tags, devtoTagMax, true) // dev.to tags: lowercase, alnum
	case PlatformMedium:
		m.Title = truncateChars(m.Title, mediumTitleMax)
		m.Tags = topStrings(m.Tags, mediumTagMax)
		m.Extra["publishStatus"] = mediumVisibility(vis)
	case PlatformHashnode:
		m.Title = truncateChars(m.Title, hashnodeTitleMax)
		m.Tags = topStrings(m.Tags, hashnodeTagMax)
		m.Extra["slug"] = slugify(c.Title)
		if pubID := c.Metadata["hashnodePublicationId"]; pubID != "" {
			m.Extra["publicationId"] = pubID
		}
	case PlatformYouTube:
		m.Title = truncateChars(m.Title, ytTitleMax)
		m.Description = truncateChars(withChapters(m.Description, c.Metadata["chapters"]), ytDescMax)
		m.Tags = fitTagBudget(m.Tags, ytTagsCharBudget)
		m.Visibility = ytVisibility(vis)
		m.Extra["category"] = firstNonEmpty(c.Metadata["category"], "Science & Technology")
		if c.Metadata["publishAt"] != "" {
			m.Extra["publishAt"] = c.Metadata["publishAt"]
		}
	case PlatformGitHub:
		m.Extra["path"] = firstNonEmpty(c.Metadata["path"], "docs/"+slugify(c.Title)+".md")
		m.Extra["commitMessage"] = firstNonEmpty(c.Metadata["commitMessage"], "docs: publish "+c.Title)
		m.Extra["branch"] = firstNonEmpty(c.Metadata["branch"], "main")
	}
	return m
}

func normalizeTags(tags []string, max int, alnum bool) []string {
	var out []string
	for _, t := range tags {
		if alnum {
			var b []rune
			for _, r := range t {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
					b = append(b, r)
				}
			}
			t = string(b)
		}
		if t != "" {
			out = append(out, t)
		}
	}
	return topStrings(dedupe(out), max)
}

// fitTagBudget keeps tags until the combined character budget is exhausted.
func fitTagBudget(tags []string, budget int) []string {
	var out []string
	used := 0
	for _, t := range tags {
		if used+len(t)+1 > budget {
			break
		}
		out = append(out, t)
		used += len(t) + 1
	}
	return out
}

func mediumVisibility(v string) string {
	switch v {
	case "draft":
		return "draft"
	case "unlisted", "private":
		return "unlisted"
	default:
		return "public"
	}
}

func ytVisibility(v string) string {
	switch v {
	case "unlisted":
		return "unlisted"
	case "private", "draft":
		return "private"
	default:
		return "public"
	}
}

func withChapters(desc, chapters string) string {
	if chapters == "" {
		return desc
	}
	return desc + "\n\nChapters:\n" + chapters
}
