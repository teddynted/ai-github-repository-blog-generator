// Package tiktok is the Social Video engine (Milestone 8). It adapts the
// previously generated artifacts — Release Context (M2), Technical Blog (M3),
// Storyboard (M4), Voice-over Script (M5), long-form YouTube Script (M6), and
// YouTube Shorts (M7) — into multiple TikTok-native educational videos.
//
// It plans Social Video; it does not render it. It ADAPTS existing content for
// TikTok and never regenerates repository knowledge, and it never invents
// architecture or implementation details: every video is anchored to a real
// Short/chapter/scene and a grounded fact from the Release Context. It is
// deterministic where it matters — topic discovery, timing, scenes, captions,
// visuals, camera, animation, engagement, hashtags, metadata, and validation are
// rule-based and independently testable. The shared releasegen.Model port is
// used only to sharpen hooks, scripts, and captions, with a deterministic
// fallback so nothing is fabricated.
package tiktok

// SchemaVersion is the TikTok document version (SemVer, additive-only) so future
// rendering/publishing/subtitle/analytics milestones extend it without breaking.
const SchemaVersion = "1.0.0"

// TikTokCollection is the full set of TikTok videos derived from one release.
type TikTokCollection struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Metadata            Metadata     `json:"metadata"`
	Videos              []Video      `json:"videos"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source artifacts the videos were adapted from.
type Metadata struct {
	Repository      string            `json:"repository"`
	Release         string            `json:"release"`
	SourceBlogTitle string            `json:"sourceBlogTitle"`
	GeneratedAt     string            `json:"generatedAt"`
	VideoCount      int               `json:"videoCount"`
	SourceSchemas   map[string]string `json:"sourceSchemas,omitempty"`
}

// Video is one TikTok-native 20–60 second vertical video plan teaching a single
// technical concept.
type Video struct {
	ID               int         `json:"id"`
	Title            string      `json:"title"`
	Topic            string      `json:"topic"`    // AWS Tip | CloudFormation Trick | AI Workflow | GitHub Automation | Architecture Insight | Performance Improvement | Code Optimization | Common Mistake | Developer Productivity | Deployment Strategy | Interesting Statistic | Best Practice
	Duration         string      `json:"duration"` // "M:SS"
	DurationSec      int         `json:"durationSec"`
	Hook             string      `json:"hook"`
	Script           string      `json:"script"` // full spoken script (hook → problem → solution → takeaway → engagement → CTA)
	Problem          string      `json:"problem"`
	Solution         string      `json:"solution"`
	Takeaway         string      `json:"takeaway"`
	Scenes           []Scene     `json:"scenes"`
	Captions         []Caption   `json:"captions"`
	Visuals          []Visual    `json:"visuals"`
	Camera           []CameraDir `json:"camera"`
	Animations       []Animation `json:"animations"`
	EngagementPrompt string      `json:"engagementPrompt"`
	CTA              string      `json:"cta"`
	Hashtags         []string    `json:"hashtags"`
	SEO              SEO         `json:"seo"`
	Source           Source      `json:"source"`         // provenance / grounding
	RetentionScore   int         `json:"retentionScore"` // 0–100 heuristic
	WordCount        int         `json:"wordCount"`
}

// Scene is one shot within a TikTok (vertical, fast-paced).
type Scene struct {
	Number      int    `json:"number"`
	DurationSec int    `json:"durationSec"`
	Narration   string `json:"narration"`
	Visual      string `json:"visual"`
	Camera      string `json:"camera"`
	Animation   string `json:"animation"`
	Overlay     string `json:"overlay,omitempty"`
	Transition  string `json:"transition"`
}

// Caption is a timed, burned-in subtitle chunk.
type Caption struct {
	Text     string `json:"text"`
	StartSec int    `json:"startSec"`
	EndSec   int    `json:"endSec"`
	Style    string `json:"style,omitempty"` // large-bold | highlight
}

// Visual is a grounded on-screen element. Reference ties it back to a real
// artifact (a Mermaid source, the repo, an AWS service, a chapter/Short).
type Visual struct {
	Kind        string `json:"kind"` // Repository Screenshot | GitHub Release Page | CloudFormation Template | AWS Console | Terminal Recording | Code Walkthrough | Mermaid Diagram | Architecture Animation | Command Execution | Editor View
	Description string `json:"description"`
	Reference   string `json:"reference,omitempty"`
}

// CameraDir is a camera instruction for a scene.
type CameraDir struct {
	Scene     int    `json:"scene"`
	Direction string `json:"direction"` // Zoom | Pan | Push In | Pull Out | Focus Code | Highlight Diagram | Screen Recording | Repository Tour
	Target    string `json:"target,omitempty"`
}

// Animation is an animation instruction for a scene.
type Animation struct {
	Scene    int    `json:"scene"`
	Type     string `json:"type"` // Fade | Slide | Typing Animation | Diagram Build | Arrow Highlights | Zoom Effects | Callout Popups | Pulse | Code Highlight
	Target   string `json:"target,omitempty"`
	Sequence int    `json:"sequence"`
}

// SEO is per-video publishing metadata.
type SEO struct {
	AlternativeTitles []string `json:"alternativeTitles,omitempty"`
	Caption           string   `json:"caption"`
	PostingTime       string   `json:"postingTime,omitempty"`
}

// Source records where a video was adapted from, for grounding and traceability.
type Source struct {
	Short           int    `json:"short,omitempty"`           // YouTube Short ID adapted
	Chapter         int    `json:"chapter,omitempty"`         // YouTube chapter number
	StoryboardScene int    `json:"storyboardScene,omitempty"` // storyboard scene number
	Seed            string `json:"seed,omitempty"`            // the grounded fact the video is built on
}

// Intelligence is collection-level production metadata.
type Intelligence struct {
	VideoCount         int      `json:"videoCount"`
	TotalDurationSec   int      `json:"totalDurationSec"`
	AverageDurationSec int      `json:"averageDurationSec"`
	AverageRetention   int      `json:"averageRetentionScore"`
	Topics             []string `json:"topics,omitempty"`
	Audience           string   `json:"audience"`
	Difficulty         string   `json:"difficulty"`
	SEOKeywords        []string `json:"seoKeywords,omitempty"`
	PostingCadence     string   `json:"postingCadence"`
	ProductionNotes    []string `json:"productionNotes,omitempty"`
}
