package seo

// planYouTube assembles the YouTube SEO by reusing the YouTube script's own
// content intelligence (titles, description, chapters, pinned comment, playlist)
// and normalizing it: title bounded to the platform limit, tags/keywords/
// hashtags deduped. Nothing is regenerated.
func planYouTube(pkg ReleasePackage, k Keywords) YouTubeSEO {
	ci := pkg.YouTube.ContentIntelligence

	title := firstNonEmpty(ci.SuggestedTitle, pkg.YouTube.Video.Title, repoShortName(pkg)+" "+releaseTag(pkg)+" — Full Walkthrough")
	if len(title) > YouTubeTitleMax {
		title = truncateChars(title, YouTubeTitleMax)
	}

	desc := firstNonEmpty(ci.SuggestedDescription, summary(pkg))

	var chapters []ChapterTitle
	for _, m := range ci.Chapters {
		chapters = append(chapters, ChapterTitle{Timestamp: m.Timestamp, Title: m.Title})
	}

	playlists := []string{}
	if ci.SuggestedPlaylist != "" {
		playlists = append(playlists, ci.SuggestedPlaylist)
	}
	playlists = append(playlists, "Release Deep Dives", "AWS & Cloud Engineering")

	var thumb []string
	if ci.SuggestedThumbnail != "" {
		thumb = append(thumb, ci.SuggestedThumbnail)
	}
	thumb = append(thumb, "THE ARCHITECTURE", releaseTagUpper(pkg))

	return YouTubeSEO{
		Title:             title,
		AlternativeTitles: topStrings(dedupe(ci.AlternativeTitles), 5),
		Description:       desc,
		Keywords:          topStrings(allKeywords(k), 15),
		Tags:              planYouTubeTags(pkg, k),
		Hashtags:          topStrings(reuseHashtags(youtubeHashtags(pkg), prependHash(k.AWS), ytHashtagMax), ytHashtagMax),
		ChapterTitles:     chapters,
		PinnedComment:     firstNonEmpty(ci.PinnedComment, defaultPinned(pkg)),
		Playlists:         topStrings(dedupe(playlists), 4),
		ThumbnailText:     topStrings(dedupe(thumb), 4),
	}
}

func releaseTagUpper(pkg ReleasePackage) string {
	t := releaseTag(pkg)
	up := ""
	for _, r := range t {
		if r >= 'a' && r <= 'z' {
			up += string(r - 'a' + 'A')
		} else {
			up += string(r)
		}
	}
	if up == "" {
		return "NEW RELEASE"
	}
	return up
}

func defaultPinned(pkg ReleasePackage) string {
	return "📌 Everything in this video is generated from " + repoShortName(pkg) + " " + releaseTag(pkg) +
		"'s own Release Context. Repo + docs in the description. What should the next deep dive cover?"
}
