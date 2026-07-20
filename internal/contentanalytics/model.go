// Package contentanalytics is the Content Analytics & Performance engine
// (Milestone 17). It continuously measures how published content performs across
// every supported platform (Dev.to, Medium, Hashnode, YouTube, and future
// social platforms), normalizes platform-specific metrics into one unified
// analytics model, stores immutable daily/weekly/monthly historical snapshots,
// detects trends, generates structured reports and dashboard datasets, and emits
// custom metrics to CloudWatch.
//
// It follows Clean Architecture and dependency inversion: the domain (this file)
// is pure; the collector, trend, report, dashboard, and optimization engines
// depend only on ports (AnalyticsProvider, Repository, MetricsPublisher, Clock,
// Sleeper, HTTPDoer); and each platform is an independent, interchangeable
// AnalyticsProvider adapter, so new platforms are added without touching existing
// analytics logic. Historical snapshots are immutable — performance metrics are
// never overwritten — so trend analysis always has an honest record.
//
// This milestone ONLY produces analytics. It performs no content optimization;
// it emits a structured OptimizationSignals bundle for Milestone 18 to consume.
package contentanalytics

import "time"

// SchemaVersion is the analytics document version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// Platform identifies a publishing platform whose analytics are collected. The
// values intentionally match internal/publishing so a Publication produced by
// the distribution layer maps 1:1 onto its analytics.
type Platform string

const (
	PlatformDevTo    Platform = "Dev.to"
	PlatformMedium   Platform = "Medium"
	PlatformHashnode Platform = "Hashnode"
	PlatformYouTube  Platform = "YouTube"
	PlatformGitHub   Platform = "GitHub"
	// Future platforms implement the same AnalyticsProvider — no core changes.
	PlatformLinkedIn Platform = "LinkedIn"
	PlatformX        Platform = "X"
	PlatformTikTok   Platform = "TikTok"
)

// ContentType identifies the kind of published artifact being measured.
type ContentType string

const (
	TypeBlog          ContentType = "Technical Blog"
	TypeArticle       ContentType = "Article"
	TypeYouTubeVideo  ContentType = "YouTube Video"
	TypeYouTubeShorts ContentType = "YouTube Shorts"
	TypeLinkedInPost  ContentType = "LinkedIn Post"
	TypeXThread       ContentType = "X Thread"
	TypeTikTokVideo   ContentType = "TikTok Video"
)

// Date is an ISO calendar date (YYYY-MM-DD) — the snapshot key granularity.
type Date = string

// Period is a historical-snapshot cadence. Snapshots are stored at all three
// cadences so trend analysis can operate at any resolution.
type Period string

const (
	PeriodDaily   Period = "daily"
	PeriodWeekly  Period = "weekly"
	PeriodMonthly Period = "monthly"
)

// Publication is one published artifact under measurement. It is the analytics
// mirror of a publishing.Publication: the ID and ContentID are shared, so
// distribution and analytics join without coupling the packages.
type Publication struct {
	ID          string      `json:"id"`
	ContentID   string      `json:"contentId"`
	Platform    Platform    `json:"platform"`
	ContentType ContentType `json:"contentType"`
	Title       string      `json:"title"`
	URL         string      `json:"url"`
	PlatformID  string      `json:"platformId,omitempty"` // platform-side id (video id, article id)
	Tags        []string    `json:"tags,omitempty"`
	Keywords    []string    `json:"keywords,omitempty"`
	Author      string      `json:"author,omitempty"`
	PublishedAt time.Time   `json:"publishedAt"`
	LastUpdated time.Time   `json:"lastUpdated,omitempty"`
	Status      string      `json:"status"`              // published | scheduled | draft | removed
	Scheduled   bool        `json:"scheduled,omitempty"` // scheduled vs already published
}

// Reach captures how many people the content reached.
type Reach struct {
	Views         int64 `json:"views"`
	Impressions   int64 `json:"impressions,omitempty"`
	UniqueViewers int64 `json:"uniqueViewers,omitempty"`
	AudienceReach int64 `json:"audienceReach,omitempty"`
}

// Engagement captures direct audience interactions.
type Engagement struct {
	Likes     int64 `json:"likes,omitempty"`
	Reactions int64 `json:"reactions,omitempty"`
	Comments  int64 `json:"comments,omitempty"`
	Replies   int64 `json:"replies,omitempty"`
	Shares    int64 `json:"shares,omitempty"`
	Reposts   int64 `json:"reposts,omitempty"`
	Saves     int64 `json:"saves,omitempty"`
	Bookmarks int64 `json:"bookmarks,omitempty"`
}

// Total sums every engagement interaction.
func (e Engagement) Total() int64 {
	return e.Likes + e.Reactions + e.Comments + e.Replies + e.Shares + e.Reposts + e.Saves + e.Bookmarks
}

// Watch captures video/watch-time performance (where a platform reports it).
type Watch struct {
	WatchTimeMinutes int64   `json:"watchTimeMinutes,omitempty"`
	AvgViewSeconds   float64 `json:"avgViewSeconds,omitempty"`
	RetentionPct     float64 `json:"retentionPct,omitempty"`
	CompletionRate   float64 `json:"completionRate,omitempty"`
}

// Click captures click-through performance.
type Click struct {
	CTR            float64 `json:"ctr,omitempty"`          // click-through rate (%)
	ThumbnailCTR   float64 `json:"thumbnailCtr,omitempty"` // %
	LinkClicks     int64   `json:"linkClicks,omitempty"`
	ExternalClicks int64   `json:"externalClicks,omitempty"`
}

// Growth captures audience growth attributable to the content.
type Growth struct {
	SubscribersGained int64 `json:"subscribersGained,omitempty"`
	FollowersGained   int64 `json:"followersGained,omitempty"`
	SubscribersLost   int64 `json:"subscribersLost,omitempty"`
	AudienceGrowth    int64 `json:"audienceGrowth,omitempty"`
}

// TrafficSource is where views came from (e.g. search, feed, external).
type TrafficSource struct {
	Source  string  `json:"source"`
	Views   int64   `json:"views"`
	Percent float64 `json:"percent"`
}

// MetricsSnapshot is one immutable, normalized measurement of a publication at a
// point in time and cadence. The same (publication, period, date) is written
// once and never overwritten.
type MetricsSnapshot struct {
	Platform       Platform        `json:"platform"`
	PublicationID  string          `json:"publicationId"`
	ContentID      string          `json:"contentId"`
	Period         Period          `json:"period"`
	Date           Date            `json:"date"`
	Reach          Reach           `json:"reach"`
	Engagement     Engagement      `json:"engagement"`
	Watch          Watch           `json:"watch,omitempty"`
	Click          Click           `json:"click,omitempty"`
	Growth         Growth          `json:"growth,omitempty"`
	TrafficSources []TrafficSource `json:"trafficSources,omitempty"`
	Partial        bool            `json:"partial,omitempty"` // some fields could not be collected
	CollectedAt    time.Time       `json:"collectedAt"`
}

// EngagementRate is total engagements / views as a percentage.
func (s MetricsSnapshot) EngagementRate() float64 {
	if s.Reach.Views == 0 {
		return 0
	}
	return round2(float64(s.Engagement.Total()) / float64(s.Reach.Views) * 100)
}

// Delta is an absolute + percentage change between two values.
type Delta struct {
	Absolute float64 `json:"absolute"`
	Percent  float64 `json:"percent"`
}

// GrowthTrend summarizes how a metric moved over the standard windows.
type GrowthTrend struct {
	Platform       Platform `json:"platform,omitempty"`
	Metric         string   `json:"metric"`
	Current        float64  `json:"current"`
	DayOverDay     Delta    `json:"dayOverDay"`
	WeekOverWeek   Delta    `json:"weekOverWeek"`
	MonthOverMonth Delta    `json:"monthOverMonth"`
	Direction      string   `json:"direction"` // up | down | flat
}

// ContentPerformance is one publication's ranked performance for a window.
type ContentPerformance struct {
	PublicationID  string      `json:"publicationId"`
	ContentID      string      `json:"contentId"`
	Platform       Platform    `json:"platform"`
	ContentType    ContentType `json:"contentType"`
	Title          string      `json:"title"`
	URL            string      `json:"url,omitempty"`
	Views          int64       `json:"views"`
	Engagements    int64       `json:"engagements"`
	EngagementRate float64     `json:"engagementRate"`
	CTR            float64     `json:"ctr,omitempty"`
	RetentionPct   float64     `json:"retentionPct,omitempty"`
	WatchMinutes   int64       `json:"watchTimeMinutes,omitempty"`
	Score          float64     `json:"score"` // 0–100 blended
	Rank           int         `json:"rank"`
}

// PlatformSummary aggregates a platform's performance across its publications.
type PlatformSummary struct {
	Platform          Platform `json:"platform"`
	Publications      int      `json:"publications"`
	Views             int64    `json:"views"`
	Impressions       int64    `json:"impressions,omitempty"`
	Engagements       int64    `json:"engagements"`
	EngagementRate    float64  `json:"engagementRate"`
	CTR               float64  `json:"ctr,omitempty"`
	WatchTimeMinutes  int64    `json:"watchTimeMinutes,omitempty"`
	AvgRetentionPct   float64  `json:"avgRetentionPct,omitempty"`
	SubscribersGained int64    `json:"subscribersGained,omitempty"`
}

// TagSignal / TopicSignal / PublishingTime surface aggregate patterns.
type TagSignal struct {
	Tag            string  `json:"tag"`
	Publications   int     `json:"publications"`
	Views          int64   `json:"views"`
	EngagementRate float64 `json:"engagementRate"`
}

type TopicSignal struct {
	Keyword        string  `json:"keyword"`
	Publications   int     `json:"publications"`
	Views          int64   `json:"views"`
	EngagementRate float64 `json:"engagementRate"`
}

type PublishingTime struct {
	Weekday        string  `json:"weekday"`
	Hour           int     `json:"hour"`
	Publications   int     `json:"publications"`
	AvgViews       float64 `json:"avgViews"`
	EngagementRate float64 `json:"engagementRate"`
}

// PlatformRecommendation is a grounded platform-level observation for M18.
type PlatformRecommendation struct {
	Platform Platform `json:"platform"`
	Note     string   `json:"note"`
	Evidence []string `json:"evidence,omitempty"`
}

// Report is the structured analytics report.
type Report struct {
	SchemaVersion       string               `json:"schemaVersion"`
	GeneratedAt         time.Time            `json:"generatedAt"`
	Window              string               `json:"window"` // e.g. "2026-07-01..2026-07-20"
	ExecutiveSummary    string               `json:"executiveSummary"`
	Platforms           []PlatformSummary    `json:"platformSummary"`
	ContentPerformance  []ContentPerformance `json:"contentPerformance"`
	EngagementBreakdown EngagementBreakdown  `json:"engagementBreakdown"`
	GrowthTrends        []GrowthTrend        `json:"growthTrends"`
	TopArticles         []ContentPerformance `json:"topArticles"`
	TopVideos           []ContentPerformance `json:"topVideos"`
	WorstPerforming     []ContentPerformance `json:"worstPerforming"`
	Recommendations     []string             `json:"recommendations"`
	Historical          []GrowthTrend        `json:"historicalComparisons"`
	DataQuality         DataQuality          `json:"dataQuality"`
}

// EngagementBreakdown decomposes total engagement by interaction type.
type EngagementBreakdown struct {
	Likes    int64 `json:"likes"`
	Comments int64 `json:"comments"`
	Shares   int64 `json:"shares"`
	Saves    int64 `json:"saves"`
	Other    int64 `json:"other"`
	Total    int64 `json:"total"`
}

// DataQuality reports how complete the collected analytics are.
type DataQuality struct {
	Publications      int      `json:"publications"`
	Snapshots         int      `json:"snapshots"`
	PartialSnapshots  int      `json:"partialSnapshots"`
	MissingPlatforms  []string `json:"missingPlatforms,omitempty"`
	CompletenessScore float64  `json:"completenessScore"` // 0–100
}

// DashboardSeries is one named time series for a dashboard.
type DashboardSeries struct {
	Name   string  `json:"name"`
	Unit   string  `json:"unit"`
	Points []Point `json:"points"`
}

// Point is a (date, value) sample.
type Point struct {
	Date  Date    `json:"date"`
	Value float64 `json:"value"`
}

// DashboardDataset is the dashboard-ready dataset (CloudWatch / Grafana / web).
type DashboardDataset struct {
	SchemaVersion string            `json:"schemaVersion"`
	GeneratedAt   time.Time         `json:"generatedAt"`
	Series        []DashboardSeries `json:"series"`
	Platforms     []PlatformSummary `json:"platforms"`
	Totals        Totals            `json:"totals"`
}

// Totals are headline dashboard numbers.
type Totals struct {
	Views          int64   `json:"views"`
	Engagements    int64   `json:"engagements"`
	EngagementRate float64 `json:"engagementRate"`
	Publications   int     `json:"publications"`
	WatchMinutes   int64   `json:"watchTimeMinutes,omitempty"`
}

// OptimizationSignals is the structured analytics bundle handed to Milestone 18
// (AI Content Optimization). It contains only grounded observations — this
// milestone performs no optimization.
type OptimizationSignals struct {
	SchemaVersion           string                   `json:"schemaVersion"`
	GeneratedAt             time.Time                `json:"generatedAt"`
	BestPublishingTimes     []PublishingTime         `json:"bestPublishingTimes"`
	BestTopics              []TopicSignal            `json:"bestPerformingTopics"`
	StrongestHashtags       []TagSignal              `json:"strongestHashtags"`
	HighestCTR              []ContentPerformance     `json:"highestCtr"`
	BestRetention           []ContentPerformance     `json:"bestAudienceRetention"`
	SuccessfulPatterns      []string                 `json:"successfulContentPatterns"`
	PlatformRecommendations []PlatformRecommendation `json:"platformRecommendations"`
}
