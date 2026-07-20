package platform

import (
	"context"
	"sort"
	"sync"
)

// Language is a BCP-47 language tag. The supported set is a registry, not a
// hard-coded list, so new languages are added at runtime without touching
// existing workflows.
type Language string

const (
	LangEnglish    Language = "en"
	LangFrench     Language = "fr"
	LangSpanish    Language = "es"
	LangPortuguese Language = "pt"
	LangGerman     Language = "de"
	LangJapanese   Language = "ja"
	LangChinese    Language = "zh"
	LangArabic     Language = "ar"
)

// LanguageRegistry is the extensible set of supported languages. Adding a
// language is a Register call — workflows read Supported() and never hard-code.
type LanguageRegistry struct {
	mu  sync.RWMutex
	set map[Language]string // tag -> display name
}

// NewLanguageRegistry seeds the registry with the initial supported languages.
func NewLanguageRegistry() *LanguageRegistry {
	r := &LanguageRegistry{set: map[Language]string{}}
	for tag, name := range map[Language]string{
		LangEnglish: "English", LangFrench: "French", LangSpanish: "Spanish",
		LangPortuguese: "Portuguese", LangGerman: "German", LangJapanese: "Japanese",
		LangChinese: "Chinese", LangArabic: "Arabic",
	} {
		r.set[tag] = name
	}
	return r
}

// Register adds (or renames) a supported language.
func (r *LanguageRegistry) Register(tag Language, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.set[tag] = name
}

// Supports reports whether a language is supported.
func (r *LanguageRegistry) Supports(tag Language) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.set[tag]
	return ok
}

// Supported returns the supported language tags, sorted.
func (r *LanguageRegistry) Supported() []Language {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Language, 0, len(r.set))
	for tag := range r.set {
		out = append(out, tag)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// TranslateRequest is a provider-neutral translation request.
type TranslateRequest struct {
	Text   string
	Source Language // "" = auto-detect
	Target Language
}

// TranslateResult is the translation output.
type TranslateResult struct {
	Provider string
	Text     string
	Source   Language
	Target   Language
}

// Translator is the abstraction for future translation providers. It embeds
// Provider so translators participate in the registry like any other plug-in.
type Translator interface {
	Provider
	Translate(ctx context.Context, req TranslateRequest) (TranslateResult, error)
}

// identityTranslator is the reference translator: it returns the source text
// annotated with the target language (a deterministic offline stand-in for a
// real MT engine). It demonstrates the contract and the language-registry gate.
type identityTranslator struct {
	id    string
	langs *LanguageRegistry
}

func (t *identityTranslator) ID() string { return t.id }
func (t *identityTranslator) Kind() Kind { return KindTranslation }
func (t *identityTranslator) Capabilities() []Capability {
	return []Capability{CapMultiLang}
}
func (t *identityTranslator) Health(context.Context) Health { return OK() }

func (t *identityTranslator) Translate(_ context.Context, req TranslateRequest) (TranslateResult, error) {
	if !t.langs.Supports(req.Target) {
		return TranslateResult{}, ErrUnsupported
	}
	src := req.Source
	if src == "" {
		src = LangEnglish // reference auto-detect
	}
	return TranslateResult{Provider: t.id, Text: req.Text, Source: src, Target: req.Target}, nil
}

// NewIdentityTranslator builds the reference translator over a language registry.
func NewIdentityTranslator(id string, langs *LanguageRegistry) Translator {
	if langs == nil {
		langs = NewLanguageRegistry()
	}
	return &identityTranslator{id: id, langs: langs}
}

func registerTranslation(r *Registry) {
	langs := NewLanguageRegistry()
	r.MustRegister(Registration{
		Descriptor: Descriptor{
			ID: "identity", Kind: KindTranslation, Name: "Identity (reference)", Version: "1.0.0",
			Priority: 1, Description: "Reference translator over the language registry",
			Capabilities: []Capability{CapMultiLang},
		},
		Factory: func(ConfigSource) (Provider, error) { return &identityTranslator{id: "identity", langs: langs}, nil },
	})
}
