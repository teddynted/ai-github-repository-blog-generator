package shorts

import (
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// maxShorts bounds how many Shorts one release yields by default.
const maxShorts = 6

// candidate is a discovered Short-worthy moment, anchored to a real source.
type candidate struct {
	Angle           string
	Title           string
	Seed            string // grounded fact the Short is built on
	Chapter         int    // YouTube chapter number (0 = none)
	SceneType       string // storyboard scene type, for visual/camera planning
	StoryboardScene int
}

// discover mines the release package for Short-worthy moments, most-valuable
// first, then selects a diverse, deduplicated set (≤ max). Every candidate is
// grounded: its seed comes from a real callout, chapter, or Release Context
// statistic — never invented.
func discover(pkg ReleasePackage, max int) []candidate {
	if max <= 0 {
		max = maxShorts
	}
	var cands []candidate

	// 1. Callout-driven candidates from the (already grounded) YouTube chapters.
	for _, ch := range pkg.YouTube.Chapters {
		for _, co := range ch.Callouts {
			if angle := angleForCallout(co.Kind, ch.Type); angle != "" && collapse(co.Text) != "" {
				cands = append(cands, candidate{
					Angle: angle, Title: titleFor(angle, pkg), Seed: co.Text,
					Chapter: ch.Number, SceneType: ch.Type, StoryboardScene: storyboardSceneOf(ch),
				})
			}
		}
	}

	// 2. Chapter-type candidates, so a release still yields Shorts when callouts
	//    are sparse.
	for _, ch := range pkg.YouTube.Chapters {
		if angle := angleForChapter(ch); angle != "" {
			cands = append(cands, candidate{
				Angle: angle, Title: titleFor(angle, pkg), Seed: firstSentences(ch.Script, 2),
				Chapter: ch.Number, SceneType: ch.Type, StoryboardScene: storyboardSceneOf(ch),
			})
		}
	}

	// 3. A statistic-driven candidate from the Release Context.
	if seed := statisticSeed(pkg); seed != "" {
		cands = append(cands, candidate{Angle: "Interesting Statistic", Title: titleFor("Interesting Statistic", pkg), Seed: seed})
	}

	return selectDiverse(cands, max)
}

// angleForCallout maps a chapter callout kind to a Short angle.
func angleForCallout(kind, chapterType string) string {
	switch kind {
	case "Architecture Decision":
		return "Architecture Reveal"
	case "Best Practice":
		return "AWS Best Practice"
	case "Tip":
		return "Developer Tip"
	case "Performance Note":
		return "Optimization"
	case "Lesson Learned":
		return "Lesson Learned"
	case "Common Mistake":
		return "Common Mistake"
	case "Warning":
		if chapterType == "cloudformation" {
			return "CloudFormation Tip"
		}
		return "Common Mistake"
	default:
		return ""
	}
}

// angleForChapter maps a chapter type to a Short angle.
func angleForChapter(ch youtube.Chapter) string {
	switch ch.Type {
	case "architecture", "diagram":
		return "Architecture Reveal"
	case "cloudformation":
		return "CloudFormation Tip"
	case "implementation":
		if len(ch.Demonstration) > 0 {
			return "Code Walkthrough"
		}
		return "Code Walkthrough"
	case "results":
		return "Demo Highlight"
	default:
		return ""
	}
}

// statisticSeed builds a grounded, quotable statistic from the Release Context.
// statisticSeed is retired for evergreen body copy: a Short must speak to what the
// system does, never a commit / file / line count or "this release" framing. There
// is no evergreen way to phrase a raw changelog statistic, so this yields no seed
// and the statistic angle is dropped downstream in favour of substantive topics.
func statisticSeed(_ ReleasePackage) string { return "" }

// selectDiverse picks up to max candidates favouring angle diversity (one per
// angle first), and drops duplicates by title.
func selectDiverse(cands []candidate, max int) []candidate {
	var out []candidate
	usedAngle := map[string]bool{}
	usedTitle := map[string]bool{}

	// First pass: one per angle.
	for _, c := range cands {
		if len(out) >= max {
			break
		}
		if usedAngle[c.Angle] || usedTitle[slug(c.Title)] {
			continue
		}
		usedAngle[c.Angle] = true
		usedTitle[slug(c.Title)] = true
		out = append(out, c)
	}
	// Second pass: fill remaining slots with any non-duplicate title.
	for _, c := range cands {
		if len(out) >= max {
			break
		}
		if usedTitle[slug(c.Title)] {
			continue
		}
		usedTitle[slug(c.Title)] = true
		out = append(out, c)
	}
	return out
}

// titleFor builds a marketing title for an angle, grounded to the repo/release.
func titleFor(angle string, pkg ReleasePackage) string {
	tag := releaseTag(pkg)
	switch angle {
	case "Architecture Reveal":
		return "This architecture in under a minute"
	case "AWS Best Practice":
		return "An AWS best practice worth stealing"
	case "Developer Tip":
		return "A developer tip from " + tag
	case "Optimization":
		return "The optimization that mattered"
	case "Lesson Learned":
		return "A lesson from shipping " + tag
	case "Common Mistake":
		return "Most developers get this wrong"
	case "CloudFormation Tip":
		return "One CloudFormation tip to remember"
	case "Code Walkthrough":
		return "How this code actually works"
	case "Demo Highlight":
		return tag + " in action"
	case "Interesting Statistic":
		return "The numbers behind the build"
	default:
		return angle
	}
}

// storyboardSceneOf returns the storyboard scene a chapter maps to (chapters are
// 1:1 with scenes), or 0.
func storyboardSceneOf(ch youtube.Chapter) int {
	if len(ch.StoryboardScenes) > 0 {
		return ch.StoryboardScenes[0]
	}
	return 0
}
