package publishing

import "strings"

// ValidationEngine performs pre-publication validation. It never lets invalid or
// unapproved content reach a Publisher.
type ValidationEngine struct{}

// Validate checks the content + metadata are publishable to the target platform.
// The first check is always approval — unapproved content is rejected.
func (ValidationEngine) Validate(c Content, m PlatformMetadata, pub Publisher) error {
	if !c.Approved {
		return ErrNotApproved
	}
	if strings.TrimSpace(c.ID) == "" {
		return errInvalid("content has no ID")
	}
	if pub == nil {
		return ErrNoPublisher
	}
	if !pub.Supports(c.Type) {
		return errUnsupported(string(pub.Name()) + " does not support " + string(c.Type))
	}

	// Required, platform-neutral metadata.
	if strings.TrimSpace(m.Title) == "" {
		return errInvalid("missing title")
	}
	// Articles need a body; media needs assets.
	switch c.Type {
	case TypeBlog, TypeDevToArticle, TypeMediumArticle, TypeHashnodeArticle:
		if strings.TrimSpace(m.Body) == "" {
			return errInvalid("article has no body")
		}
	case TypeYouTubeVideo, TypeYouTubeShorts:
		if len(c.Assets) == 0 && c.Metadata["videoFile"] == "" {
			return errMissingAsset("no video file/asset provided")
		}
	case TypeThumbnail, TypeImage, TypeDiagram:
		if len(c.Assets) == 0 && c.CoverImage == "" {
			return errMissingAsset("no image asset provided")
		}
	}

	// Platform-specific validation (also checks credentials via the adapter).
	return pub.Validate(c, m)
}
