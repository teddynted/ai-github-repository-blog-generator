package platform

import (
	"context"
	"fmt"
)

// Voice capabilities.
const (
	CapNarration    Capability = "narration"
	CapPodcastVoice Capability = "podcast"
	CapVoiceClone   Capability = "voice-clone"
	CapMultiLang    Capability = "multi-language"
)

// SpeechRequest is a provider-neutral text-to-speech request.
type SpeechRequest struct {
	Text     string
	VoiceID  string
	Language string // BCP-47, e.g. "en-US"
	Format   string // mp3 | wav | ogg
	SpeedPct int    // 100 = normal
}

// AudioResult references synthesized audio output.
type AudioResult struct {
	Provider   string
	Format     string
	DurationMs int
	URI        string
	Bytes      []byte
}

// VoiceProvider is the abstraction for future TTS/voice providers (Amazon Polly,
// ElevenLabs, OpenAI TTS, Azure Speech, Google TTS).
type VoiceProvider interface {
	Provider
	Synthesize(ctx context.Context, req SpeechRequest) (AudioResult, error)
	// Voices lists available voice ids (optionally filtered by language).
	Voices(ctx context.Context, language string) ([]string, error)
}

// stubVoice is the reference voice provider — deterministic and offline.
type stubVoice struct{ id string }

func (s *stubVoice) ID() string { return s.id }
func (s *stubVoice) Kind() Kind { return KindVoice }
func (s *stubVoice) Capabilities() []Capability {
	return []Capability{CapNarration, CapPodcastVoice, CapMultiLang}
}
func (s *stubVoice) Health(context.Context) Health { return OK() }

func (s *stubVoice) Synthesize(_ context.Context, req SpeechRequest) (AudioResult, error) {
	if req.Text == "" {
		return AudioResult{}, fmt.Errorf("%s: empty text", s.id)
	}
	// Rough duration estimate: ~15 chars/second of speech.
	dur := len(req.Text) * 1000 / 15
	format := req.Format
	if format == "" {
		format = "mp3"
	}
	return AudioResult{Provider: s.id, Format: format, DurationMs: dur, URI: "s3://audio/" + s.id + "." + format}, nil
}

func (s *stubVoice) Voices(_ context.Context, _ string) ([]string, error) {
	return []string{"reference-neutral", "reference-warm"}, nil
}

// NewStubVoice builds the reference voice provider.
func NewStubVoice(id string) VoiceProvider { return &stubVoice{id: id} }

func registerVoice(r *Registry) {
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "stub", Kind: KindVoice, Name: "Stub (reference)", Version: "1.0.0",
			Priority: 1, Description: "Deterministic reference voice provider",
			Capabilities: []Capability{CapNarration, CapPodcastVoice, CapMultiLang},
		},
		Factory: func(ConfigSource) (Provider, error) { return &stubVoice{id: "stub"}, nil },
	})
}
