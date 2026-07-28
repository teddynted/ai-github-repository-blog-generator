// Package architecture is the Architecture Visualization engine (Milestone 11).
// It converts the Release Context (M2) and downstream artifacts (Blog M3,
// Storyboard M4) into production-ready architecture diagrams — Mermaid, Graphviz
// DOT, and SVG — plus PNG export metadata, for blogs, docs, videos, and slides.
//
// It focuses on visualization, not provisioning, and it NEVER invents
// infrastructure: nodes come only from the AWS services, parsed Mermaid,
// CloudFormation resources, components, and repository structure in the Release
// Context, and edges are only drawn when explicitly present (parsed Mermaid
// edges, event-driven flows, or factual containment). The engine is built as
// analysis → a shared graph model → deterministic renderers, so every output
// format stays synchronized with the same grounded graph. The shared
// releasegen.Model port is used only to phrase titles/descriptions and suggest
// layout, with deterministic fallbacks so nothing is fabricated.
package architecture

// SchemaVersion is the architecture document version (SemVer, additive-only) so
// future visualization engines extend it without breaking.
const SchemaVersion = "1.0.0"

// ArchitectureCollection is the full set of diagrams for one release.
type ArchitectureCollection struct {
	SchemaVersion string   `json:"schemaVersion"`
	Metadata      Metadata `json:"metadata"`
	// PlatformOverview is a grounded, high-level prose summary of the platform,
	// rendered before the diagrams. Empty when the context has no overview.
	PlatformOverview    string       `json:"platformOverview,omitempty"`
	Diagrams            []Diagram    `json:"diagrams"`
	ContentIntelligence Intelligence `json:"contentIntelligence"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Metadata identifies the source of the diagrams.
type Metadata struct {
	Repository    string            `json:"repository"`
	Release       string            `json:"release"`
	GeneratedAt   string            `json:"generatedAt"`
	DiagramCount  int               `json:"diagramCount"`
	SourceSchemas map[string]string `json:"sourceSchemas,omitempty"`
}

// Diagram is one architecture visualization rendered in multiple formats.
type Diagram struct {
	ID          int         `json:"id"`
	Title       string      `json:"title"`
	Subtitle    string      `json:"subtitle,omitempty"`
	Description string      `json:"description"`
	Type        string      `json:"type"`        // High-Level Architecture | Event-Driven Architecture | Data Flow Diagram | Sequence Diagram | Component Diagram | CI/CD Pipeline | Infrastructure Topology
	MermaidType string      `json:"mermaidType"` // flowchart | sequenceDiagram | ...
	Mermaid     string      `json:"mermaid"`
	Graphviz    string      `json:"graphviz"`
	SVG         string      `json:"svg"`
	PNG         PNGExport   `json:"png"`
	AWSServices []string    `json:"awsServices,omitempty"`
	References  []string    `json:"references,omitempty"`
	Metadata    DiagramMeta `json:"metadata"`
}

// PNGExport carries the settings a rasterizer needs to export a PNG.
type PNGExport struct {
	RecommendedResolution string `json:"recommendedResolution"` // e.g. "1920x1080"
	AspectRatio           string `json:"aspectRatio"`
	CanvasWidth           int    `json:"canvasWidth"`
	CanvasHeight          int    `json:"canvasHeight"`
	DPI                   int    `json:"dpi"`
	Background            string `json:"background"`
	ExportSettings        string `json:"exportSettings"`
}

// DiagramMeta is per-diagram metadata.
type DiagramMeta struct {
	Complexity          string   `json:"complexity"` // low | medium | high
	Audience            string   `json:"audience"`
	ReleaseVersion      string   `json:"releaseVersion"`
	GeneratedAt         string   `json:"generatedAt"`
	ReferencedFiles     []string `json:"referencedFiles,omitempty"`
	ReferencedResources []string `json:"referencedResources,omitempty"`
	NodeCount           int      `json:"nodeCount"`
	EdgeCount           int      `json:"edgeCount"`
	Confidence          int      `json:"confidence"` // 0–100
}

// Intelligence is collection-level architecture metadata.
type Intelligence struct {
	ArchitectureStyle        string `json:"architectureStyle"`
	DeploymentPattern        string `json:"deploymentPattern"`
	InfrastructureComplexity string `json:"infrastructureComplexity"`
	// Hybrid AI signals — populated only when the context grounds both local and
	// cloud inference; empty otherwise, so non-AI repositories never get AI framing.
	LocalInference          []string `json:"localInference,omitempty"`
	CloudInference          []string `json:"cloudInference,omitempty"`
	PrimaryWorkflow         string   `json:"primaryWorkflow,omitempty"`
	OperationalModel        string   `json:"operationalModel,omitempty"`
	CloudServices           []string `json:"cloudServices,omitempty"`
	ComputeComponents       []string `json:"computeComponents,omitempty"`
	ServerlessComponents    []string `json:"serverlessComponents,omitempty"`
	StorageComponents       []string `json:"storageComponents,omitempty"`
	DatabaseComponents      []string `json:"databaseComponents,omitempty"`
	MessagingComponents     []string `json:"messagingComponents,omitempty"`
	NetworkingComponents    []string `json:"networkingComponents,omitempty"`
	SecurityComponents      []string `json:"securityComponents,omitempty"`
	IntegrationServices     []string `json:"integrationServices,omitempty"`
	ObservabilityComponents []string `json:"observabilityComponents,omitempty"`
	EstimatedReadingTime    string   `json:"estimatedReadingTime"`
	DiagramConfidence       int      `json:"diagramConfidence"` // 0–100
}
