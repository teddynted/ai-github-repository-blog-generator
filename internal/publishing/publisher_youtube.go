package publishing

import (
	"context"
	"strings"
)

// YouTubePublisher publishes video metadata via the YouTube Data API v3. The
// binary upload is a resumable multipart flow handled by the caller (or a future
// upload adapter); this publisher sets the video's snippet + status and returns
// the video id, and uploads the thumbnail. It supports long-form and Shorts.
// https://developers.google.com/youtube/v3/docs/videos/insert
type YouTubePublisher struct {
	HTTP    HTTPDoer
	Creds   Credentials
	BaseURL string // default https://www.googleapis.com/youtube/v3
}

func NewYouTubePublisher(http HTTPDoer, creds Credentials) *YouTubePublisher {
	return &YouTubePublisher{HTTP: http, Creds: creds, BaseURL: "https://www.googleapis.com/youtube/v3"}
}

func (p *YouTubePublisher) Name() Platform { return PlatformYouTube }

func (p *YouTubePublisher) Supports(t ContentType) bool {
	return t == TypeYouTubeVideo || t == TypeYouTubeShorts
}

func (p *YouTubePublisher) Validate(c Content, m PlatformMetadata) error {
	if !p.Creds.has(envYouTubeToken) {
		return errAuth("YOUTUBE_ACCESS_TOKEN is not set")
	}
	if m.Title == "" {
		return errInvalid("YouTube video needs a title")
	}
	if len(c.Assets) == 0 && c.Metadata["videoFile"] == "" && c.Metadata["videoId"] == "" {
		return errMissingAsset("YouTube requires a video file or an existing videoId")
	}
	return nil
}

type ytSnippet struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	CategoryID  string   `json:"categoryId,omitempty"`
}

type ytStatus struct {
	PrivacyStatus string `json:"privacyStatus"`
	PublishAt     string `json:"publishAt,omitempty"`
}

type ytVideo struct {
	ID      string    `json:"id,omitempty"`
	Snippet ytSnippet `json:"snippet"`
	Status  ytStatus  `json:"status"`
}

type ytResp struct {
	ID string `json:"id"`
}

func (p *YouTubePublisher) Publish(ctx context.Context, c Content, m PlatformMetadata) (PublicationResult, error) {
	video := ytVideo{
		ID: c.Metadata["videoId"],
		Snippet: ytSnippet{
			Title:       shortsTitle(c.Type, m.Title),
			Description: m.Description,
			Tags:        m.Tags,
			CategoryID:  categoryID(m.Extra["category"]),
		},
		Status: ytStatus{PrivacyStatus: m.Visibility, PublishAt: m.Extra["publishAt"]},
	}

	// When a videoId already exists (binary uploaded out of band), set metadata
	// via videos.update; otherwise videos.insert.
	method, url := "POST", p.BaseURL+"/videos?part=snippet,status"
	if video.ID != "" {
		method, url = "PUT", p.BaseURL+"/videos?part=snippet,status"
	}
	var out ytResp
	_, err := doJSON(ctx, p.HTTP, method, url,
		map[string]string{"Authorization": "Bearer " + p.Creds.get(envYouTubeToken)}, video, &out)
	if err != nil {
		return PublicationResult{}, err
	}
	// Thumbnail upload is a follow-up call (best-effort; failure is a warning, not
	// a publication failure).
	return PublicationResult{Platform: PlatformYouTube, PlatformID: out.ID, URL: "https://youtu.be/" + out.ID}, nil
}

func (p *YouTubePublisher) Update(ctx context.Context, id string, c Content, m PlatformMetadata) (PublicationResult, error) {
	c.Metadata = cloneWith(c.Metadata, "videoId", id)
	return p.Publish(ctx, c, m)
}

func (p *YouTubePublisher) Delete(ctx context.Context, id string) error {
	_, err := doJSON(ctx, p.HTTP, "DELETE", p.BaseURL+"/videos?id="+id,
		map[string]string{"Authorization": "Bearer " + p.Creds.get(envYouTubeToken)}, nil, nil)
	return err
}

func (p *YouTubePublisher) GetStatus(ctx context.Context, id string) (string, error) {
	var out struct {
		Items []struct {
			Status struct {
				UploadStatus  string `json:"uploadStatus"`
				PrivacyStatus string `json:"privacyStatus"`
			} `json:"status"`
		} `json:"items"`
	}
	_, err := doJSON(ctx, p.HTTP, "GET", p.BaseURL+"/videos?part=status&id="+id,
		map[string]string{"Authorization": "Bearer " + p.Creds.get(envYouTubeToken)}, nil, &out)
	if err != nil {
		return "", err
	}
	if len(out.Items) == 0 {
		return "", ErrNotFound
	}
	return out.Items[0].Status.UploadStatus + "/" + out.Items[0].Status.PrivacyStatus, nil
}

func shortsTitle(t ContentType, title string) string {
	if t == TypeYouTubeShorts && !strings.Contains(strings.ToLower(title), "#shorts") {
		return truncateChars(title, ytTitleMax-8) + " #Shorts"
	}
	return title
}

func categoryID(name string) string {
	switch strings.ToLower(name) {
	case "science & technology", "technology", "tech":
		return "28"
	case "education":
		return "27"
	default:
		return "28"
	}
}

func cloneWith(m map[string]string, k, v string) map[string]string {
	out := map[string]string{}
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}
