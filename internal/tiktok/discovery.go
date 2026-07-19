package tiktok

import (
	"fmt"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// maxVideos bounds how many TikToks one release yields by default.
const maxVideos = 6

// topic is a discovered TikTok-worthy concept, anchored to a real source.
type topic struct {
	Topic           string // TikTok topic label
	Title           string
	Seed            string // grounded fact
	Short           int    // source YouTube Short ID (0 = none)
	Chapter         int
	StoryboardScene int
}

// discover finds TikTok topics, most-valuable first, then selects a diverse,
// deduplicated set (≤ max). It PREFERS adapting existing YouTube Shorts (M7);
// when none are present it mines the YouTube chapters and Release Context. Every
// topic is grounded — never invented.
func discover(pkg ReleasePackage, max int) []topic {
	if max <= 0 {
		max = maxVideos
	}
	var topics []topic

	// 1. Adapt existing Shorts 1:1 (the primary path — reuse, don't regenerate).
	for _, s := range pkg.Shorts.Shorts {
		tk := shortAngleToTopic(s.Angle)
		seed := firstNonEmpty(s.Source.Seed, s.CoreExplanation, s.Hook)
		topics = append(topics, topic{
			Topic: tk, Title: titleFor(tk, pkg), Seed: seed,
			Short: s.ID, Chapter: s.Source.Chapter, StoryboardScene: s.Source.StoryboardScene,
		})
	}

	// 2. Fallback: mine from the YouTube chapters and their callouts.
	if len(topics) == 0 {
		for _, ch := range pkg.YouTube.Chapters {
			for _, co := range ch.Callouts {
				if tk := calloutToTopic(co.Kind, ch.Type); tk != "" && collapse(co.Text) != "" {
					topics = append(topics, topic{
						Topic: tk, Title: titleFor(tk, pkg), Seed: co.Text,
						Chapter: ch.Number, StoryboardScene: storyboardSceneOf(ch),
					})
				}
			}
			if tk := chapterToTopic(ch.Type); tk != "" {
				topics = append(topics, topic{
					Topic: tk, Title: titleFor(tk, pkg), Seed: firstSentences(ch.Script, 2),
					Chapter: ch.Number, StoryboardScene: storyboardSceneOf(ch),
				})
			}
		}
	}

	// 3. Always offer a grounded statistic topic.
	if seed := statisticSeed(pkg); seed != "" {
		topics = append(topics, topic{Topic: "Interesting Statistic", Title: titleFor("Interesting Statistic", pkg), Seed: seed})
	}

	return selectDiverse(topics, max)
}

// shortAngleToTopic maps a YouTube Short angle to a TikTok topic.
func shortAngleToTopic(angle string) string {
	switch angle {
	case "Architecture Reveal":
		return "Architecture Insight"
	case "AWS Best Practice":
		return "AWS Tip"
	case "CloudFormation Tip":
		return "CloudFormation Trick"
	case "Optimization":
		return "Code Optimization"
	case "Developer Tip":
		return "Developer Productivity"
	case "Common Mistake":
		return "Common Mistake"
	case "Lesson Learned":
		return "Best Practice"
	case "Code Walkthrough":
		return "Developer Productivity"
	case "Demo Highlight":
		return "GitHub Automation"
	case "Interesting Statistic":
		return "Interesting Statistic"
	default:
		return "Best Practice"
	}
}

func calloutToTopic(kind, chapterType string) string {
	switch kind {
	case "Architecture Decision":
		return "Architecture Insight"
	case "Best Practice":
		return "AWS Tip"
	case "Tip":
		return "Developer Productivity"
	case "Performance Note":
		return "Performance Improvement"
	case "Lesson Learned":
		return "Best Practice"
	case "Common Mistake":
		return "Common Mistake"
	case "Warning":
		if chapterType == "cloudformation" {
			return "CloudFormation Trick"
		}
		return "Common Mistake"
	default:
		return ""
	}
}

func chapterToTopic(chapterType string) string {
	switch chapterType {
	case "architecture", "diagram":
		return "Architecture Insight"
	case "cloudformation":
		return "CloudFormation Trick"
	case "implementation":
		return "Developer Productivity"
	case "results":
		return "GitHub Automation"
	default:
		return ""
	}
}

func statisticSeed(pkg ReleasePackage) string {
	c := pkg.Context
	if c == nil {
		return ""
	}
	commits := c.CommitStats.Analyzed
	if commits == 0 {
		commits = c.CommitStats.Total
	}
	files := c.FileStats.Total
	switch {
	case commits > 0 && files > 0:
		return fmt.Sprintf("This release landed %d commits across %d changed files.", commits, files)
	case commits > 0:
		return fmt.Sprintf("This release analyzed %d commits.", commits)
	case len(c.Changelog.Features) > 0:
		return fmt.Sprintf("This release shipped %d new features.", len(c.Changelog.Features))
	default:
		return ""
	}
}

// selectDiverse picks up to max topics favouring topic diversity (one per topic
// first), dropping duplicate titles.
func selectDiverse(topics []topic, max int) []topic {
	var out []topic
	usedTopic := map[string]bool{}
	usedTitle := map[string]bool{}
	for _, t := range topics {
		if len(out) >= max {
			break
		}
		if usedTopic[t.Topic] || usedTitle[slug(t.Title)] {
			continue
		}
		usedTopic[t.Topic] = true
		usedTitle[slug(t.Title)] = true
		out = append(out, t)
	}
	for _, t := range topics {
		if len(out) >= max {
			break
		}
		if usedTitle[slug(t.Title)] {
			continue
		}
		usedTitle[slug(t.Title)] = true
		out = append(out, t)
	}
	return out
}

// titleFor builds a TikTok-native title for a topic.
func titleFor(t string, pkg ReleasePackage) string {
	tag := releaseTag(pkg)
	switch t {
	case "Architecture Insight":
		return "The architecture nobody explains"
	case "AWS Tip":
		return "An AWS tip most devs miss"
	case "CloudFormation Trick":
		return "This CloudFormation trick is underrated"
	case "Code Optimization":
		return "The optimization that actually mattered"
	case "Developer Productivity":
		return "Steal this developer workflow"
	case "Common Mistake":
		return "You're probably doing this wrong"
	case "Best Practice":
		return "A best practice worth stealing"
	case "GitHub Automation":
		return "This GitHub automation saves hours"
	case "Performance Improvement":
		return "How we made it faster"
	case "Deployment Strategy":
		return "Deploy it the smart way"
	case "AI Workflow":
		return "The AI workflow behind " + tag
	case "Interesting Statistic":
		return "The numbers behind " + tag
	default:
		return t
	}
}

func storyboardSceneOf(ch youtube.Chapter) int {
	if len(ch.StoryboardScenes) > 0 {
		return ch.StoryboardScenes[0]
	}
	return 0
}
