// Package youtube is the Long-form Video engine (Milestone 6). It combines the
// previously generated artifacts — Release Context (M2), Technical Blog (M3),
// Storyboard (M4), and Voice-over Script (M5) — into a single, production-ready
// long-form YouTube script optimized for educational software-engineering videos
// (10–20 minutes).
//
// It CONSUMES those artifacts and never regenerates them, and it never invents
// implementation details: chapters map 1:1 to storyboard scenes, timing comes
// from the storyboard timeline, and architecture/repository facts are drawn from
// the Release Context. It is deterministic where it matters — chapter structure,
// timestamps, callouts, demonstration steps, CTAs, metadata, and validation are
// rule-based and independently testable. The shared releasegen.Model port is
// used only to expand narration, hooks, and transitions into engaging teaching
// prose, with a deterministic fallback (built from the voice-over narration and
// the Release Context) so nothing is fabricated.
package youtube

// SchemaVersion is the YouTube-script document version (SemVer, additive-only)
// so future publishing/subtitle/translation milestones extend it without
// breaking.
const SchemaVersion = "1.0.0"

// YouTubeScript is the complete long-form video script for one release.
type YouTubeScript struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Metadata            Metadata     `json:"metadata"`
	Video               Video        `json:"video"`
	Hook                Hook         `json:"hook"`
	Introduction        Introduction `json:"introduction"`
	Chapters            []Chapter    `json:"chapters"`
	Conclusion          Conclusion   `json:"conclusion"`
	CallToAction        CallToAction `json:"callToAction"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source artifacts the script was assembled from.
type Metadata struct {
	Repository      string            `json:"repository"`
	Release         string            `json:"release"`
	SourceBlogTitle string            `json:"sourceBlogTitle"`
	GeneratedAt     string            `json:"generatedAt"`
	SourceSchemas   map[string]string `json:"sourceSchemas,omitempty"` // provenance of consumed artifacts
}

// Video describes the target video as a whole.
type Video struct {
	Title               string `json:"title"`
	Format              string `json:"format"`   // long-form
	Duration            string `json:"duration"` // "M:SS"
	DurationSec         int    `json:"durationSec"`
	Audience            string `json:"audience"`
	Difficulty          string `json:"difficulty"` // beginner | intermediate | advanced
	TargetMinRuntimeSec int    `json:"targetMinRuntimeSec"`
	TargetMaxRuntimeSec int    `json:"targetMaxRuntimeSec"`
}

// Timestamp places a section on the video timeline.
type Timestamp struct {
	Start    string `json:"start"` // "MM:SS"
	End      string `json:"end"`   // "MM:SS"
	StartSec int    `json:"startSec"`
	EndSec   int    `json:"endSec"`
	Label    string `json:"label"` // "MM:SS–MM:SS"
}

// Duration is a section's timing plan (seconds).
type Duration struct {
	MinSec    int `json:"minSec"`
	MaxSec    int `json:"maxSec"`
	TargetSec int `json:"targetSec"`
}

// Hook is the first 15–30 seconds designed to earn the watch.
type Hook struct {
	Type        string    `json:"type"` // problem | insight | outcome | showcase | comparison
	Script      string    `json:"script"`
	DurationSec int       `json:"durationSec"`
	Timestamp   Timestamp `json:"timestamp"`
}

// Introduction frames the video (30–60s). It references the storyboard's
// introduction scene rather than inventing new context.
type Introduction struct {
	Script           string    `json:"script"`
	DurationSec      int       `json:"durationSec"`
	Timestamp        Timestamp `json:"timestamp"`
	Technologies     []string  `json:"technologies,omitempty"`
	LearningOutcomes []string  `json:"learningOutcomes,omitempty"`
	Agenda           []string  `json:"agenda,omitempty"`
	StoryboardScenes []int     `json:"storyboardScenes,omitempty"`
}

// Chapter is one logical section of the video, mapped 1:1 to a storyboard scene.
type Chapter struct {
	Number              int        `json:"number"`
	Title               string     `json:"title"`
	Type                string     `json:"type"`
	Timestamp           Timestamp  `json:"timestamp"`
	Duration            Duration   `json:"duration"`
	Script              string     `json:"script"`
	VisualReferences    []string   `json:"visualReferences,omitempty"`
	StoryboardScenes    []int      `json:"storyboardScenes"`
	VoiceOverReferences []int      `json:"voiceOverReferences"`
	Demonstration       []DemoStep `json:"demonstration,omitempty"`
	Callouts            []Callout  `json:"callouts,omitempty"`
	Engagement          []string   `json:"engagement,omitempty"`
	Transition          string     `json:"transition"`
	WordCount           int        `json:"wordCount"`
}

// DemoStep is one on-screen demonstration action.
type DemoStep struct {
	Step   int    `json:"step"`
	Action string `json:"action"`
	Detail string `json:"detail,omitempty"`
}

// Callout is an educational aside surfaced at a teaching moment.
type Callout struct {
	Kind string `json:"kind"` // Best Practice | Tip | Warning | Performance Note | Lesson Learned | Architecture Decision | Common Mistake
	Text string `json:"text"`
}

// Conclusion wraps the video up.
type Conclusion struct {
	Script           string    `json:"script"`
	DurationSec      int       `json:"durationSec"`
	Timestamp        Timestamp `json:"timestamp"`
	WhatWasBuilt     []string  `json:"whatWasBuilt,omitempty"`
	KeyTakeaways     []string  `json:"keyTakeaways,omitempty"`
	NextRelease      string    `json:"nextRelease,omitempty"`
	StoryboardScenes []int     `json:"storyboardScenes,omitempty"`
}

// CallToAction is the closing ask.
type CallToAction struct {
	Script        string    `json:"script"`
	Items         []CTAItem `json:"items"`
	PinnedComment string    `json:"pinnedComment,omitempty"`
}

// CTAItem is one concrete call to action.
type CTAItem struct {
	Kind string `json:"kind"` // GitHub Repository | Documentation | Subscribe | Like | Comment | Future Releases | Contribute
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

// Intelligence is production/SEO metadata for the whole video.
type Intelligence struct {
	EstimatedRuntime     string          `json:"estimatedRuntime"` // "M:SS"
	EstimatedRuntimeSec  int             `json:"estimatedRuntimeSec"`
	WordCount            int             `json:"wordCount"`
	SpeakingTime         string          `json:"speakingTime"` // "M:SS"
	ReadingLevel         string          `json:"readingLevel"`
	Audience             string          `json:"audience"`
	Difficulty           string          `json:"difficulty"`
	TechnicalTopics      []string        `json:"technicalTopics,omitempty"`
	SEOKeywords          []string        `json:"seoKeywords,omitempty"`
	SuggestedTitle       string          `json:"suggestedTitle"`
	AlternativeTitles    []string        `json:"alternativeTitles,omitempty"`
	SuggestedThumbnail   string          `json:"suggestedThumbnailText"`
	SuggestedDescription string          `json:"suggestedDescription"`
	SuggestedTags        []string        `json:"suggestedTags,omitempty"`
	SuggestedPlaylist    string          `json:"suggestedPlaylist,omitempty"`
	PinnedComment        string          `json:"pinnedComment,omitempty"`
	Chapters             []ChapterMarker `json:"chapterMarkers,omitempty"`
}

// ChapterMarker is a YouTube description chapter line.
type ChapterMarker struct {
	Timestamp string `json:"timestamp"` // "MM:SS"
	Title     string `json:"title"`
}
