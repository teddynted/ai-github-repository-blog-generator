// Package socialintel is the Social Media Intelligence platform (Milestone 16).
// It collects daily performance data from social platforms (YouTube, Instagram,
// X, TikTok), stores immutable historical snapshots, computes trends and content
// effectiveness, analyzes subscriber conversion, compares platforms, and
// generates grounded AI insights, a morning briefing, and structured
// intelligence for the Business Advisory Council.
//
// It follows Clean Architecture and dependency inversion: the domain (this file)
// is pure; the collector, analytics, insights, briefing, and advisory engines
// depend only on ports (Provider, Repository, AIInsighter, Monitor, Clock,
// HTTPDoer); and each platform is an independent, interchangeable Provider
// adapter, so new platforms are added without touching existing logic. Snapshots
// are immutable — historical metrics are never overwritten — and every
// AI-generated insight is grounded in the collected metrics: statistics are
// never fabricated.
package socialintel

import "time"

// SchemaVersion is the intelligence document version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// Platform identifies a social platform.
type Platform string

const (
	PlatformYouTube   Platform = "YouTube"
	PlatformInstagram Platform = "Instagram"
	PlatformX         Platform = "X"
	PlatformTikTok    Platform = "TikTok"
	// Future providers implement the same Provider interface — no core changes.
	PlatformFacebook  Platform = "Facebook"
	PlatformLinkedIn  Platform = "LinkedIn"
	PlatformThreads   Platform = "Threads"
	PlatformReddit    Platform = "Reddit"
	PlatformPinterest Platform = "Pinterest"
	PlatformBluesky   Platform = "Bluesky"
)

// Date is an ISO calendar date (YYYY-MM-DD) — the snapshot key granularity.
type Date = string

// AccountSnapshot is one immutable daily snapshot of an account's metrics.
// Platform-neutral fields cover the common cases; Extra carries platform
// specifics (e.g. impressions, saves, revenue).
type AccountSnapshot struct {
	Platform         Platform           `json:"platform"`
	Date             Date               `json:"date"`
	Followers        int64              `json:"followers"` // subscribers / followers / fans
	Following        int64              `json:"following,omitempty"`
	Views            int64              `json:"views,omitempty"`
	Impressions      int64              `json:"impressions,omitempty"`
	Reach            int64              `json:"reach,omitempty"`
	ProfileVisits    int64              `json:"profileVisits,omitempty"`
	WatchTimeMinutes int64              `json:"watchTimeMinutes,omitempty"`
	Extra            map[string]float64 `json:"extra,omitempty"`
	CollectedAt      time.Time          `json:"collectedAt"`
}

// ContentKind distinguishes the content forms across platforms.
type ContentKind string

const (
	KindVideo  ContentKind = "video"
	KindShort  ContentKind = "short"
	KindReel   ContentKind = "reel"
	KindPost   ContentKind = "post"
	KindThread ContentKind = "thread"
)

// ContentSnapshot is one immutable daily snapshot of a piece of content. It
// unifies YouTube videos, Instagram posts/reels, X posts, and TikTok videos.
type ContentSnapshot struct {
	Platform          Platform           `json:"platform"`
	ContentID         string             `json:"contentId"`
	Kind              ContentKind        `json:"kind"`
	Title             string             `json:"title,omitempty"`
	PublishedAt       Date               `json:"publishedAt,omitempty"`
	Date              Date               `json:"date"` // snapshot date
	Views             int64              `json:"views,omitempty"`
	Likes             int64              `json:"likes,omitempty"`
	Comments          int64              `json:"comments,omitempty"`
	Shares            int64              `json:"shares,omitempty"`
	Saves             int64              `json:"saves,omitempty"`
	Impressions       int64              `json:"impressions,omitempty"`
	CTR               float64            `json:"ctr,omitempty"`
	AvgViewSeconds    float64            `json:"avgViewSeconds,omitempty"`
	RetentionPct      float64            `json:"retentionPct,omitempty"`
	CompletionRate    float64            `json:"completionRate,omitempty"`
	WatchTimeMinutes  int64              `json:"watchTimeMinutes,omitempty"`
	SubscribersGained int64              `json:"subscribersGained,omitempty"`
	SubscribersLost   int64              `json:"subscribersLost,omitempty"`
	Tags              []string           `json:"tags,omitempty"`
	Extra             map[string]float64 `json:"extra,omitempty"`
	CollectedAt       time.Time          `json:"collectedAt"`
}

// Point is one (date, value) sample in a historical series.
type Point struct {
	Date  Date    `json:"date"`
	Value float64 `json:"value"`
}

// HistoricalSeries is an ordered metric time series.
type HistoricalSeries struct {
	Metric string  `json:"metric"`
	Points []Point `json:"points"`
}

// GrowthMetrics captures growth over standard windows.
type GrowthMetrics struct {
	Platform       Platform `json:"platform"`
	Metric         string   `json:"metric"`
	Current        float64  `json:"current"`
	DayOverDay     Delta    `json:"dayOverDay"`
	WeekOverWeek   Delta    `json:"weekOverWeek"`
	MonthOverMonth Delta    `json:"monthOverMonth"`
	Rolling7Avg    float64  `json:"rolling7Avg"`
	Velocity       float64  `json:"velocity"` // avg daily change over the series
	Momentum       string   `json:"momentum"` // accelerating | steady | decelerating | flat
}

// Delta is an absolute + percentage change.
type Delta struct {
	Absolute float64 `json:"absolute"`
	Percent  float64 `json:"percent"`
}

// Trend summarizes the direction of a series.
type Trend struct {
	Metric    string  `json:"metric"`
	Direction string  `json:"direction"` // up | down | flat
	ChangePct float64 `json:"changePct"`
	Series    []Point `json:"series,omitempty"`
}

// SubscriberConversion measures how effectively a video acquires subscribers.
type SubscriberConversion struct {
	ContentID         string  `json:"contentId"`
	Title             string  `json:"title,omitempty"`
	Views             int64   `json:"views"`
	SubscribersGained int64   `json:"subscribersGained"`
	SubscribersLost   int64   `json:"subscribersLost"`
	NetSubscribers    int64   `json:"netSubscribers"`
	ConversionRate    float64 `json:"conversionRate"` // net subs / views (%)
	ViewsPerSub       float64 `json:"viewsPerSubscriber"`
	Rank              int     `json:"rank"`
}

// ContentEffectiveness ranks a piece of content across quality dimensions.
type ContentEffectiveness struct {
	ContentID      string      `json:"contentId"`
	Platform       Platform    `json:"platform"`
	Kind           ContentKind `json:"kind"`
	Title          string      `json:"title,omitempty"`
	Views          int64       `json:"views"`
	EngagementRate float64     `json:"engagementRate"` // (likes+comments+shares+saves)/views (%)
	ViralityScore  float64     `json:"viralityScore"`  // 0–100
	EvergreenScore float64     `json:"evergreenScore"` // 0–100 (sustained views)
	RetentionPct   float64     `json:"retentionPct,omitempty"`
	Score          float64     `json:"score"` // 0–100 overall
	Rank           int         `json:"rank"`
}

// PlatformComparison is one platform's cross-platform standing.
type PlatformComparison struct {
	Platform          Platform `json:"platform"`
	Followers         int64    `json:"followers"`
	FollowerGrowthPct float64  `json:"followerGrowthPct"`
	EngagementRate    float64  `json:"engagementRate"`
	ConversionRate    float64  `json:"conversionRate,omitempty"`
	AvgRetentionPct   float64  `json:"avgRetentionPct,omitempty"`
	PublishingFreq    float64  `json:"publishingFrequency"` // content/day over the window
}

// CrossPlatform is the comparative intelligence across platforms.
type CrossPlatform struct {
	Platforms         []PlatformComparison `json:"platforms"`
	BestPlatform      Platform             `json:"bestPlatform"`
	FastestGrowing    Platform             `json:"fastestGrowing"`
	HighestEngagement Platform             `json:"highestEngagement"`
	HighestConversion Platform             `json:"highestConversion,omitempty"`
	HighestRetention  Platform             `json:"highestRetention,omitempty"`
}

// Recommendation is a grounded, actionable suggestion.
type Recommendation struct {
	Category  string   `json:"category"` // content | posting | audience | growth | risk
	Action    string   `json:"action"`
	Rationale string   `json:"rationale"`
	Evidence  []string `json:"evidence,omitempty"` // the metrics that ground it
}

// BusinessInsight is a qualitative finding grounded in metrics.
type BusinessInsight struct {
	Kind    string `json:"kind"` // strength | weakness | opportunity | risk | prediction
	Summary string `json:"summary"`
	Metric  string `json:"metric,omitempty"`
}

// MorningBriefing is the daily executive summary.
type MorningBriefing struct {
	SchemaVersion     string                 `json:"schemaVersion"`
	Date              Date                   `json:"date"`
	GeneratedAt       time.Time              `json:"generatedAt"`
	Headline          string                 `json:"headline"`
	Platforms         []PlatformPerformance  `json:"platforms"`
	NewFollowers      int64                  `json:"newFollowers"`
	BestContent       []ContentEffectiveness `json:"bestContent,omitempty"`
	WorstContent      []ContentEffectiveness `json:"worstContent,omitempty"`
	TopConversion     []SubscriberConversion `json:"topConversion,omitempty"`
	CrossPlatform     CrossPlatform          `json:"crossPlatform"`
	TrendingTopics    []string               `json:"trendingTopics,omitempty"`
	Insights          []BusinessInsight      `json:"insights,omitempty"`
	Recommendations   []Recommendation       `json:"recommendations,omitempty"`
	Alerts            []string               `json:"alerts,omitempty"`
	PublishingWindows []string               `json:"publishingOpportunities,omitempty"`
}

// PlatformPerformance is one platform's yesterday-vs-history line in the briefing.
type PlatformPerformance struct {
	Platform       Platform `json:"platform"`
	Followers      int64    `json:"followers"`
	FollowerDelta  int64    `json:"followerDelta"`
	Views          int64    `json:"views,omitempty"`
	ViewsDelta     int64    `json:"viewsDelta,omitempty"`
	EngagementRate float64  `json:"engagementRate"`
}

// AdvisoryPackage is the structured intelligence handed to the Business Advisory
// Council (delivered as JSON).
type AdvisoryPackage struct {
	SchemaVersion        string                 `json:"schemaVersion"`
	Date                 Date                   `json:"date"`
	YesterdayPerformance []PlatformPerformance  `json:"yesterdayPerformance"`
	HistoricalComparison []GrowthMetrics        `json:"historicalComparison"`
	GrowthTrends         []Trend                `json:"growthTrends"`
	ContentEffectiveness []ContentEffectiveness `json:"contentEffectiveness"`
	CrossPlatform        CrossPlatform          `json:"crossPlatform"`
	Recommendations      []Recommendation       `json:"recommendations"`
	StrategicActions     []string               `json:"strategicActions"`
}
