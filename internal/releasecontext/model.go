// Package releasecontext is the Content Intelligence core of the platform. It
// analyzes a GitHub repository together with a selected GitHub Release and
// assembles a single, structured Release Context — the canonical, AI-ready
// source of truth that downstream milestones (blog generation, release
// summaries, social scripts, SEO metadata, …) consume.
//
// The package is pure domain logic: it depends on a Sources port for raw inputs
// (repository metadata, the release, commits, changed files, file contents) and
// on nothing else — no AWS, no network, no filesystem. That keeps every
// analyzer independently unit-testable and the whole engine reusable from a
// Lambda, a CLI, or a test.
package releasecontext

// SchemaVersion is the version of the Release Context document. It follows
// semantic versioning; the schema is additive-only, so consumers pinned to a
// major version keep working as later milestones add fields.
const SchemaVersion = "2.0.0"

// ReleaseContext is the complete, structured analysis of one repository at one
// release. It is the single input contract for every downstream AI workflow.
type ReleaseContext struct {
	SchemaVersion string `json:"schemaVersion"`
	// ContextID uniquely identifies this built context (UUIDv4).
	ContextID string `json:"contextId"`
	// GeneratedAt is the RFC3339 timestamp the context was built.
	GeneratedAt string `json:"generatedAt"`

	Repository          Repository             `json:"repository"`
	Release             Release                `json:"release"`
	Documentation       Documentation          `json:"documentation"`
	RepositoryStructure RepositoryStructure    `json:"repositoryStructure"`
	Architecture        Architecture           `json:"architecture"`
	Commits             []Commit               `json:"commits"`
	CommitStats         CommitStats            `json:"commitStats"`
	ChangedFiles        []ChangedFile          `json:"changedFiles"`
	FileStats           FileStats              `json:"fileStats"`
	Technologies        []Technology           `json:"technologies"`
	CloudFormation      CloudFormationAnalysis `json:"cloudformation"`
	Mermaid             []MermaidDiagram       `json:"mermaid"`
	Changelog           ChangelogAnalysis      `json:"changelog"`
	Implementation      ImplementationSummary  `json:"implementation"`
	ContentIntelligence ContentIntelligence    `json:"contentIntelligence"`
	// Engineering is the structured engineering analysis (Stage 2 of the content
	// pipeline). It is populated by the engineering analyzer AFTER the factual context
	// is built, and consumed by the Claude technical-writer stage. Nil when the
	// analysis stage is disabled or failed, in which case generation falls back
	// to the factual context alone. See internal/engineeringanalysis.
	Engineering *EngineeringContext `json:"engineering,omitempty"`
	// Warnings collects non-fatal analyzer notices (e.g. a missing CHANGELOG),
	// so a partial context is still useful and the gaps are explicit.
	Warnings []string `json:"warnings,omitempty"`
}

// Repository is the GitHub repository metadata and a derived summary.
type Repository struct {
	Owner         string   `json:"owner"`
	Name          string   `json:"name"`
	FullName      string   `json:"fullName"`
	Description   string   `json:"description,omitempty"`
	Topics        []string `json:"topics,omitempty"`
	Homepage      string   `json:"homepage,omitempty"`
	License       string   `json:"license,omitempty"`
	Visibility    string   `json:"visibility,omitempty"`
	DefaultBranch string   `json:"defaultBranch,omitempty"`
	Language      string   `json:"language,omitempty"`
	URL           string   `json:"url,omitempty"`
	Summary       string   `json:"summary,omitempty"`
}

// Release is the selected GitHub Release plus a derived summary.
type Release struct {
	Tag         string         `json:"tag"`
	Name        string         `json:"name,omitempty"`
	PublishedAt string         `json:"publishedAt,omitempty"`
	Author      string         `json:"author,omitempty"`
	Body        string         `json:"body,omitempty"`
	URL         string         `json:"url,omitempty"`
	PreRelease  bool           `json:"preRelease"`
	PreviousTag string         `json:"previousTag,omitempty"`
	Assets      []ReleaseAsset `json:"assets,omitempty"`
	Summary     string         `json:"summary,omitempty"`
}

// ReleaseAsset is a downloadable artifact attached to a release.
type ReleaseAsset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	URL         string `json:"url,omitempty"`
}

// Documentation is the structured reading of the project's docs.
type Documentation struct {
	Purpose              string        `json:"purpose,omitempty"`
	BusinessProblem      string        `json:"businessProblem,omitempty"`
	TechnicalGoals       []string      `json:"technicalGoals,omitempty"`
	DesignDecisions      []string      `json:"designDecisions,omitempty"`
	ArchitecturePatterns []string      `json:"architecturePatterns,omitempty"`
	DeploymentStrategy   string        `json:"deploymentStrategy,omitempty"`
	DevelopmentWorkflow  string        `json:"developmentWorkflow,omitempty"`
	Documents            []DocumentRef `json:"documents,omitempty"`
}

// DocumentRef is a single analyzed documentation file.
type DocumentRef struct {
	Path     string   `json:"path"`
	Title    string   `json:"title,omitempty"`
	Kind     string   `json:"kind"` // readme | architecture | adr | deployment | guide | other
	Headings []string `json:"headings,omitempty"`
}

// RepositoryStructure describes how the project is organized.
type RepositoryStructure struct {
	Directories []DirectoryInfo `json:"directories"`
	Overview    string          `json:"overview,omitempty"`
	FileCount   int             `json:"fileCount"`
	Layout      string          `json:"layout,omitempty"` // e.g. "standard-go", "monorepo"
}

// DirectoryInfo names a top-level directory and its inferred responsibility.
type DirectoryInfo struct {
	Path           string `json:"path"`
	Responsibility string `json:"responsibility"`
	FileCount      int    `json:"fileCount"`
}

// Architecture is a synthesized architectural overview for technical content.
type Architecture struct {
	Overview           string                  `json:"overview,omitempty"`
	Components         []ArchitectureComponent `json:"components,omitempty"`
	DeploymentTopology string                  `json:"deploymentTopology,omitempty"`
	AWSServices        []string                `json:"awsServices,omitempty"`
	EventDrivenFlows   []string                `json:"eventDrivenFlows,omitempty"`
	Scalability        string                  `json:"scalability,omitempty"`
	Reliability        string                  `json:"reliability,omitempty"`
	Security           string                  `json:"security,omitempty"`
	Insights           []string                `json:"insights,omitempty"`
}

// ArchitectureComponent is one major component and its responsibility.
type ArchitectureComponent struct {
	Name           string `json:"name"`
	Responsibility string `json:"responsibility"`
	Kind           string `json:"kind,omitempty"` // lambda | api | queue | store | compute | ...
}

// Commit is a categorized commit between the previous and selected release.
type Commit struct {
	SHA          string `json:"sha"`
	Subject      string `json:"subject"`
	Author       string `json:"author,omitempty"`
	Date         string `json:"date,omitempty"`
	Type         string `json:"type"` // feat | fix | docs | refactor | ...
	Scope        string `json:"scope,omitempty"`
	Category     string `json:"category"` // Features | Bug Fixes | Documentation | Infrastructure | ...
	Breaking     bool   `json:"breaking"`
	Conventional bool   `json:"conventional"`
}

// CommitStats summarizes the analyzed commit range.
type CommitStats struct {
	Total        int            `json:"total"`
	Analyzed     int            `json:"analyzed"` // excludes merges / bumps / format-only
	Ignored      int            `json:"ignored"`
	Conventional int            `json:"conventional"`
	Breaking     int            `json:"breaking"`
	ByCategory   map[string]int `json:"byCategory"`
	Contributors []string       `json:"contributors,omitempty"`
}

// ChangedFile is one file modified for the release, with a category.
type ChangedFile struct {
	Path      string `json:"path"`
	Category  string `json:"category"`         // Infrastructure | Documentation | Application Code | ...
	Status    string `json:"status,omitempty"` // added | modified | removed | renamed
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
}

// FileStats summarizes changed files.
type FileStats struct {
	Total      int            `json:"total"`
	Additions  int            `json:"additions"`
	Deletions  int            `json:"deletions"`
	ByCategory map[string]int `json:"byCategory"`
	Summary    string         `json:"summary,omitempty"`
}

// Technology is one detected technology in the inventory.
type Technology struct {
	Name       string `json:"name"`
	Category   string `json:"category"`   // AWS Service | Language | Framework | Library | Tool | Deployment
	Confidence string `json:"confidence"` // high | medium | low
	Evidence   string `json:"evidence,omitempty"`
}

// CloudFormationAnalysis is the infrastructure summary from CFN templates.
type CloudFormationAnalysis struct {
	Templates  []string      `json:"templates,omitempty"`
	Resources  []CFNResource `json:"resources,omitempty"`
	Parameters []string      `json:"parameters,omitempty"`
	Outputs    []string      `json:"outputs,omitempty"`
	Counts     CFNCounts     `json:"counts"`
	Services   []string      `json:"services,omitempty"`
	Summary    string        `json:"summary,omitempty"`
}

// CFNResource is one resource parsed from a template.
type CFNResource struct {
	LogicalID string `json:"logicalId"`
	Type      string `json:"type"`
	Service   string `json:"service,omitempty"`
	Template  string `json:"template,omitempty"`
	Category  string `json:"category,omitempty"` // Compute | Storage | Serverless | Networking | IAM | Messaging | Other
}

// CFNCounts tallies resource categories across all templates.
type CFNCounts struct {
	Resources  int `json:"resources"`
	Parameters int `json:"parameters"`
	Outputs    int `json:"outputs"`
	IAM        int `json:"iam"`
	Serverless int `json:"serverless"`
	Compute    int `json:"compute"`
	Storage    int `json:"storage"`
	Networking int `json:"networking"`
	Messaging  int `json:"messaging"`
}

// MermaidDiagram is one parsed Mermaid diagram.
type MermaidDiagram struct {
	Source string   `json:"source,omitempty"` // originating doc path
	Type   string   `json:"type"`             // flowchart | sequence | state | class | er | other
	Nodes  []string `json:"nodes,omitempty"`
	// NodeLabels maps a node id to its human label when the diagram defines one
	// (e.g. "GH" → "GitHub Release"), so consumers can show descriptive names
	// instead of cryptic ids. Absent when a node has no label.
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`
	Edges      []MermaidEdge     `json:"edges,omitempty"`
	NodeCount  int               `json:"nodeCount"`
	EdgeCount  int               `json:"edgeCount"`
	Summary    string            `json:"summary,omitempty"`
}

// MermaidEdge is a directed relationship between two nodes.
type MermaidEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// ChangelogAnalysis is the parsed CHANGELOG section for the release.
type ChangelogAnalysis struct {
	Version         string   `json:"version,omitempty"`
	Date            string   `json:"date,omitempty"`
	Features        []string `json:"features,omitempty"`
	Improvements    []string `json:"improvements,omitempty"`
	BugFixes        []string `json:"bugFixes,omitempty"`
	BreakingChanges []string `json:"breakingChanges,omitempty"`
	Deprecations    []string `json:"deprecations,omitempty"`
	MigrationNotes  []string `json:"migrationNotes,omitempty"`
	Found           bool     `json:"found"`
}

// ImplementationSummary explains what shipped and why it matters.
type ImplementationSummary struct {
	WhatChanged                []string `json:"whatChanged,omitempty"`
	HowItWorks                 string   `json:"howItWorks,omitempty"`
	TechnicalImprovements      []string `json:"technicalImprovements,omitempty"`
	InfrastructureImprovements []string `json:"infrastructureImprovements,omitempty"`
	DeveloperExperience        []string `json:"developerExperience,omitempty"`
	DocumentationImprovements  []string `json:"documentationImprovements,omitempty"`
	WhyItMatters               string   `json:"whyItMatters,omitempty"`
}

// ContentIntelligence is the AI-ready metadata layer.
type ContentIntelligence struct {
	Summary                  string   `json:"summary"`
	TechnicalHighlights      []string `json:"technicalHighlights,omitempty"`
	InfrastructureHighlights []string `json:"infrastructureHighlights,omitempty"`
	ArchitectureHighlights   []string `json:"architectureHighlights,omitempty"`
	ImplementationComplexity string   `json:"implementationComplexity,omitempty"` // low | medium | high
	DeveloperValue           string   `json:"developerValue,omitempty"`
	BusinessValue            string   `json:"businessValue,omitempty"`
	TargetAudience           string   `json:"targetAudience,omitempty"`
	SEOKeywords              []string `json:"seoKeywords,omitempty"`
	BlogTitles               []string `json:"blogTitles,omitempty"`
	ArticleOutline           []string `json:"articleOutline,omitempty"`
	LinkedInPost             string   `json:"linkedInPost,omitempty"`
	YouTubeShortsTopic       string   `json:"youTubeShortsTopic,omitempty"`
	TikTokTopic              string   `json:"tikTokTopic,omitempty"`
	DocumentationUpdates     []string `json:"documentationUpdates,omitempty"`
	FutureEnhancements       []string `json:"futureEnhancements,omitempty"`
}
