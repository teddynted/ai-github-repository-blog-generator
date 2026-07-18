package storyboard

import (
	"fmt"
	"strings"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// planIntelligence derives production and creative metadata for the whole video.
func planIntelligence(scenes []Scene, post releasegen.BlogPost, rctx *rc.ReleaseContext, video VideoSpec) Intelligence {
	chapters := planChapters(scenes)
	ci := Intelligence{
		Difficulty:           difficultyFor(rctx),
		Audience:             orDefault(rctx.ContentIntelligence.TargetAudience, "Software engineers and cloud practitioners"),
		EstimatedVideoLength: mmss(video.TotalDurationSec),
		SceneCount:           len(scenes),
		AnimationComplexity:  animationComplexity(scenes),
		ProductionComplexity: productionComplexity(scenes),
		VoiceoverDurationSec: video.VoiceoverDurationSec,
		ThumbnailConcept:     thumbnailConcept(rctx),
		SuggestedTitle:       orDefault(post.Title, rctx.Repository.Name+" "+rctx.Release.Tag),
		Chapters:             chapters,
		SEOKeywords:          seoKeywords(post, rctx),
	}
	ci.YouTubeDescription = youTubeDescription(post, rctx, chapters)
	return ci
}

func difficultyFor(rctx *rc.ReleaseContext) string {
	switch rctx.ContentIntelligence.ImplementationComplexity {
	case "high":
		return "advanced"
	case "medium":
		return "intermediate"
	case "low":
		return "beginner"
	default:
		return "intermediate"
	}
}

func animationComplexity(scenes []Scene) string {
	total := 0
	for _, s := range scenes {
		total += len(s.Animations)
	}
	switch {
	case total >= 40:
		return "high"
	case total >= 18:
		return "medium"
	default:
		return "low"
	}
}

func productionComplexity(scenes []Scene) string {
	assets := map[string]bool{}
	hasDiagram, hasCode := false, false
	for _, s := range scenes {
		for _, a := range s.Assets {
			assets[a] = true
		}
		if len(s.Diagrams) > 0 {
			hasDiagram = true
		}
		if len(s.Code) > 0 {
			hasCode = true
		}
	}
	score := len(assets)
	if hasDiagram {
		score += 2
	}
	if hasCode {
		score += 2
	}
	switch {
	case score >= 10:
		return "high"
	case score >= 6:
		return "medium"
	default:
		return "low"
	}
}

func planChapters(scenes []Scene) []Chapter {
	chapters := make([]Chapter, 0, len(scenes))
	start := 0
	for _, s := range scenes {
		chapters = append(chapters, Chapter{Title: s.Title, StartSec: start})
		start += s.Duration.RecommendedSec
	}
	return chapters
}

func thumbnailConcept(rctx *rc.ReleaseContext) string {
	svc := ""
	if len(rctx.Architecture.AWSServices) > 0 {
		svc = ", plus a badge for " + rctx.Architecture.AWSServices[0]
	}
	return fmt.Sprintf(
		"Bold release tag %q over the architecture diagram, with the repository logo%s. High contrast, minimal text.",
		rctx.Release.Tag, svc,
	)
}

func youTubeDescription(post releasegen.BlogPost, rctx *rc.ReleaseContext, chapters []Chapter) string {
	var b strings.Builder
	desc := post.MetaDescription
	if desc == "" {
		desc = rctx.ContentIntelligence.Summary
	}
	b.WriteString(desc)
	b.WriteString("\n\nChapters:\n")
	for _, c := range chapters {
		fmt.Fprintf(&b, "%s %s\n", mmss(c.StartSec), c.Title)
	}
	if tags := seoKeywords(post, rctx); len(tags) > 0 {
		b.WriteString("\n")
		for _, t := range topStrings(tags, 8) {
			fmt.Fprintf(&b, "#%s ", strings.ReplaceAll(t, " ", ""))
		}
	}
	return strings.TrimSpace(b.String())
}

func seoKeywords(post releasegen.BlogPost, rctx *rc.ReleaseContext) []string {
	if len(post.Tags) > 0 {
		return post.Tags
	}
	return rctx.ContentIntelligence.SEOKeywords
}

// mmss formats seconds as "M:SS".
func mmss(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
