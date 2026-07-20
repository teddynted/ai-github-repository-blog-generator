package platform

import (
	"context"
	"fmt"
)

// Video capabilities cover the output formats future video models target.
const (
	CapLongForm    Capability = "long-form"
	CapShorts      Capability = "shorts"
	CapTikTok      Capability = "tiktok"
	CapReel        Capability = "reel"
	CapExplainer   Capability = "animated-explainer"
	CapProductDemo Capability = "product-demo"
)

// VideoRequest is a provider-neutral video-render request.
type VideoRequest struct {
	Script      string
	Format      Capability // e.g. CapShorts
	Seconds     int
	AspectRatio string // 16:9 | 9:16 | 1:1
	VoiceID     string // optional narration voice
}

// VideoJob is an async render handle — video generation is long-running, so the
// contract is submit-then-poll, matching Veo/Runway/Pika/Luma/Kling.
type VideoJob struct {
	Provider string
	JobID    string
	Status   string // queued | rendering | done | failed
	URI      string // populated when done
}

// VideoProvider is the abstraction for future AI video models.
type VideoProvider interface {
	Provider
	// Submit starts an async render and returns a handle.
	Submit(ctx context.Context, req VideoRequest) (VideoJob, error)
	// Poll returns the current status of a render job.
	Poll(ctx context.Context, jobID string) (VideoJob, error)
}

// stubVideo is the reference video provider: it "completes" immediately and
// deterministically so the async contract is exercisable offline.
type stubVideo struct{ id string }

func (s *stubVideo) ID() string { return s.id }
func (s *stubVideo) Kind() Kind { return KindVideo }
func (s *stubVideo) Capabilities() []Capability {
	return []Capability{CapLongForm, CapShorts, CapTikTok, CapReel, CapExplainer, CapProductDemo}
}
func (s *stubVideo) Health(context.Context) Health { return OK() }

func (s *stubVideo) Submit(_ context.Context, req VideoRequest) (VideoJob, error) {
	if req.Script == "" {
		return VideoJob{}, fmt.Errorf("%s: empty script", s.id)
	}
	id := "job-" + string(req.Format)
	return VideoJob{Provider: s.id, JobID: id, Status: "done", URI: "s3://videos/" + id + ".mp4"}, nil
}

func (s *stubVideo) Poll(_ context.Context, jobID string) (VideoJob, error) {
	return VideoJob{Provider: s.id, JobID: jobID, Status: "done", URI: "s3://videos/" + jobID + ".mp4"}, nil
}

// NewStubVideo builds the reference video provider.
func NewStubVideo(id string) VideoProvider { return &stubVideo{id: id} }

func registerVideo(r *Registry) {
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "stub", Kind: KindVideo, Name: "Stub (reference)", Version: "1.0.0",
			Priority: 1, Description: "Deterministic reference video provider",
			Capabilities: []Capability{CapLongForm, CapShorts, CapTikTok, CapReel, CapExplainer, CapProductDemo},
		},
		Factory: func(ConfigSource) (Provider, error) { return &stubVideo{id: "stub"}, nil },
	})
}
