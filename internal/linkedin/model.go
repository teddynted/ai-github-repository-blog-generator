// Package linkedin is the Professional Marketing engine (Milestone 12). It
// converts the content pipeline's artifacts — Release Context (M2), Blog (M3),
// Storyboard (M4), Voice-over (M5), YouTube (M6), Shorts (M7), TikTok (M8),
// Visual Assets (M9), SEO (M10), and Architecture diagrams (M11) — into
// professional LinkedIn content: multiple post variations targeting software,
// cloud, and AI engineering audiences.
//
// It generates structured content, not published posts. It CONSUMES the upstream
// artifacts and never regenerates repository knowledge, and it never invents
// features, AWS services, or performance claims: every post is grounded in the
// Release Context. It is deterministic where it matters — post discovery,
// technical highlights, engagement prompts, CTAs, hashtags, visual references,
// metadata, and validation are rule-based and independently testable. The shared
// releasegen.Model port is used only to write post bodies and titles in a
// professional, non-hype voice, with a deterministic fallback so nothing is
// fabricated.
package linkedin

// SchemaVersion is the LinkedIn document version (SemVer, additive-only) so
// future publishing automation extends it without breaking.
const SchemaVersion = "1.0.0"

// LinkedInCollection is the full set of LinkedIn assets for one release.
type LinkedInCollection struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Metadata            Metadata     `json:"metadata"`
	Posts               []Post       `json:"posts"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source artifacts the posts were built from.
type Metadata struct {
	Repository      string            `json:"repository"`
	Release         string            `json:"release"`
	SourceBlogTitle string            `json:"sourceBlogTitle"`
	GeneratedAt     string            `json:"generatedAt"`
	PostCount       int               `json:"postCount"`
	SourceSchemas   map[string]string `json:"sourceSchemas,omitempty"`
}

// Post is one LinkedIn post variation.
type Post struct {
	ID                  int         `json:"id"`
	Type                string      `json:"type"`      // Release Announcement | Feature Spotlight | Architecture Deep Dive | Engineering Lesson | Technical Insight | Behind-the-Build | Performance Improvement | AWS Best Practice | Developer Productivity Tip | AI Engineering Highlight
	Variation           string      `json:"variation"` // Short Update | Medium Post | Long-form Post | Article Introduction
	Audience            string      `json:"audience"`
	Title               string      `json:"title"`
	Summary             string      `json:"summary"`
	Body                string      `json:"body"`
	TechnicalHighlights []string    `json:"technicalHighlights,omitempty"`
	EngagementPrompt    string      `json:"engagementPrompt"`
	CTA                 string      `json:"cta"`
	Hashtags            []string    `json:"hashtags"`
	VisualReferences    []VisualRef `json:"visualReferences,omitempty"`
	Metadata            PostMeta    `json:"metadata"`
}

// VisualRef points at a previously generated visual asset (never regenerated).
type VisualRef struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
	Source    string `json:"source"` // which milestone produced it
}

// PostMeta is per-post metadata.
type PostMeta struct {
	Topic                  string   `json:"topic"`
	Difficulty             string   `json:"difficulty"`
	Tone                   string   `json:"tone"`
	TechnologyStack        []string `json:"technologyStack,omitempty"`
	AWSServices            []string `json:"awsServices,omitempty"`
	ProgrammingLanguages   []string `json:"programmingLanguages,omitempty"`
	EstimatedReadingTime   string   `json:"estimatedReadingTime"`
	EngagementScore        int      `json:"engagementScore"`        // 0–100
	ProfessionalConfidence int      `json:"professionalConfidence"` // 0–100
	SEOKeywords            []string `json:"seoKeywords,omitempty"`
	SuggestedPublishTime   string   `json:"suggestedPublishTime"`
	CharacterCount         int      `json:"characterCount"`
}

// Intelligence is collection-level marketing metadata.
type Intelligence struct {
	PostCount                     int      `json:"postCount"`
	TargetAudiences               []string `json:"targetAudiences,omitempty"`
	Topics                        []string `json:"topics,omitempty"`
	TechnologyStack               []string `json:"technologyStack,omitempty"`
	AWSServices                   []string `json:"awsServices,omitempty"`
	ProgrammingLanguages          []string `json:"programmingLanguages,omitempty"`
	AverageEngagementScore        int      `json:"averageEngagementScore"`
	AverageProfessionalConfidence int      `json:"averageProfessionalConfidence"`
	SEOKeywords                   []string `json:"seoKeywords,omitempty"`
	Tone                          string   `json:"tone"`
	PublishCadence                string   `json:"publishCadence"`
	ProductionNotes               []string `json:"productionNotes,omitempty"`
}
