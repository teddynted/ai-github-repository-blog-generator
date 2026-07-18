// Package shorts is the Short-form Video engine (Milestone 7). It mines the
// previously generated artifacts — Release Context (M2), Technical Blog (M3),
// Storyboard (M4), Voice-over Script (M5), and long-form YouTube Script (M6) —
// for the most valuable technical moments and turns each into a standalone
// 30–60 second YouTube Short plan.
//
// It plans Short-form Video; it does not render it. It CONSUMES the upstream
// artifacts and never regenerates them, and it never invents architecture or
// implementation details: every Short is anchored to a real chapter/scene and a
// grounded fact from the Release Context. It is deterministic where it matters —
// discovery, timing, scenes, visuals, camera, animation, hashtags, metadata, and
// validation are rule-based and independently testable. The shared
// releasegen.Model port is used only to sharpen hooks, narration, and captions,
// with a deterministic fallback so nothing is fabricated.
package shorts

// SchemaVersion is the Shorts document version (SemVer, additive-only) so future
// rendering/publishing/subtitle milestones extend it without breaking.
const SchemaVersion = "1.0.0"

// ShortsCollection is the full set of Shorts derived from one release.
type ShortsCollection struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Metadata            Metadata     `json:"metadata"`
	Shorts              []Short      `json:"shorts"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source artifacts the Shorts were mined from.
type Metadata struct {
	Repository      string            `json:"repository"`
	Release         string            `json:"release"`
	SourceBlogTitle string            `json:"sourceBlogTitle"`
	GeneratedAt     string            `json:"generatedAt"`
	ShortCount      int               `json:"shortCount"`
	SourceSchemas   map[string]string `json:"sourceSchemas,omitempty"`
}

// Short is one standalone 30–60 second vertical video plan communicating a
// single technical idea.
type Short struct {
	ID              int         `json:"id"`
	Title           string      `json:"title"`
	Angle           string      `json:"angle"`    // Architecture Reveal | Performance Improvement | CloudFormation Tip | AWS Best Practice | Code Walkthrough | Interesting Statistic | Demo Highlight | Lesson Learned | Optimization | Common Mistake | Developer Tip
	Duration        string      `json:"duration"` // "M:SS"
	DurationSec     int         `json:"durationSec"`
	Hook            string      `json:"hook"`
	Script          string      `json:"script"` // full spoken script (hook → core → takeaway → CTA)
	CoreExplanation string      `json:"coreExplanation"`
	Takeaway        string      `json:"takeaway"`
	Scenes          []Scene     `json:"scenes"`
	Captions        []Caption   `json:"captions"`
	Visuals         []Visual    `json:"visuals"`
	Camera          []CameraDir `json:"camera"`
	Animations      []Animation `json:"animations"`
	CTA             string      `json:"cta"`
	Hashtags        []string    `json:"hashtags"`
	SEO             SEO         `json:"seo"`
	Source          Source      `json:"source"` // provenance / grounding
	WordCount       int         `json:"wordCount"`
}

// Scene is one shot within a Short (vertical, fast-paced).
type Scene struct {
	Number      int    `json:"number"`
	DurationSec int    `json:"durationSec"`
	Visual      string `json:"visual"`
	Camera      string `json:"camera"`
	Animation   string `json:"animation"`
	Overlay     string `json:"overlay,omitempty"`
	Transition  string `json:"transition"`
	Narration   string `json:"narration"`
}

// Caption is a timed, burned-in subtitle chunk.
type Caption struct {
	Text     string `json:"text"`
	StartSec int    `json:"startSec"`
	EndSec   int    `json:"endSec"`
	Style    string `json:"style,omitempty"` // large-bold | emphasis
}

// Visual is a grounded on-screen element. Reference ties it back to a real
// artifact (a Mermaid source, the repo URL, a storyboard scene, a chapter).
type Visual struct {
	Kind        string `json:"kind"` // Repository Screenshot | GitHub Release | Terminal Recording | Architecture Diagram | Mermaid Animation | CloudFormation Template | Code Highlight | AWS Console | Animated Callout
	Description string `json:"description"`
	Reference   string `json:"reference,omitempty"`
}

// CameraDir is a camera instruction for a scene.
type CameraDir struct {
	Scene     int    `json:"scene"`
	Direction string `json:"direction"` // Zoom In | Zoom Out | Pan | Code Focus | Diagram Focus | Terminal Focus | Highlight Resource | Screen Recording
	Target    string `json:"target,omitempty"`
}

// Animation is an animation instruction for a scene.
type Animation struct {
	Scene    int    `json:"scene"`
	Type     string `json:"type"` // Fade | Slide | Zoom | Highlight | Arrow | Pulse | Typing | Diagram Build | Sequential Reveal
	Target   string `json:"target,omitempty"`
	Sequence int    `json:"sequence"`
}

// SEO is per-Short publishing metadata.
type SEO struct {
	AlternativeTitles    []string `json:"alternativeTitles,omitempty"`
	ThumbnailText        string   `json:"thumbnailText"`
	Description          string   `json:"description"`
	SuggestedPublishTime string   `json:"suggestedPublishTime,omitempty"`
}

// Source records where a Short was mined from, for grounding and traceability.
type Source struct {
	Chapter         int    `json:"chapter,omitempty"`         // YouTube chapter number
	StoryboardScene int    `json:"storyboardScene,omitempty"` // storyboard scene number
	Seed            string `json:"seed,omitempty"`            // the grounded fact the Short is built on
}

// Intelligence is collection-level production metadata.
type Intelligence struct {
	ShortCount         int      `json:"shortCount"`
	TotalDurationSec   int      `json:"totalDurationSec"`
	AverageDurationSec int      `json:"averageDurationSec"`
	Topics             []string `json:"topics,omitempty"`
	Audience           string   `json:"audience"`
	Difficulty         string   `json:"difficulty"`
	SEOKeywords        []string `json:"seoKeywords,omitempty"`
	PublishCadence     string   `json:"publishCadence"`
	ProductionNotes    []string `json:"productionNotes,omitempty"`
}
