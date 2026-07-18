package youtube

import (
	"context"
	"fmt"
	"strings"
)

// introduction builds the 30–60s framing section: project overview, release
// summary, technologies, learning outcomes, and the agenda. It references the
// storyboard's introduction scene(s) and is grounded entirely in the Release
// Context. The Model, when present, only smooths the prose.
func (g *Generator) introduction(ctx context.Context, pkg ReleasePackage, chapters []Chapter) Introduction {
	techs := technologies(pkg)
	outcomes := learningOutcomes(pkg, chapters)
	agenda := agendaFrom(chapters)
	scenes := scenesOfType(pkg, "introduction")

	draft := introDraft(pkg, techs, agenda)
	script := draft
	if g.Model != nil && draft != "" {
		if out, err := g.Model.Generate(ctx, introPrompt(draft, agenda)); err == nil {
			if r := collapse(strings.TrimSpace(out)); r != "" {
				script = r
			}
		}
	}
	dur := clampInt(speakingSeconds(wordCount(script), g.wpm()), introMinSec, introMaxSec)
	return Introduction{
		Script:           script,
		DurationSec:      dur,
		Technologies:     techs,
		LearningOutcomes: outcomes,
		Agenda:           agenda,
		StoryboardScenes: scenes,
	}
}

func introDraft(pkg ReleasePackage, techs, agenda []string) string {
	repo := repoName(pkg)
	tag := releaseTag(pkg)
	var b strings.Builder
	fmt.Fprintf(&b, "Welcome back. In this video we're doing a deep dive into %s %s.", repo, tag)
	if pkg.Context != nil {
		if s := firstSentences(pkg.Context.ContentIntelligence.Summary, 2); s != "" {
			fmt.Fprintf(&b, " %s", s)
		}
	}
	if len(techs) > 0 {
		fmt.Fprintf(&b, " We'll be working with %s.", joinAnd(topStrings(techs, 4)))
	}
	if len(agenda) > 0 {
		fmt.Fprintf(&b, " Here's the plan: we'll cover %s.", joinAnd(topStrings(agenda, 5)))
	}
	return collapse(b.String())
}

func introPrompt(draft string, agenda []string) string {
	return fmt.Sprintf(
		"You are writing the introduction (30–60 seconds, spoken) for a long-form technical YouTube video.\n\n"+
			"Rewrite the DRAFT into a warm, confident spoken introduction that frames the video and previews the "+
			"agenda (%s). Sound like an experienced engineer teaching a peer. Use ONLY the facts in the draft — "+
			"invent nothing. Output only the introduction.\n\nDRAFT:\n%s",
		joinAnd(agenda), draft)
}

// technologies gathers the tech stack from the Release Context (technologies +
// AWS services + repository language), grounded and de-duplicated.
func technologies(pkg ReleasePackage) []string {
	var out []string
	if c := pkg.Context; c != nil {
		if c.Repository.Language != "" {
			out = append(out, c.Repository.Language)
		}
		for _, t := range c.Technologies {
			out = append(out, t.Name)
		}
		out = append(out, c.Architecture.AWSServices...)
	}
	if len(out) == 0 {
		out = append(out, pkg.Blog.Tags...)
	}
	return topStrings(dedupe(out), 8)
}

// learningOutcomes derives what the viewer will learn from the chapter topics.
func learningOutcomes(pkg ReleasePackage, chapters []Chapter) []string {
	var out []string
	seen := map[string]bool{}
	for _, ch := range chapters {
		o := outcomeFor(ch.Type)
		if o != "" && !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	if c := pkg.Context; c != nil && c.ContentIntelligence.DeveloperValue != "" {
		out = append(out, "Why it matters: "+firstSentences(c.ContentIntelligence.DeveloperValue, 1))
	}
	return topStrings(out, 6)
}

func outcomeFor(typ string) string {
	switch typ {
	case "problem":
		return "Understand the problem this release solves"
	case "architecture", "diagram":
		return "Read the architecture and how the components fit together"
	case "cloudformation":
		return "See how the infrastructure is defined as code"
	case "repository":
		return "Navigate the repository and its Clean Architecture layout"
	case "implementation":
		return "Follow how the feature was implemented"
	case "results":
		return "Evaluate the outcomes and their impact"
	case "lessons":
		return "Take away practical guidance you can apply"
	default:
		return ""
	}
}

// agendaFrom lists the body chapter titles (skipping the intro/conclusion
// framing chapters) as the on-screen agenda.
func agendaFrom(chapters []Chapter) []string {
	var out []string
	for _, ch := range chapters {
		if ch.Type == "introduction" || ch.Type == "conclusion" {
			continue
		}
		out = append(out, ch.Title)
	}
	if len(out) == 0 { // degenerate: no body chapters, use everything
		for _, ch := range chapters {
			out = append(out, ch.Title)
		}
	}
	return out
}

// scenesOfType returns the storyboard scene numbers matching a scene type.
func scenesOfType(pkg ReleasePackage, typ string) []int {
	var out []int
	for _, sc := range pkg.Storyboard.Scenes {
		if sc.Type == typ {
			out = append(out, sc.SceneNumber)
		}
	}
	return out
}
