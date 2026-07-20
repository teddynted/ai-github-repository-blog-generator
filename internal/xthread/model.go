// Package xthread is the Social Marketing engine (Milestone 13). It converts the
// content pipeline's artifacts — Release Context (M2) through LinkedIn (M12) —
// into technical X (Twitter) threads: multiple thread variations targeting
// developer, cloud, AI, and DevOps audiences.
//
// It generates structured thread packages, not published threads. It CONSUMES
// the upstream artifacts and never regenerates repository knowledge, and it never
// invents features, AWS services, metrics, or performance claims: every thread is
// grounded in the Release Context, and code snippets come only from the blog's
// real fenced blocks. It is deterministic where it matters — thread discovery,
// post composition, character-limit enforcement, code extraction, takeaways,
// engagement, CTAs, hashtags, metadata, and validation are rule-based and
// independently testable. The shared releasegen.Model port is used only to
// sharpen the opening hook, with a deterministic fallback so nothing is
// fabricated.
package xthread

// SchemaVersion is the X-thread document version (SemVer, additive-only) so
// future publishing automation extends it without breaking.
const SchemaVersion = "1.0.0"

// MaxPostChars is X's per-post character limit.
const MaxPostChars = 280

// XThreadCollection is the full set of threads for one release.
type XThreadCollection struct {
	SchemaVersion       string       `json:"schemaVersion"`
	Metadata            Metadata     `json:"metadata"`
	Threads             []Thread     `json:"threads"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source artifacts the threads were built from.
type Metadata struct {
	Repository      string            `json:"repository"`
	Release         string            `json:"release"`
	SourceBlogTitle string            `json:"sourceBlogTitle"`
	GeneratedAt     string            `json:"generatedAt"`
	ThreadCount     int               `json:"threadCount"`
	PostsPerThread  int               `json:"postsPerThread"`
	SourceSchemas   map[string]string `json:"sourceSchemas,omitempty"`
}

// Thread is one X thread variation.
type Thread struct {
	ID               int          `json:"id"`
	Type             string       `json:"type"` // Release Announcement | Feature Breakdown | Architecture Walkthrough | Implementation Deep Dive | Engineering Lessons Learned | Performance Improvements | AWS Best Practices | AI Engineering Insights | Developer Tips | Open Source Update | Technical Summary | Repository Highlights
	Title            string       `json:"title"`
	Audience         string       `json:"audience"`
	Length           int          `json:"length"` // number of posts
	Posts            []ThreadPost `json:"posts"`
	Summary          string       `json:"summary"`
	KeyTakeaways     []string     `json:"keyTakeaways"`
	EngagementPrompt string       `json:"engagementPrompt"`
	CTA              string       `json:"cta"`
	Hashtags         []string     `json:"hashtags"`
	VisualReferences []VisualRef  `json:"visualReferences,omitempty"`
	Metadata         ThreadMeta   `json:"metadata"`
}

// ThreadPost is one post within a thread.
type ThreadPost struct {
	Index           int    `json:"index"`
	Content         string `json:"content"`
	CodeSnippet     string `json:"codeSnippet,omitempty"`
	VisualReference string `json:"visualReference,omitempty"`
	CharacterCount  int    `json:"characterCount"`
}

// VisualRef points at a previously generated visual asset (never regenerated).
type VisualRef struct {
	Type      string `json:"type"`
	Reference string `json:"reference"`
	Source    string `json:"source"`
}

// ThreadMeta is per-thread metadata.
type ThreadMeta struct {
	Topic                string   `json:"topic"`
	Difficulty           string   `json:"difficulty"`
	Tone                 string   `json:"tone"`
	TechnologyStack      []string `json:"technologyStack,omitempty"`
	AWSServices          []string `json:"awsServices,omitempty"`
	ProgrammingLanguages []string `json:"programmingLanguages,omitempty"`
	EstimatedReadingTime string   `json:"estimatedReadingTime"`
	EngagementScore      int      `json:"engagementScore"`     // 0–100
	TechnicalConfidence  int      `json:"technicalConfidence"` // 0–100
	SEOKeywords          []string `json:"seoKeywords,omitempty"`
	SuggestedPostingTime string   `json:"suggestedPostingTime"`
	TotalCharacters      int      `json:"totalCharacters"`
}

// Intelligence is collection-level marketing metadata.
type Intelligence struct {
	ThreadCount                int      `json:"threadCount"`
	TargetAudiences            []string `json:"targetAudiences,omitempty"`
	Topics                     []string `json:"topics,omitempty"`
	TechnologyStack            []string `json:"technologyStack,omitempty"`
	AWSServices                []string `json:"awsServices,omitempty"`
	ProgrammingLanguages       []string `json:"programmingLanguages,omitempty"`
	AverageEngagementScore     int      `json:"averageEngagementScore"`
	AverageTechnicalConfidence int      `json:"averageTechnicalConfidence"`
	SEOKeywords                []string `json:"seoKeywords,omitempty"`
	Tone                       string   `json:"tone"`
	PostingCadence             string   `json:"postingCadence"`
	ProductionNotes            []string `json:"productionNotes,omitempty"`
}
