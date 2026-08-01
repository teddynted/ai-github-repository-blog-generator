package seo

import "strings"

// planYouTube assembles the YouTube SEO by reusing the YouTube script's own
// content intelligence (titles, description, chapters, pinned comment, playlist)
// and normalizing it: title bounded to the platform limit, tags/keywords/
// hashtags deduped. Nothing is regenerated.
func planYouTube(pkg ReleasePackage, k Keywords) YouTubeSEO {
	ci := pkg.YouTube.ContentIntelligence

	title := firstNonEmpty(ci.SuggestedTitle, pkg.YouTube.Video.Title, repoShortName(pkg)+" "+releaseTag(pkg)+" — Full Walkthrough")
	title = shortenYouTubeTitle(title)

	desc := firstNonEmpty(ci.SuggestedDescription, summary(pkg))
	desc = leadWithTopic(desc, k.Primary, pkg.Blog.Title)

	var chapters []ChapterTitle
	for _, m := range ci.Chapters {
		chapters = append(chapters, ChapterTitle{Timestamp: m.Timestamp, Title: m.Title})
	}

	playlists := []string{}
	if ci.SuggestedPlaylist != "" {
		playlists = append(playlists, ci.SuggestedPlaylist)
	}
	playlists = append(playlists, "Release Deep Dives", "AWS & Cloud Engineering")

	// Thumbnail text is built from the TOPIC — release tag + up to two short topic
	// lines — rather than reusing the (often contaminated) suggested thumbnail or
	// a generic "THE ARCHITECTURE" tile. Max 3 lines.
	thumb := append([]string{releaseTagUpper(pkg)}, thumbnailTopicLines(k)...)
	thumb = topStrings(dedupe(thumb), 3)

	// Hashtags lead with the article's central AWS services and topic, not the
	// full detected service inventory.
	topicTags := prependHash(dedupe(concatStrings(centralAWS(k.AWS, topicSignals(pkg)), k.Primary)))

	return YouTubeSEO{
		Title:             title,
		AlternativeTitles: topStrings(dedupe(ci.AlternativeTitles), 5),
		Description:       desc,
		Keywords:          topStrings(dropJunk(allKeywords(k), pkg), 15),
		Tags:              planYouTubeTags(pkg, k),
		Hashtags:          topStrings(reuseHashtags(youtubeHashtags(pkg), topicTags, ytHashtagMax), ytHashtagMax),
		ChapterTitles:     chapters,
		PinnedComment:     firstNonEmpty(ci.PinnedComment, defaultPinned(pkg)),
		Playlists:         topStrings(dedupePlaylists(playlists), 4),
		ThumbnailText:     topStrings(dedupe(thumb), 4),
	}
}

// leadWithTopic guarantees the description opens on the article's topic: if no
// primary keyword appears in the first 150 characters, it prepends the (grounded)
// blog title as a lead sentence. It never invents copy — the lead is the title.
func leadWithTopic(desc string, primary []string, title string) string {
	if descLeadsWithKeyword(desc, primary) {
		return desc
	}
	lead := strings.TrimRight(collapse(title), " .:—-")
	if lead == "" || strings.HasPrefix(strings.ToLower(collapse(desc)), strings.ToLower(lead)) {
		return desc
	}
	return lead + ". " + collapse(desc)
}

// thumbnailTopicLines returns up to two short, upper-cased topic labels for the
// thumbnail, drawn from the primary keywords (≤ 3 words each).
func thumbnailTopicLines(k Keywords) []string {
	var lines []string
	for _, p := range k.Primary {
		words := strings.Fields(p)
		if len(words) > 3 {
			words = words[:3]
		}
		if line := strings.ToUpper(strings.Join(words, " ")); line != "" {
			lines = append(lines, line)
		}
		if len(lines) >= 2 {
			break
		}
	}
	return lines
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

// shortenYouTubeTitle keeps a YouTube title within the preferred display length
// (YouTubeTitlePref, 70). A "Headline: subtitle" title uses the headline — a
// natural short title — rather than a mid-phrase cut; otherwise it truncates on
// a word boundary. It never exceeds the hard YouTubeTitleMax.
func shortenYouTubeTitle(title string) string {
	title = collapse(title)
	if len(title) <= YouTubeTitlePref {
		return title
	}
	if i := strings.Index(title, ": "); i >= 15 && i <= YouTubeTitlePref {
		return strings.TrimSpace(title[:i])
	}
	// Word-boundary cut with no ellipsis — a title reads better clipped cleanly,
	// and this keeps the result within the preferred byte length.
	cut := title[:YouTubeTitlePref]
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:—-")
}

// dedupePlaylists removes playlists that name the same series, keeping the first.
// The suggested playlist is repo-qualified ("<repo> — Release Deep Dives") while
// the fallbacks are bare ("Release Deep Dives"); exact-match dedup missed that,
// so we key on the trailing series name after an em dash.
func dedupePlaylists(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		key := strings.ToLower(strings.TrimSpace(p))
		if i := strings.LastIndex(key, "— "); i >= 0 {
			key = strings.TrimSpace(key[i+len("— "):])
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}
