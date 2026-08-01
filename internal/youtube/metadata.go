package youtube

import (
	"fmt"
	"strings"
)

// planIntelligence derives the whole-video production/SEO metadata. Everything
// is computed deterministically from the assembled script and the grounded
// Release Context.
func (g *Generator) planIntelligence(pkg ReleasePackage, s YouTubeScript) Intelligence {
	words := totalWords(s)
	topics := technicalTopics(pkg)
	keywords := seoKeywords(pkg, topics)
	markers := chapterMarkers(s)

	ci := Intelligence{
		EstimatedRuntime:    mmss(s.Video.DurationSec),
		EstimatedRuntimeSec: s.Video.DurationSec,
		WordCount:           words,
		SpeakingTime:        mmss(speakingSeconds(words, g.wpm())),
		ReadingLevel:        readingLevel(pkg),
		Audience:            s.Video.Audience,
		Difficulty:          s.Video.Difficulty,
		TechnicalTopics:     topics,
		SEOKeywords:         keywords,
		SuggestedTitle:      s.Video.Title,
		AlternativeTitles:   alternativeTitles(pkg, s.Video.Title),
		SuggestedThumbnail:  thumbnailText(pkg),
		SuggestedTags:       suggestedTags(keywords, topics),
		SuggestedPlaylist:   playlist(pkg),
		PinnedComment:       s.CallToAction.PinnedComment,
		Chapters:            markers,
	}
	ci.SuggestedDescription = description(pkg, markers, keywords)
	return ci
}

func totalWords(s YouTubeScript) int {
	n := wordCount(s.Hook.Script) + wordCount(s.Introduction.Script) + wordCount(s.Conclusion.Script) + wordCount(s.CallToAction.Script)
	for _, ch := range s.Chapters {
		n += ch.WordCount
	}
	return n
}

func technicalTopics(pkg ReleasePackage) []string {
	var out []string
	if c := pkg.Context; c != nil {
		out = append(out, c.Architecture.AWSServices...)
		for _, t := range c.Technologies {
			out = append(out, t.Name)
		}
		out = append(out, c.ContentIntelligence.TechnicalHighlights...)
	}
	return topStrings(dedupe(out), 12)
}

func seoKeywords(pkg ReleasePackage, topics []string) []string {
	var out []string
	if len(pkg.Blog.Tags) > 0 {
		out = append(out, pkg.Blog.Tags...)
	}
	if c := pkg.Context; c != nil {
		out = append(out, c.ContentIntelligence.SEOKeywords...)
	}
	out = append(out, topics...)
	return topStrings(dedupe(out), 15)
}

func readingLevel(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		switch c.ContentIntelligence.ImplementationComplexity {
		case "high":
			return "advanced"
		case "low":
			return "accessible"
		default:
			return "intermediate"
		}
	}
	return "intermediate"
}

func alternativeTitles(pkg ReleasePackage, primary string) []string {
	var out []string
	if c := pkg.Context; c != nil {
		out = append(out, c.ContentIntelligence.BlogTitles...)
	}
	// Evergreen alternatives — no repository name or version.
	out = append(out,
		"How This Architecture Actually Works",
		"An Event-Driven AWS Architecture: A Full Walkthrough",
	)
	// Remove the primary title from the alternatives.
	filtered := out[:0]
	for _, t := range dedupe(out) {
		if !strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(primary)) {
			filtered = append(filtered, t)
		}
	}
	return topStrings(filtered, 5)
}

// thumbnailText is evergreen on-screen thumbnail copy — no version tag.
func thumbnailText(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil && len(c.Architecture.AWSServices) > 0 {
		return strings.ToUpper(c.Architecture.AWSServices[0]) + " · ARCHITECTURE"
	}
	return "AWS · ARCHITECTURE"
}

func suggestedTags(keywords, topics []string) []string {
	base := []string{"software engineering", "aws", "golang", "clean architecture", "devops", "tutorial", "system design"}
	return topStrings(dedupe(append(append([]string{}, keywords...), append(topics, base...)...)), 20)
}

func playlist(pkg ReleasePackage) string {
	return repoName(pkg) + " — Release Deep Dives"
}

// suggestedTitle picks a strong, grounded video title.
func suggestedTitle(pkg ReleasePackage) string {
	if pkg.Blog.Title != "" {
		return pkg.Blog.Title
	}
	if pkg.Storyboard.ContentIntelligence.SuggestedTitle != "" {
		return pkg.Storyboard.ContentIntelligence.SuggestedTitle
	}
	if c := pkg.Context; c != nil && len(c.ContentIntelligence.BlogTitles) > 0 {
		return c.ContentIntelligence.BlogTitles[0]
	}
	return fmt.Sprintf("%s %s — Full Walkthrough", repoName(pkg), releaseTag(pkg))
}

func chapterMarkers(s YouTubeScript) []ChapterMarker {
	markers := []ChapterMarker{{Timestamp: clock(s.Hook.Timestamp.StartSec), Title: "Intro / Hook"}}
	for _, ch := range s.Chapters {
		markers = append(markers, ChapterMarker{Timestamp: clock(ch.Timestamp.StartSec), Title: ch.Title})
	}
	if s.Conclusion.Timestamp.EndSec > s.Conclusion.Timestamp.StartSec {
		markers = append(markers, ChapterMarker{Timestamp: clock(s.Conclusion.Timestamp.StartSec), Title: "Conclusion"})
	}
	return markers
}

func description(pkg ReleasePackage, markers []ChapterMarker, keywords []string) string {
	var b strings.Builder
	if c := pkg.Context; c != nil && c.ContentIntelligence.Summary != "" {
		b.WriteString(c.ContentIntelligence.Summary)
	} else {
		fmt.Fprintf(&b, "A full walkthrough of %s %s.", repoName(pkg), releaseTag(pkg))
	}
	b.WriteString("\n\n⏱ Chapters:\n")
	for _, m := range markers {
		fmt.Fprintf(&b, "%s %s\n", m.Timestamp, m.Title)
	}
	if url := repoURL(pkg); url != "" {
		fmt.Fprintf(&b, "\n🔗 Repository: %s\n", url)
	}
	if docs := docsURL(pkg); docs != "" {
		fmt.Fprintf(&b, "📚 Docs: %s\n", docs)
	}
	if len(keywords) > 0 {
		b.WriteString("\n")
		for _, k := range topStrings(keywords, 10) {
			fmt.Fprintf(&b, "#%s ", strings.ReplaceAll(k, " ", ""))
		}
	}
	return strings.TrimSpace(b.String())
}
