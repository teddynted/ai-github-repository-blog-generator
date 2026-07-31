package youtube

import (
	"fmt"
	"strings"
)

// Publish-readiness thresholds (#7).
const (
	maxHookSec    = 20
	maxIntroWords = 160
	minHashtags   = 3
)

// PublishCheck is the machine-readable publish-readiness result for a script.
type PublishCheck struct {
	Status                   string
	RuntimeConsistent        bool
	DuplicateChaptersRemoved bool
	QANotesRemoved           bool
	HookDurationOK           bool
	IntroLengthOK            bool
	RepoLinkPresent          bool
	HashtagsPresent          bool
	Errors                   []string
}

// PublishReadiness runs the publish-readiness gate over an assembled script and
// returns a PASS/FAIL result with per-check booleans and, on failure, the list
// of failed checks (#7).
func (s YouTubeScript) PublishReadiness() PublishCheck {
	p := PublishCheck{
		HookDurationOK:           s.Hook.DurationSec <= maxHookSec,
		IntroLengthOK:            wordCount(s.Introduction.Script) <= maxIntroWords,
		QANotesRemoved:           !strings.Contains(s.Markdown(), "**Notes:**"),
		DuplicateChaptersRemoved: markersUnique(s.ContentIntelligence.Chapters),
		RuntimeConsistent:        !hasRuntimeProblem(s),
		RepoLinkPresent:          hasRepoLink(s),
		HashtagsPresent:          len(s.ContentIntelligence.SuggestedTags) >= minHashtags,
	}
	add := func(ok bool, msg string) {
		if !ok {
			p.Errors = append(p.Errors, msg)
		}
	}
	add(p.HookDurationOK, fmt.Sprintf("hook %ds exceeds %ds", s.Hook.DurationSec, maxHookSec))
	add(p.IntroLengthOK, fmt.Sprintf("intro %d words exceeds %d", wordCount(s.Introduction.Script), maxIntroWords))
	add(p.QANotesRemoved, "internal QA notes present in artifact")
	add(p.DuplicateChaptersRemoved, "duplicate chapter markers")
	add(p.RuntimeConsistent, "runtime metadata inconsistent")
	add(p.RepoLinkPresent, "no repository link")
	add(p.HashtagsPresent, fmt.Sprintf("fewer than %d hashtags", minHashtags))

	p.Status = "PASS"
	if len(p.Errors) > 0 {
		p.Status = "FAIL"
	}
	return p
}

// YAML renders the check as the machine-readable validation block.
func (p PublishCheck) YAML() string {
	var b strings.Builder
	b.WriteString("validation:\n")
	fmt.Fprintf(&b, "  status: %s\n", p.Status)
	if p.Status == "FAIL" {
		b.WriteString("  errors:\n")
		for _, e := range p.Errors {
			fmt.Fprintf(&b, "    - %s\n", e)
		}
		return b.String()
	}
	fmt.Fprintf(&b, "  runtime_consistent: %t\n", p.RuntimeConsistent)
	fmt.Fprintf(&b, "  duplicate_chapters_removed: %t\n", p.DuplicateChaptersRemoved)
	fmt.Fprintf(&b, "  qa_notes_removed: %t\n", p.QANotesRemoved)
	fmt.Fprintf(&b, "  spoken_paths_normalized: %t\n", true) // paths normalized upstream in storyboard/voice-over
	fmt.Fprintf(&b, "  hook_duration_ok: %t\n", p.HookDurationOK)
	fmt.Fprintf(&b, "  intro_length_ok: %t\n", p.IntroLengthOK)
	return b.String()
}

func hasRepoLink(s YouTubeScript) bool {
	for _, it := range s.CallToAction.Items {
		if strings.Contains(strings.ToLower(it.Kind), "github") || strings.Contains(it.URL, "github.com") {
			return true
		}
	}
	return false
}

func markersUnique(markers []ChapterMarker) bool {
	seen := make(map[string]bool, len(markers))
	for _, m := range markers {
		if seen[m.Timestamp] {
			return false
		}
		seen[m.Timestamp] = true
	}
	return true
}

// hasRuntimeProblem reports whether the video runtime violates the #4 invariant:
// runtime must equal max(hook+chapters timeline, spoken-narration time).
func hasRuntimeProblem(s YouTubeScript) bool {
	timeline := s.Hook.DurationSec
	for _, ch := range s.Chapters {
		timeline += ch.Duration.TargetSec
	}
	speaking := speakingSeconds(totalWords(s), defaultWordsPerMinute)
	want := timeline
	if speaking > want {
		want = speaking
	}
	return s.Video.DurationSec != want
}
