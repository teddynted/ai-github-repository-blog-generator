package visualassets

// styleFor dispatches a candidate to its category planner.
func styleFor(c candidate, b Branding) Style {
	switch c.Category {
	case catThumbnail:
		return thumbnailStyle(c, b)
	case catSocial, catCover:
		return socialStyle(c, b)
	case catBanner:
		return bannerStyle(c, b)
	case catIllustration, catBlog:
		return illustrationStyle(c, b)
	default:
		return socialStyle(c, b)
	}
}

// palette merges the brand primary + accent colors for a Style.
func palette(b Branding) []string {
	return append(append([]string{}, b.PrimaryColors...), b.AccentColors...)
}

// textPlaceholders returns the reserved text areas for an asset type, so the
// image renders NO literal text and a compositor adds real copy later.
func textPlaceholders(c candidate) []TextPlaceholder {
	switch c.Category {
	case catThumbnail:
		return []TextPlaceholder{
			{Area: "left third", Purpose: "headline"},
			{Area: "lower-left", Purpose: "release tag"},
		}
	case catCover:
		return []TextPlaceholder{
			{Area: "upper third", Purpose: "hook headline"},
			{Area: "lower third", Purpose: "handle / CTA"},
		}
	case catSocial:
		return []TextPlaceholder{
			{Area: "left half", Purpose: "headline"},
			{Area: "lower-left", Purpose: "repository name"},
		}
	case catBanner:
		if c.AspectRatio == "1:1" {
			return []TextPlaceholder{
				{Area: "lower third", Purpose: "release headline"},
				{Area: "top-left", Purpose: "logo"},
			}
		}
		return []TextPlaceholder{
			{Area: "left third", Purpose: "announcement headline"},
			{Area: "lower-left", Purpose: "release tag"},
		}
	case catBlog:
		return []TextPlaceholder{
			{Area: "center or lower-third", Purpose: "article title"},
		}
	case catIllustration:
		return []TextPlaceholder{
			{Area: "component blocks", Purpose: "node labels (added by compositor)"},
		}
	default:
		return []TextPlaceholder{{Area: "reserved zone", Purpose: "headline"}}
	}
}
