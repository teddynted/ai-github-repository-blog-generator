// Package storyboard is the Video Planning engine (Milestone 4). It converts a
// generated technical blog (Milestone 3) plus its Release Context (Milestone 2)
// into a structured, scene-by-scene storyboard: the canonical input for future
// video-generation pipelines (YouTube, Shorts, TikTok, AI video, voice-over,
// motion graphics).
//
// It is deterministic where it matters — scene extraction, timing, camera,
// animation, overlay, transition, asset planning, Mermaid references, and
// content intelligence are all rule-based and independently testable. The LLM
// (the shared releasegen.Model port) is used only to refine narration and visual
// descriptions, with a deterministic fallback, so architecture is never invented:
// diagram references always come from the parsed Mermaid in the Release Context.
package storyboard

// SchemaVersion is the storyboard document version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// Storyboard is the complete video plan for one technical blog.
type Storyboard struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Metadata            Metadata     `json:"metadata"`
	Video               VideoSpec    `json:"video"`
	Scenes              []Scene      `json:"scenes"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source of the storyboard.
type Metadata struct {
	Repository      string `json:"repository"`
	Release         string `json:"release"`
	SourceBlogTitle string `json:"sourceBlogTitle"`
	GeneratedAt     string `json:"generatedAt"`
}

// VideoSpec describes the target video as a whole.
type VideoSpec struct {
	TargetFormat         string `json:"targetFormat"` // long-form | short
	AspectRatio          string `json:"aspectRatio"`
	SceneCount           int    `json:"sceneCount"`
	TotalDurationSec     int    `json:"totalDurationSec"`
	VoiceoverDurationSec int    `json:"voiceoverDurationSec"`
	Pacing               string `json:"pacing"`
}

// Scene is one shot in the storyboard.
type Scene struct {
	SceneNumber  int          `json:"sceneNumber"`
	Title        string       `json:"title"`
	Type         string       `json:"type"` // introduction | problem | architecture | cloudformation | repository | implementation | results | lessons | conclusion | diagram | generic
	Objective    string       `json:"objective"`
	Duration     Duration     `json:"duration"`
	Narration    string       `json:"narration"`
	Visuals      Visuals      `json:"visuals"`
	Camera       Camera       `json:"camera"`
	Animations   []Animation  `json:"animations,omitempty"`
	Overlays     []Overlay    `json:"overlays,omitempty"`
	Diagrams     []DiagramRef `json:"diagrams,omitempty"`
	Code         []CodeRef    `json:"code,omitempty"`
	Transition   Transition   `json:"transition"`
	MusicMood    string       `json:"musicMood,omitempty"`
	SoundEffects []string     `json:"soundEffects,omitempty"`
	Assets       []string     `json:"assets,omitempty"`
}

// Duration is the timing plan for a scene (seconds).
type Duration struct {
	MinSec         int    `json:"minSec"`
	MaxSec         int    `json:"maxSec"`
	RecommendedSec int    `json:"recommendedSec"`
	Pacing         string `json:"pacing"` // fast | medium | slow
}

// Visuals describes what fills the frame.
type Visuals struct {
	Description string `json:"description"`
	Style       string `json:"style,omitempty"`
	Background  string `json:"background,omitempty"`
}

// Camera is the camera direction for a scene.
type Camera struct {
	Direction string `json:"direction"` // Static | Slow Zoom In | Push In | Diagram Focus | Code Highlight | ...
	Notes     string `json:"notes,omitempty"`
}

// Animation is one animation cue.
type Animation struct {
	Type     string `json:"type"` // Fade In | Draw Arrow | Highlight Node | Code Typing | Diagram Build | ...
	Target   string `json:"target,omitempty"`
	Sequence int    `json:"sequence"`
	Notes    string `json:"notes,omitempty"`
}

// Overlay is an on-screen text/graphic element.
type Overlay struct {
	Kind     string `json:"kind"` // Title | Subtitle | Callout | Feature Highlight | AWS Service Label | Repository Statistic | Key Takeaway | Best Practice
	Text     string `json:"text"`
	Position string `json:"position,omitempty"`
	Timing   string `json:"timing,omitempty"`
}

// DiagramRef points at a real parsed Mermaid diagram from the Release Context.
type DiagramRef struct {
	Source         string   `json:"source"` // originating doc path
	Type           string   `json:"type"`   // flowchart | sequence | ...
	HighlightNodes []string `json:"highlightNodes,omitempty"`
	Animation      string   `json:"animation"` // Diagram Build | Sequential Reveal | ...
	ZoomTarget     string   `json:"zoomTarget,omitempty"`
}

// CodeRef is a code snippet to visualize.
type CodeRef struct {
	Language    string `json:"language"`
	Instruction string `json:"instruction"` // Reveal YAML progressively | Highlight function | Terminal demonstration | ...
	Snippet     string `json:"snippet"`
}

// Transition is the cut into the next scene.
type Transition struct {
	Type        string  `json:"type"` // Fade | Cross Dissolve | Zoom Transition | Diagram Morph | Terminal Wipe | ...
	DurationSec float64 `json:"durationSec"`
}

// Intelligence is production/creative metadata for the whole video.
type Intelligence struct {
	Difficulty           string    `json:"difficulty"`
	Audience             string    `json:"audience"`
	EstimatedVideoLength string    `json:"estimatedVideoLength"` // "M:SS"
	SceneCount           int       `json:"sceneCount"`
	AnimationComplexity  string    `json:"animationComplexity"`
	ProductionComplexity string    `json:"productionComplexity"`
	VoiceoverDurationSec int       `json:"voiceoverDurationSec"`
	ThumbnailConcept     string    `json:"thumbnailConcept"`
	SuggestedTitle       string    `json:"suggestedTitle"`
	YouTubeDescription   string    `json:"youTubeDescription"`
	Chapters             []Chapter `json:"chapters,omitempty"`
	SEOKeywords          []string  `json:"seoKeywords,omitempty"`
}

// Chapter is a YouTube-style chapter marker.
type Chapter struct {
	Title    string `json:"title"`
	StartSec int    `json:"startSec"`
}
