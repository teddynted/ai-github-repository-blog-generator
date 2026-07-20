// Package publishing is the content-distribution layer (Milestone 15). It
// automatically distributes APPROVED AI-generated content to multiple publishing
// platforms (GitHub, Dev.to, Medium, Hashnode, YouTube, …) with immediate or
// scheduled publication, configurable retries with exponential backoff, full
// publication tracking, and an audit trail.
//
// It follows Clean Architecture and dependency inversion: the domain (this file
// + status.go) is pure; the application engine orchestrates over ports
// (Publisher, Repository, Notifier, Clock, Sleeper, HTTPDoer); and each platform
// is an independent adapter implementing the common Publisher interface, so new
// platforms are added without touching existing logic. It NEVER publishes content
// that has not passed the Review & Approval Workflow — the engine refuses any
// content whose Approved flag is not set.
package publishing

import "time"

// SchemaVersion is the publishing document version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// Platform identifies a publishing destination.
type Platform string

const (
	PlatformGitHub   Platform = "GitHub"
	PlatformDevTo    Platform = "Dev.to"
	PlatformMedium   Platform = "Medium"
	PlatformHashnode Platform = "Hashnode"
	PlatformYouTube  Platform = "YouTube"
	// Future platforms implement the same Publisher interface — no core changes.
	PlatformLinkedIn   Platform = "LinkedIn"
	PlatformX          Platform = "X"
	PlatformReddit     Platform = "Reddit"
	PlatformDiscord    Platform = "Discord"
	PlatformTelegram   Platform = "Telegram"
	PlatformNewsletter Platform = "Newsletter"
)

// ContentType identifies the kind of asset being published.
type ContentType string

const (
	TypeBlog            ContentType = "Technical Blog"
	TypeDevToArticle    ContentType = "Dev.to Article"
	TypeMediumArticle   ContentType = "Medium Article"
	TypeHashnodeArticle ContentType = "Hashnode Article"
	TypeYouTubeVideo    ContentType = "YouTube Video"
	TypeYouTubeShorts   ContentType = "YouTube Shorts"
	TypeLinkedInPost    ContentType = "LinkedIn Post"
	TypeXThread         ContentType = "X Thread"
	TypeDiagram         ContentType = "Architecture Diagram"
	TypeThumbnail       ContentType = "Thumbnail"
	TypeImage           ContentType = "Supporting Image"
)

// Content is the approved asset to distribute. It is platform-neutral; the
// MetadataEngine adapts it per platform. It must be Approved to publish.
type Content struct {
	ID             string            `json:"id"`
	Type           ContentType       `json:"type"`
	Title          string            `json:"title"`
	Body           string            `json:"body"` // Markdown for articles
	Tags           []string          `json:"tags,omitempty"`
	CanonicalURL   string            `json:"canonicalUrl,omitempty"`
	CoverImage     string            `json:"coverImage,omitempty"`
	Description    string            `json:"description,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Assets         []string          `json:"assets,omitempty"` // referenced asset paths/URLs
	ReleaseVersion string            `json:"releaseVersion,omitempty"`
	Author         string            `json:"author,omitempty"`

	// Approval gate — set only after the Review & Approval Workflow (M14).
	Approved    bool   `json:"approved"`
	ApprovalRef string `json:"approvalRef,omitempty"`
}

// PlatformMetadata is the platform-optimized metadata produced for one target.
type PlatformMetadata struct {
	Platform     Platform          `json:"platform"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Body         string            `json:"body,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	CanonicalURL string            `json:"canonicalUrl,omitempty"`
	CoverImage   string            `json:"coverImage,omitempty"`
	Visibility   string            `json:"visibility"`      // public | unlisted | private | draft
	Extra        map[string]string `json:"extra,omitempty"` // platform-specific (category, chapters, publicationId, ...)
}

// ScheduleMode is how a publication is timed.
type ScheduleMode string

const (
	ScheduleImmediate ScheduleMode = "immediate"
	ScheduleAt        ScheduleMode = "scheduled"
	ScheduleDelay     ScheduleMode = "delayed"
	ScheduleRecurring ScheduleMode = "recurring"
)

// Schedule controls when a publication runs.
type Schedule struct {
	Mode      ScheduleMode   `json:"mode"`
	At        time.Time      `json:"at,omitempty"`
	Delay     time.Duration  `json:"delay,omitempty"`
	TimeZone  string         `json:"timeZone,omitempty"` // IANA name, e.g. "America/New_York"
	Window    *BusinessHours `json:"window,omitempty"`   // constrain to business hours
	RecurCron string         `json:"recurCron,omitempty"`
}

// BusinessHours constrains publication to a daily window (local to TimeZone).
type BusinessHours struct {
	StartHour int `json:"startHour"` // 0–23
	EndHour   int `json:"endHour"`   // 0–23, exclusive
}

// RetryPolicy configures retries for recoverable failures.
type RetryPolicy struct {
	MaxRetries int           `json:"maxRetries"`
	BaseDelay  time.Duration `json:"baseDelay"`
	MaxDelay   time.Duration `json:"maxDelay"`
	Multiplier float64       `json:"multiplier"`
}

// PublicationResult is what a Publisher returns on success.
type PublicationResult struct {
	Platform   Platform `json:"platform"`
	PlatformID string   `json:"platformId"`
	URL        string   `json:"url"`
	Raw        string   `json:"-"` // provider raw response (never persisted in reports)
}

// PublicationAttempt records one attempt at publishing.
type PublicationAttempt struct {
	Number      int           `json:"number"`
	StartedAt   time.Time     `json:"startedAt"`
	FinishedAt  time.Time     `json:"finishedAt"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	Recoverable bool          `json:"recoverable"`
	Duration    time.Duration `json:"durationMs"`
}

// Publication is the tracked record of distributing one content item to one
// platform. Every field makes the publication traceable.
type Publication struct {
	ID             string               `json:"id"`
	ContentID      string               `json:"contentId"`
	ContentType    ContentType          `json:"contentType"`
	Platform       Platform             `json:"platform"`
	Status         PublicationStatus    `json:"status"`
	PlatformID     string               `json:"platformId,omitempty"`
	URL            string               `json:"url,omitempty"`
	ScheduledFor   *time.Time           `json:"scheduledFor,omitempty"`
	PublishedAt    *time.Time           `json:"publishedAt,omitempty"`
	RetryCount     int                  `json:"retryCount"`
	DurationMs     int64                `json:"durationMs"`
	Errors         []string             `json:"errors,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
	ApprovalRef    string               `json:"approvalRef,omitempty"`
	ReleaseVersion string               `json:"releaseVersion,omitempty"`
	Author         string               `json:"author,omitempty"`
	Metadata       PlatformMetadata     `json:"metadata"`
	Attempts       []PublicationAttempt `json:"attempts,omitempty"`
	Audit          []AuditEntry         `json:"audit,omitempty"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

// AuditEntry is one immutable record of a publication state change.
type AuditEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Action    string            `json:"action"`
	From      PublicationStatus `json:"from,omitempty"`
	To        PublicationStatus `json:"to,omitempty"`
	Detail    string            `json:"detail,omitempty"`
}
