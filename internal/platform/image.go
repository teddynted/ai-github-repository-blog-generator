package platform

import "context"

// Image capabilities.
const (
	CapThumbnail     Capability = "thumbnail"
	CapIllustration  Capability = "illustration"
	CapDiagram       Capability = "diagram"
	CapSocialGraphic Capability = "social-graphic"
	CapBanner        Capability = "banner"
)

// ImageRequest is a provider-neutral image request. Purpose lets one provider
// serve thumbnails, illustrations, diagrams, social graphics, and banners.
type ImageRequest struct {
	Prompt  string
	Purpose Capability // e.g. CapThumbnail
	Width   int
	Height  int
	Style   string
	Seed    int64
}

// ImageResult references generated image output (URL or bytes; here a descriptor).
type ImageResult struct {
	Provider string
	Format   string // png | jpg | webp | svg
	Width    int
	Height   int
	URI      string // s3:// or file:// or data: — where the asset lives
	Bytes    []byte // optional inline bytes
}

// ImageProvider is the abstraction for future image models (Stable Diffusion,
// FLUX, DALL·E, Amazon Nova Canvas, Midjourney, …).
type ImageProvider interface {
	Provider
	Generate(ctx context.Context, req ImageRequest) (ImageResult, error)
}

// placeholderImage is the reference image provider. It returns a deterministic
// descriptor (no pixels) so the pipeline is exercisable offline; a real provider
// swaps in the model call and returns bytes/URI.
type placeholderImage struct{ id string }

func (p *placeholderImage) ID() string { return p.id }
func (p *placeholderImage) Kind() Kind { return KindImage }
func (p *placeholderImage) Capabilities() []Capability {
	return []Capability{CapThumbnail, CapIllustration, CapDiagram, CapSocialGraphic, CapBanner}
}
func (p *placeholderImage) Health(context.Context) Health { return OK() }

func (p *placeholderImage) Generate(_ context.Context, req ImageRequest) (ImageResult, error) {
	w, h := req.Width, req.Height
	if w == 0 {
		w = 1280
	}
	if h == 0 {
		h = 720
	}
	return ImageResult{
		Provider: p.id, Format: "png", Width: w, Height: h,
		URI: "data:image/png;placeholder;" + string(req.Purpose),
	}, nil
}

// NewPlaceholderImage builds the reference image provider.
func NewPlaceholderImage(id string) ImageProvider { return &placeholderImage{id: id} }

func registerImage(r *Registry) {
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "placeholder", Kind: KindImage, Name: "Placeholder (reference)", Version: "1.0.0",
			Priority: 1, Description: "Deterministic reference image provider",
			Capabilities: []Capability{CapThumbnail, CapIllustration, CapDiagram, CapSocialGraphic, CapBanner},
		},
		Factory: func(ConfigSource) (Provider, error) { return &placeholderImage{id: "placeholder"}, nil },
	})
}
