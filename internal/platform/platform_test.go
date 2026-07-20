package platform

import (
	"context"
	"errors"
	"testing"
)

func freshRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry()
	RegisterDefaults(r)
	return r
}

// --- registry core ---

func TestRegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	reg := Registration{
		Descriptor: Descriptor{ID: "x", Kind: KindLLM, Version: "1.0.0", Capabilities: []Capability{CapChat}},
		Factory:    func(ConfigSource) (Provider, error) { return NewEchoLLM("x", "X"), nil },
	}
	if err := r.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := r.Lookup(KindLLM, "x"); !ok {
		t.Error("lookup should find x")
	}
	// Duplicate rejected.
	if err := r.Register(reg); !errors.Is(err, ErrAlreadyRegistered) {
		t.Errorf("duplicate should be rejected, got %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Registration{Descriptor: Descriptor{Kind: KindLLM}, Factory: func(ConfigSource) (Provider, error) { return nil, nil }}); !errors.Is(err, ErrInvalidRegistration) {
		t.Error("missing id should be invalid")
	}
	if err := r.Register(Registration{Descriptor: Descriptor{ID: "x", Kind: KindLLM}}); !errors.Is(err, ErrInvalidRegistration) {
		t.Error("missing factory should be invalid")
	}
}

func TestIncompatibleAPIVersion(t *testing.T) {
	r := NewRegistry()
	err := r.Register(Registration{
		Descriptor: Descriptor{ID: "old", Kind: KindLLM, APIVersion: "2.0.0"},
		Factory:    staticFactory(NewEchoLLM("old", "Old")),
	})
	if !errors.Is(err, ErrIncompatible) {
		t.Errorf("major-version mismatch should be incompatible, got %v", err)
	}
	// Bad version string is also incompatible.
	if err := r.Register(Registration{Descriptor: Descriptor{ID: "bad", Kind: KindLLM, APIVersion: "not-semver"}, Factory: staticFactory(NewEchoLLM("bad", "Bad"))}); !errors.Is(err, ErrIncompatible) {
		t.Errorf("bad version should be incompatible, got %v", err)
	}
}

func TestMustRegisterPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustRegister should panic on invalid registration")
		}
	}()
	NewRegistry().MustRegister(Registration{Descriptor: Descriptor{Kind: KindLLM}})
}

func TestDescriptorsSortedByPriority(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "low", Kind: KindLLM, Priority: 1}, Factory: staticFactory(NewEchoLLM("low", "L"))})
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "high", Kind: KindLLM, Priority: 9}, Factory: staticFactory(NewEchoLLM("high", "H"))})
	descs := r.Descriptors(KindLLM)
	if len(descs) != 2 || descs[0].ID != "high" {
		t.Errorf("expected high priority first, got %+v", descs)
	}
	if len(r.Descriptors(KindVideo)) != 0 {
		t.Error("unregistered kind should be empty")
	}
}

func TestKindsAndAllDescriptors(t *testing.T) {
	r := freshRegistry(t)
	kinds := r.Kinds()
	if len(kinds) != 10 {
		t.Errorf("expected 10 kinds, got %d (%v)", len(kinds), kinds)
	}
	if len(r.AllDescriptors()) != 12 {
		t.Errorf("expected 12 built-in providers, got %d", len(r.AllDescriptors()))
	}
}

// --- selection: capability, priority, fallback, health ---

func TestSelectByCapabilityAndPriority(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "a", Kind: KindLLM, Priority: 1, Capabilities: []Capability{CapChat}}, Factory: staticFactory(NewEchoLLM("a", "A"))})
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "b", Kind: KindLLM, Priority: 5, Capabilities: []Capability{CapChat, CapJSONMode}}, Factory: staticFactory(NewEchoLLM("b", "B"))})

	prov, err := r.Select(context.Background(), KindLLM, nil, WithCapability(CapChat))
	if err != nil || prov.ID() != "b" {
		t.Fatalf("should select highest-priority chat provider b, got %v %v", prov, err)
	}
	// Capability that only 'a' would have — but neither has it → not found.
	if _, err := r.Select(context.Background(), KindLLM, nil, WithCapability(CapToolUse)); err == nil {
		t.Error("missing capability should fail selection")
	}
	// JSON-mode only b has.
	prov, err = r.Select(context.Background(), KindLLM, nil, WithCapability(CapJSONMode))
	if err != nil || prov.ID() != "b" {
		t.Errorf("should select b for json-mode, got %v %v", prov, err)
	}
}

func TestSelectFallbackOnUnhealthy(t *testing.T) {
	r := NewRegistry()
	// Higher priority is unhealthy → selection falls back to the healthy one.
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "down", Kind: KindLLM, Priority: 9, Capabilities: []Capability{CapChat}}, Factory: staticFactory(&flakyProvider{id: "down", healthy: false})})
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "up", Kind: KindLLM, Priority: 1, Capabilities: []Capability{CapChat}}, Factory: staticFactory(&flakyProvider{id: "up", healthy: true})})

	prov, err := r.Select(context.Background(), KindLLM, nil, WithCapability(CapChat))
	if err != nil || prov.ID() != "up" {
		t.Fatalf("should fall back to healthy provider, got %v %v", prov, err)
	}
}

func TestSelectAllUnhealthy(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "d1", Kind: KindLLM}, Factory: staticFactory(&flakyProvider{id: "d1", healthy: false})})
	_, err := r.Select(context.Background(), KindLLM, nil)
	var se *SelectionError
	if !errors.As(err, &se) {
		t.Fatalf("expected SelectionError, got %v", err)
	}
	if len(se.Tried) != 1 || se.Error() == "" {
		t.Errorf("selection error should list tried: %+v", se)
	}
}

func TestSelectFactoryError(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "boom", Kind: KindLLM}, Factory: func(ConfigSource) (Provider, error) { return nil, errors.New("build failed") }})
	if _, err := r.Select(context.Background(), KindLLM, nil); err == nil {
		t.Error("factory error should propagate to selection failure")
	}
}

func TestSelectManagedLocalFilters(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "cloud", Kind: KindLLM, Managed: true, Priority: 1}, Factory: staticFactory(NewEchoLLM("cloud", "C"))})
	r.MustRegister(Registration{Descriptor: Descriptor{ID: "local", Kind: KindLLM, Managed: false, Priority: 2}, Factory: staticFactory(NewEchoLLM("local", "L"))})

	prov, _ := r.Select(context.Background(), KindLLM, nil, ManagedOnly())
	if prov.ID() != "cloud" {
		t.Error("ManagedOnly should pick cloud")
	}
	prov, _ = r.Select(context.Background(), KindLLM, nil, LocalOnly())
	if prov.ID() != "local" {
		t.Error("LocalOnly should pick local")
	}
	// WithID forces a specific one.
	prov, _ = r.Select(context.Background(), KindLLM, nil, WithID("cloud"))
	if prov.ID() != "cloud" {
		t.Error("WithID should force cloud")
	}
	// MinPriority filters out low ones.
	if c := r.Candidates(KindLLM, WithMinPriority(2)); len(c) != 1 || c[0].ID != "local" {
		t.Errorf("MinPriority should keep only local, got %+v", c)
	}
}

func TestCreateNotFound(t *testing.T) {
	r := freshRegistry(t)
	if _, err := r.Create(KindLLM, "nope", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id should be not-found, got %v", err)
	}
}

// --- typed selection / factory ---

func TestTypedSelection(t *testing.T) {
	r := freshRegistry(t)
	llm, err := SelectTyped[LLMProvider](context.Background(), r, KindLLM, nil, WithCapability(CapTextGeneration))
	if err != nil {
		t.Fatalf("typed select: %v", err)
	}
	resp, err := llm.Generate(context.Background(), LLMRequest{Prompt: "hi"})
	if err != nil || resp.Text == "" {
		t.Errorf("generate: %v %+v", err, resp)
	}
	// Asserting to the wrong interface fails cleanly.
	if _, err := SelectTyped[VideoProvider](context.Background(), r, KindLLM, nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("wrong-interface assertion should be unsupported, got %v", err)
	}
	// Typed threads a prior error.
	if _, err := Typed[LLMProvider](nil, ErrNotFound); !errors.Is(err, ErrNotFound) {
		t.Error("Typed should thread the input error")
	}
}

// --- config framework ---

func TestConfigSources(t *testing.T) {
	m := MapConfig{"A": "1"}
	if v, ok := m.Get("A"); !ok || v != "1" {
		t.Error("map get")
	}
	chain := NewChainConfig(MapConfig{}, m, nil)
	if v, ok := chain.Get("A"); !ok || v != "1" {
		t.Error("chain should find A in a later source")
	}
	if _, ok := chain.Get("missing"); ok {
		t.Error("chain should miss")
	}
	if GetDefault(m, "A", "z") != "1" || GetDefault(m, "B", "z") != "z" {
		t.Error("GetDefault")
	}
	if Lookup(nil, "x") != "" || Lookup(m, "A") != "1" {
		t.Error("Lookup")
	}
	pfx := PrefixConfig{Prefix: "P_", Source: MapConfig{"P_K": "v"}}
	if v, _ := pfx.Get("K"); v != "v" {
		t.Error("prefix config")
	}
	if _, ok := (PrefixConfig{}).Get("x"); ok {
		t.Error("nil-source prefix should miss")
	}
}

// --- reference providers ---

func TestEchoLLMAndStreaming(t *testing.T) {
	e := NewEchoLLM("echo", "Echo")
	if _, err := e.Generate(context.Background(), LLMRequest{}); err == nil {
		t.Error("empty prompt should error")
	}
	s, ok := e.(StreamingLLM)
	if !ok {
		t.Fatal("echo should implement StreamingLLM")
	}
	var tokens int
	_, err := s.GenerateStream(context.Background(), LLMRequest{Prompt: "a b c"}, func(string) error { tokens++; return nil })
	if err != nil || tokens == 0 {
		t.Errorf("stream: %v tokens=%d", err, tokens)
	}
}

func TestImageVideoVoiceReferences(t *testing.T) {
	img, _ := NewPlaceholderImage("img").Generate(context.Background(), ImageRequest{Purpose: CapThumbnail})
	if img.Width == 0 || img.Format != "png" {
		t.Error("image defaults")
	}
	vid := NewStubVideo("vid")
	if _, err := vid.Submit(context.Background(), VideoRequest{}); err == nil {
		t.Error("empty script should error")
	}
	job, _ := vid.Submit(context.Background(), VideoRequest{Script: "s", Format: CapShorts})
	if job.Status != "done" {
		t.Error("stub video should complete")
	}
	if p, _ := vid.Poll(context.Background(), job.JobID); p.URI == "" {
		t.Error("poll should return uri")
	}
	voice := NewStubVoice("v")
	if _, err := voice.Synthesize(context.Background(), SpeechRequest{}); err == nil {
		t.Error("empty text should error")
	}
	au, _ := voice.Synthesize(context.Background(), SpeechRequest{Text: "hello world"})
	if au.DurationMs == 0 {
		t.Error("voice duration")
	}
	if vs, _ := voice.Voices(context.Background(), "en"); len(vs) == 0 {
		t.Error("voices")
	}
}

func TestTranslationAndLanguageRegistry(t *testing.T) {
	langs := NewLanguageRegistry()
	if !langs.Supports(LangJapanese) || len(langs.Supported()) != 8 {
		t.Errorf("seed languages, got %d", len(langs.Supported()))
	}
	// Extensible: add a new language without touching workflows.
	langs.Register(Language("sw"), "Swahili")
	if !langs.Supports("sw") {
		t.Error("added language should be supported")
	}
	tr := NewIdentityTranslator("id", langs)
	res, err := tr.Translate(context.Background(), TranslateRequest{Text: "hi", Target: LangFrench})
	if err != nil || res.Target != LangFrench || res.Source != LangEnglish {
		t.Errorf("translate: %v %+v", err, res)
	}
	if _, err := tr.Translate(context.Background(), TranslateRequest{Text: "x", Target: "zz"}); !errors.Is(err, ErrUnsupported) {
		t.Error("unsupported target should error")
	}
	if NewIdentityTranslator("d", nil) == nil {
		t.Error("nil langs should default")
	}
}

func TestVectorStore(t *testing.T) {
	vs := NewMemoryVectorStore("m")
	if err := vs.Upsert(context.Background(), []Vector{{ID: ""}}); !errors.Is(err, ErrUnsupported) {
		t.Error("empty id should error")
	}
	_ = vs.Upsert(context.Background(), []Vector{
		{ID: "a", Values: []float32{1, 0}, Metadata: map[string]string{"k": "v"}},
		{ID: "b", Values: []float32{0, 1}},
	})
	matches, _ := vs.Query(context.Background(), []float32{1, 0.05}, 1)
	if len(matches) != 1 || matches[0].ID != "a" {
		t.Errorf("nearest should be a, got %+v", matches)
	}
	_ = vs.Delete(context.Background(), []string{"a"})
	matches, _ = vs.Query(context.Background(), []float32{1, 0}, 5)
	if len(matches) != 1 || matches[0].ID != "b" {
		t.Error("delete should remove a")
	}
	// Dimension mismatch / empty → 0 score, no panic.
	if cosine([]float32{1}, []float32{1, 2}) != 0 || cosine(nil, nil) != 0 {
		t.Error("cosine guards")
	}
}

func TestPublishingReference(t *testing.T) {
	pub := NewDryRunPublisher("dry")
	if _, err := pub.Publish(context.Background(), Article{}); err == nil {
		t.Error("empty title should error")
	}
	res, _ := pub.Publish(context.Background(), Article{Title: "T", Draft: true})
	if res.Status != "draft" {
		t.Error("draft status")
	}
}

func TestMCPReference(t *testing.T) {
	mcp := NewMockMCPServer("mock")
	tools, _ := mcp.ListTools(context.Background())
	if len(tools) != 2 || tools[0].Name != "clock" {
		t.Errorf("tools sorted: %+v", tools)
	}
	res, _ := mcp.CallTool(context.Background(), "echo", map[string]string{"message": "hi"})
	if res.Content != "hi" {
		t.Error("echo tool")
	}
	if c, _ := mcp.CallTool(context.Background(), "clock", nil); c.Content == "" {
		t.Error("clock tool")
	}
	if _, err := mcp.CallTool(context.Background(), "nope", nil); err == nil {
		t.Error("unknown tool should error")
	}
}

func TestWorkflowReference(t *testing.T) {
	w := NewInProcessWorkflow("wf")
	if _, err := w.Submit(context.Background(), WorkflowSpec{}); err == nil {
		t.Error("empty spec should error")
	}
	h, _ := w.Submit(context.Background(), WorkflowSpec{Name: "pipe", Steps: []string{"a", "b"}})
	if h.Status != "succeeded" {
		t.Error("workflow should succeed")
	}
	got, _ := w.Status(context.Background(), h.RunID)
	if got.RunID != h.RunID {
		t.Error("status lookup")
	}
	if _, err := w.Status(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Error("missing run should be not-found")
	}
}

func TestContentAndPodcastReference(t *testing.T) {
	m := NewContentModule("blog", ContentBlog)
	if m.ContentType() != ContentBlog {
		t.Error("content type")
	}
	if _, err := m.Build(context.Background(), ContentRequest{}); err == nil {
		t.Error("empty title should error")
	}
	art, _ := m.Build(context.Background(), ContentRequest{Title: "T", Topic: "go"})
	if art.Format != "markdown" || art.Body == "" {
		t.Error("artifact")
	}
	pod := NewStubPodcast("pod")
	if _, err := pod.Assemble(context.Background(), PodcastSpec{}); err == nil {
		t.Error("empty title should error")
	}
	ep, _ := pod.Assemble(context.Background(), PodcastSpec{Title: "E1", Hosts: []string{"AI"}})
	if ep.AudioURI == "" || ep.RSSItem == "" {
		t.Error("episode")
	}
	rss := pod.RSS("Show", []PodcastEpisode{ep})
	if len(rss) == 0 || rss[:5] != "<rss " {
		t.Error("rss")
	}
}

// --- health + default registry ---

func TestHealthHelpers(t *testing.T) {
	if !OK().Healthy() || Down("x").Healthy() {
		t.Error("health helpers")
	}
	if (Health{Status: HealthDegraded}).Healthy() != true {
		t.Error("degraded is usable")
	}
}

func TestDefaultRegistryAutoRegistration(t *testing.T) {
	// The package init() must have populated the default registry.
	if len(Default().AllDescriptors()) < 12 {
		t.Errorf("default registry should be auto-populated, got %d", len(Default().AllDescriptors()))
	}
	llm, err := SelectTyped[LLMProvider](context.Background(), Default(), KindLLM, EnvConfig{}, WithCapability(CapTextGeneration))
	if err != nil || llm.ID() != "echo" {
		t.Errorf("default select: %v", err)
	}
}

func TestPackageRegisterHelpers(t *testing.T) {
	// Register a custom provider into the default registry (plugin pattern).
	err := Register(Registration{
		Descriptor: Descriptor{ID: "custom-test", Kind: KindLLM, Version: "1.0.0"},
		Factory:    staticFactory(NewEchoLLM("custom-test", "Custom")),
	})
	if err != nil {
		t.Fatalf("register into default: %v", err)
	}
	if _, ok := Default().Lookup(KindLLM, "custom-test"); !ok {
		t.Error("custom provider should be registered")
	}
	// MustRegister duplicate panics.
	defer func() { _ = recover() }()
	MustRegister(Registration{Descriptor: Descriptor{ID: "custom-test", Kind: KindLLM}, Factory: staticFactory(NewEchoLLM("custom-test", "x"))})
}

// TestAllReferenceBaseMethods creates every registered provider via its factory
// and exercises the base Provider contract — covering all factories + accessors.
func TestAllReferenceBaseMethods(t *testing.T) {
	r := freshRegistry(t)
	for _, d := range r.AllDescriptors() {
		prov, err := r.Create(d.Kind, d.ID, EnvConfig{})
		if err != nil {
			t.Fatalf("create %s/%s: %v", d.Kind, d.ID, err)
		}
		if prov.ID() != d.ID {
			t.Errorf("id mismatch: %s vs %s", prov.ID(), d.ID)
		}
		if prov.Kind() != d.Kind {
			t.Errorf("kind mismatch for %s", d.ID)
		}
		if len(prov.Capabilities()) == 0 {
			t.Errorf("%s advertises no capabilities", d.ID)
		}
		if !prov.Health(context.Background()).Healthy() {
			t.Errorf("%s should be healthy", d.ID)
		}
	}
}

func TestSelectionErrorUnwrap(t *testing.T) {
	se := &SelectionError{Kind: KindLLM, Capability: CapChat, Err: ErrNotFound}
	if !errors.Is(se, ErrNotFound) || se.Unwrap() != ErrNotFound {
		t.Error("SelectionError should unwrap to its cause")
	}
	if se.Error() == "" {
		t.Error("error string")
	}
}

func TestStreamOnTokenError(t *testing.T) {
	s := NewEchoLLM("e", "E").(StreamingLLM)
	boom := errors.New("stop")
	_, err := s.GenerateStream(context.Background(), LLMRequest{Prompt: "a b"}, func(string) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("onToken error should propagate, got %v", err)
	}
}

// --- test double ---

type flakyProvider struct {
	id      string
	healthy bool
}

func (f *flakyProvider) ID() string                 { return f.id }
func (f *flakyProvider) Kind() Kind                 { return KindLLM }
func (f *flakyProvider) Capabilities() []Capability { return []Capability{CapChat} }
func (f *flakyProvider) Health(context.Context) Health {
	if f.healthy {
		return OK()
	}
	return Down("unavailable")
}
func (f *flakyProvider) Generate(context.Context, LLMRequest) (LLMResponse, error) {
	return LLMResponse{Text: f.id}, nil
}
