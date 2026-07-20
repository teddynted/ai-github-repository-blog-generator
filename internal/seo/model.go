// Package seo is the canonical SEO metadata engine (Milestone 10). It converts
// the previously generated artifacts — Release Context (M2), Technical Blog
// (M3), Storyboard (M4), Voice-over (M5), YouTube Script (M6), YouTube Shorts
// (M7), TikTok (M8), and Visual Assets (M9) — into structured, platform-specific
// SEO metadata for blogs, videos, short-form, social, Open Graph, and structured
// data (JSON-LD / Schema.org / RSS / sitemap).
//
// It generates METADATA, not content: it aggregates and normalizes what the
// content pipeline already produced, enforces platform limits, dedupes, and
// emits structured data ready for a publishing system to consume without further
// transformation. It never regenerates repository knowledge and never invents
// features: every value is grounded in the Release Context and the generated
// artifacts. It is deterministic where it matters — slugs, excerpts, keyword
// taxonomy, hashtags, Open Graph, structured data, limits, and validation are
// rule-based and independently testable. The shared releasegen.Model port is
// used only to polish titles, descriptions, and excerpts, with a deterministic
// fallback so nothing is fabricated.
package seo

// SchemaVersion is the SEO document version (SemVer, additive-only) so future
// publishing milestones extend it without breaking.
const SchemaVersion = "1.0.0"

// Recommended platform limits (characters).
const (
	BlogDescMax      = 160
	YouTubeTitleMax  = 100
	YouTubeTitlePref = 70
	OGDescMax        = 200
	TwitterDescMax   = 200
	SocialSummaryMax = 220
)

// SEOMetadata is the complete SEO metadata set for one release.
type SEOMetadata struct {
	SchemaVersion       string         `json:"schemaVersion"`
	Metadata            Metadata       `json:"metadata"`
	Blog                BlogSEO        `json:"blog"`
	YouTube             YouTubeSEO     `json:"youtube"`
	Shorts              ShortsSEO      `json:"shorts"`
	Social              SocialSEO      `json:"social"`
	Keywords            Keywords       `json:"keywords"`
	Hashtags            PlatformTags   `json:"hashtags"`
	OpenGraph           OpenGraph      `json:"openGraph"`
	StructuredData      StructuredData `json:"structuredData"`
	ContentIntelligence Intelligence   `json:"contentIntelligence"`
	Warnings            []string       `json:"warnings,omitempty"`
}

// Metadata identifies the source artifacts the SEO was derived from.
type Metadata struct {
	Repository      string            `json:"repository"`
	Release         string            `json:"release"`
	SourceBlogTitle string            `json:"sourceBlogTitle"`
	GeneratedAt     string            `json:"generatedAt"`
	SourceSchemas   map[string]string `json:"sourceSchemas,omitempty"`
}

// BlogSEO is the metadata for the technical blog and its cross-posts (Dev.to,
// Medium, documentation).
type BlogSEO struct {
	Title                string      `json:"title"`
	MetaDescription      string      `json:"metaDescription"`
	Keywords             []string    `json:"keywords,omitempty"`
	Tags                 []string    `json:"tags,omitempty"`
	Slug                 string      `json:"slug"`
	AlternativeSlugs     []string    `json:"alternativeSlugs,omitempty"`
	Excerpt              string      `json:"excerpt"`
	Excerpts             Excerpts    `json:"excerpts"`
	OpenGraphTitle       string      `json:"openGraphTitle"`
	OpenGraphDescription string      `json:"openGraphDescription"`
	Twitter              TwitterCard `json:"twitter"`
	Canonical            Canonical   `json:"canonical"`
	ReadingTime          string      `json:"readingTime"`
	ReadingTimeMinutes   int         `json:"readingTimeMinutes"`
	Category             string      `json:"category"`
	TopicClusters        []string    `json:"topicClusters,omitempty"`
	Platforms            []string    `json:"platforms,omitempty"`
}

// Excerpts holds excerpts of three lengths for different surfaces.
type Excerpts struct {
	Short  string `json:"short50"`   // ~50 words
	Medium string `json:"medium100"` // ~100 words
	Long   string `json:"long200"`   // ~200 words
}

// Canonical is canonical-URL / robots metadata.
type Canonical struct {
	URL    string `json:"url,omitempty"`
	Robots string `json:"robots"`
}

// YouTubeSEO is the metadata for the long-form YouTube video.
type YouTubeSEO struct {
	Title             string         `json:"title"`
	AlternativeTitles []string       `json:"alternativeTitles,omitempty"`
	Description       string         `json:"description"`
	Keywords          []string       `json:"keywords,omitempty"`
	Tags              []string       `json:"tags,omitempty"`
	Hashtags          []string       `json:"hashtags,omitempty"`
	ChapterTitles     []ChapterTitle `json:"chapterTitles,omitempty"`
	PinnedComment     string         `json:"pinnedComment,omitempty"`
	Playlists         []string       `json:"playlists,omitempty"`
	ThumbnailText     []string       `json:"thumbnailText,omitempty"`
}

// ChapterTitle is a timestamped chapter for the video description.
type ChapterTitle struct {
	Timestamp string `json:"timestamp"`
	Title     string `json:"title"`
}

// ShortsSEO is the metadata for short-form videos (YouTube Shorts, TikTok, and
// future short-form platforms).
type ShortsSEO struct {
	Items []ShortItem `json:"items"`
}

// ShortItem is SEO for one short-form video.
type ShortItem struct {
	Platform         string   `json:"platform"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Keywords         []string `json:"keywords,omitempty"`
	Hashtags         []string `json:"hashtags,omitempty"`
	Caption          string   `json:"caption"`
	EngagementPrompt string   `json:"engagementPrompt,omitempty"`
	CTA              string   `json:"cta,omitempty"`
}

// SocialSEO is the metadata for social posts and developer communities.
type SocialSEO struct {
	Items []SocialItem `json:"items"`
}

// SocialItem is SEO for one social platform.
type SocialItem struct {
	Platform         string   `json:"platform"`
	Title            string   `json:"title"`
	Summary          string   `json:"summary"`
	Excerpt          string   `json:"excerpt"`
	Keywords         []string `json:"keywords,omitempty"`
	Hashtags         []string `json:"hashtags,omitempty"`
	EngagementPrompt string   `json:"engagementPrompt,omitempty"`
}

// Keywords is the grounded keyword taxonomy.
type Keywords struct {
	Primary    []string `json:"primary"`
	Secondary  []string `json:"secondary,omitempty"`
	LongTail   []string `json:"longTail,omitempty"`
	Technical  []string `json:"technical,omitempty"`
	Technology []string `json:"technology,omitempty"`
	AWS        []string `json:"aws,omitempty"`
	Developer  []string `json:"developer,omitempty"`
}

// PlatformTags holds platform-specific hashtags plus a merged set.
type PlatformTags struct {
	YouTube  []string `json:"youtube,omitempty"`
	TikTok   []string `json:"tiktok,omitempty"`
	LinkedIn []string `json:"linkedin,omitempty"`
	X        []string `json:"x,omitempty"`
	All      []string `json:"all,omitempty"`
}

// OpenGraph is the Open Graph + Twitter Card metadata.
type OpenGraph struct {
	Title         string      `json:"ogTitle"`
	Description   string      `json:"ogDescription"`
	Type          string      `json:"ogType"`
	SiteName      string      `json:"ogSiteName"`
	URL           string      `json:"ogUrl,omitempty"`
	Locale        string      `json:"ogLocale"`
	ImageGuidance string      `json:"ogImageGuidance"`
	Twitter       TwitterCard `json:"twitter"`
}

// TwitterCard is the Twitter/X card metadata.
type TwitterCard struct {
	Card        string `json:"card"` // summary_large_image
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageAlt    string `json:"imageAlt,omitempty"`
	Site        string `json:"site,omitempty"`
}

// StructuredData holds machine-readable metadata for search and syndication.
type StructuredData struct {
	SchemaType string                 `json:"schemaType"` // TechArticle | VideoObject
	JSONLD     map[string]interface{} `json:"jsonLd"`
	RSS        RSSItem                `json:"rss"`
	Sitemap    SitemapEntry           `json:"sitemap"`
}

// RSSItem is an RSS feed item.
type RSSItem struct {
	Title       string   `json:"title"`
	Link        string   `json:"link,omitempty"`
	Description string   `json:"description"`
	Categories  []string `json:"categories,omitempty"`
	PubDate     string   `json:"pubDate,omitempty"`
	GUID        string   `json:"guid,omitempty"`
}

// SitemapEntry is a sitemap URL entry.
type SitemapEntry struct {
	Loc        string `json:"loc,omitempty"`
	ChangeFreq string `json:"changefreq"`
	Priority   string `json:"priority"`
	LastMod    string `json:"lastmod,omitempty"`
}

// Intelligence is content-intelligence metadata across all channels.
type Intelligence struct {
	Audience             string   `json:"audience"`
	Difficulty           string   `json:"difficulty"`
	Topic                string   `json:"topic"`
	Category             string   `json:"category"`
	ContentType          string   `json:"contentType"`
	TechnologyStack      []string `json:"technologyStack,omitempty"`
	CloudServices        []string `json:"cloudServices,omitempty"`
	ProgrammingLanguages []string `json:"programmingLanguages,omitempty"`
	EstimatedReadingTime string   `json:"estimatedReadingTime"`
	EstimatedWatchTime   string   `json:"estimatedWatchTime"`
	SearchIntent         string   `json:"searchIntent"`
	UserIntent           string   `json:"userIntent"`
	SEOConfidenceScore   int      `json:"seoConfidenceScore"` // 0–100
}
