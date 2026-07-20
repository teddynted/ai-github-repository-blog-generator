// Package platform is the extensibility foundation (Milestone 19). It turns the
// content-generation system into a modular platform where new AI providers,
// publishing platforms, content formats, storage backends, and workflow
// integrations are added by IMPLEMENTING INTERFACES and registering them —
// never by modifying existing business logic.
//
// It embodies Clean Architecture, SOLID, the Open/Closed Principle, and
// dependency inversion:
//
//   - Every capability is expressed as a small interface (LLMProvider,
//     ImageProvider, VideoProvider, VoiceProvider, Translator, VectorStore,
//     PublishingProvider, MCPClient, WorkflowEngine, ContentModule).
//   - Every implementation is a plug-in described by a Descriptor and built by a
//     Factory, registered into a Registry.
//   - Business services depend only on the interfaces + the Registry, so they
//     never name a concrete provider — selection is by Kind + Capability +
//     priority, with health checks and fallback.
//   - Configuration is injected through the ConfigSource port, so no
//     provider-specific config logic leaks into business services.
//
// Each domain ships at least one reference implementation to demonstrate the
// model; production providers (Bedrock, OpenAI, Ollama, Stable Diffusion, Veo,
// ElevenLabs, Pinecone, LinkedIn, …) are added the same way, as new files that
// implement the interface and call Register — with zero edits to this package.
package platform

import "context"

// APIVersion is the platform extension API version (SemVer). A provider declares
// the API version it targets; the registry rejects majors it cannot support, so
// backward compatibility is explicit and enforced.
const APIVersion = "1.0.0"

// Kind identifies the category of a provider. New kinds can be added without
// changing the registry — it is keyed generically by Kind.
type Kind string

const (
	KindLLM         Kind = "llm"
	KindImage       Kind = "image"
	KindVideo       Kind = "video"
	KindVoice       Kind = "voice"
	KindTranslation Kind = "translation"
	KindVector      Kind = "vector"
	KindPublishing  Kind = "publishing"
	KindMCP         Kind = "mcp"
	KindWorkflow    Kind = "workflow"
	KindContent     Kind = "content"
	KindStorage     Kind = "storage"
)

// Capability is a free-form capability tag a provider advertises (e.g.
// "text-generation", "streaming", "thumbnail", "tts", "embed", "publish"). New
// capabilities never require code changes — selection matches on these strings.
type Capability string

// Provider is the base interface every plug-in implements. Domain interfaces
// (LLMProvider, ImageProvider, …) embed it, so the registry can treat all
// providers uniformly while callers use the richer domain interface.
type Provider interface {
	// ID is the unique provider identifier within its Kind (e.g. "bedrock").
	ID() string
	// Kind is the category this provider serves.
	Kind() Kind
	// Capabilities lists what this provider can do.
	Capabilities() []Capability
	// Health reports current availability, used for selection and fallback.
	Health(ctx context.Context) Health
}

// Descriptor is the registration metadata for a provider. It is what the registry
// stores and reasons over — selection, priority, capability, and version
// compatibility all read the Descriptor without instantiating the provider.
type Descriptor struct {
	ID           string       // unique within Kind
	Kind         Kind         // category
	Name         string       // human-readable name
	Version      string       // provider version (SemVer)
	APIVersion   string       // platform API version it targets (defaults to APIVersion)
	Priority     int          // higher wins when multiple providers match
	Capabilities []Capability // advertised capabilities
	Managed      bool         // true = managed/cloud, false = self-hosted/local
	Description  string       // one-line summary
}

// Factory constructs a provider from configuration. Lazy construction means a
// provider's SDK/credentials are only touched when it is actually selected.
type Factory func(cfg ConfigSource) (Provider, error)

// Registration bundles a Descriptor with its Factory for the registry.
type Registration struct {
	Descriptor Descriptor
	Factory    Factory
}

// HasCapability reports whether a descriptor advertises a capability.
func (d Descriptor) HasCapability(c Capability) bool {
	for _, cap := range d.Capabilities {
		if cap == c {
			return true
		}
	}
	return false
}

// apiVersion returns the descriptor's declared API version, defaulting to the
// current platform APIVersion when unset.
func (d Descriptor) apiVersion() string {
	if d.APIVersion == "" {
		return APIVersion
	}
	return d.APIVersion
}
