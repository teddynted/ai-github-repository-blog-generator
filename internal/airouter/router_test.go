package airouter

import (
	"context"
	"errors"
	"testing"
)

type stub struct {
	name  string
	err   error
	calls int
}

func (s *stub) Generate(_ context.Context, _ string) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	return s.name, nil
}

func providers() (claude, ollama *stub, m map[string]Model) {
	claude = &stub{name: "claude-out"}
	ollama = &stub{name: "ollama-out"}
	return claude, ollama, map[string]Model{ProviderClaude: claude, ProviderOllama: ollama}
}

func TestDefaultPolicyRoutesHighValueToClaudeRestToOllama(t *testing.T) {
	claude, ollama, m := providers()
	r := New(m, DefaultRules(), ProviderOllama, ProviderOllama, nil)

	// High-value → Claude.
	for _, k := range []string{"blog", "architecture", "linkedin", "x-thread"} {
		if got, _ := r.ModelFor(k).Generate(context.Background(), "p"); got != "claude-out" {
			t.Errorf("%s routed to %q, want claude", k, got)
		}
	}
	// Commodity → Ollama (default).
	for _, k := range []string{"seo-metadata", "visual-assets", "youtube-shorts", "tiktok", "storyboard", "voiceover", "youtube"} {
		if got, _ := r.ModelFor(k).Generate(context.Background(), "p"); got != "ollama-out" {
			t.Errorf("%s routed to %q, want ollama", k, got)
		}
	}
	_ = claude
	_ = ollama
}

func TestFallsBackWhenPrimaryErrors(t *testing.T) {
	claude := &stub{name: "claude-out", err: errors.New("Operation not allowed")}
	ollama := &stub{name: "ollama-out"}
	m := map[string]Model{ProviderClaude: claude, ProviderOllama: ollama}
	r := New(m, DefaultRules(), ProviderOllama, ProviderOllama, nil)

	// blog → claude, but claude errors → falls back to ollama.
	got, err := r.ModelFor("blog").Generate(context.Background(), "p")
	if err != nil {
		t.Fatalf("expected fallback, got err: %v", err)
	}
	if got != "ollama-out" {
		t.Errorf("fallback produced %q, want ollama-out", got)
	}
}

func TestMissingProviderFallsThroughToDefault(t *testing.T) {
	// Only Ollama registered (Claude not configured): claude-kinds route to Ollama.
	ollama := &stub{name: "ollama-out"}
	m := map[string]Model{ProviderOllama: ollama}
	r := New(m, DefaultRules(), ProviderOllama, ProviderOllama, nil)

	if got, _ := r.ModelFor("blog").Generate(context.Background(), "p"); got != "ollama-out" {
		t.Errorf("blog with no claude routed to %q, want ollama-out", got)
	}
	if names := r.Providers(); len(names) != 1 || names[0] != ProviderOllama {
		t.Errorf("providers = %v", names)
	}
}

func TestParseRulesProviderListForm(t *testing.T) {
	rules, err := ParseRules(`{"claude":["architecture","linkedin","x-thread"],"ollama":["seo","visual-assets"]}`)
	if err != nil {
		t.Fatalf("ParseRules: %v", err)
	}
	if rules["architecture"] != "claude" || rules["x-thread"] != "claude" {
		t.Errorf("claude rules = %+v", rules)
	}
	// alias: "seo" -> "seo-metadata".
	if rules["seo-metadata"] != "ollama" {
		t.Errorf("seo alias not normalised: %+v", rules)
	}
}

func TestParseRulesKindProviderForm(t *testing.T) {
	rules, err := ParseRules(`{"blog":"claude","tiktok":"ollama"}`)
	if err != nil {
		t.Fatalf("ParseRules: %v", err)
	}
	if rules["blog"] != "claude" || rules["tiktok"] != "ollama" {
		t.Errorf("rules = %+v", rules)
	}
}

func TestParseRulesEmptyGivesDefault(t *testing.T) {
	rules, err := ParseRules("")
	if err != nil {
		t.Fatal(err)
	}
	if rules["blog"] != "claude" {
		t.Errorf("empty config should yield default policy, got %+v", rules)
	}
}

func TestParseRulesInvalid(t *testing.T) {
	if _, err := ParseRules(`{"blog": 123}`); err == nil {
		t.Fatal("expected error for a non-list, non-string entry")
	}
}

func TestDecisionsReflectPolicy(t *testing.T) {
	_, _, m := providers()
	r := New(m, DefaultRules(), ProviderOllama, ProviderOllama, nil)
	d := r.Decisions(AllKinds)
	if d["blog"] != "claude" || d["seo-metadata"] != "ollama" {
		t.Errorf("decisions = %+v", d)
	}
}

func TestParseRulesCompactForm(t *testing.T) {
	rules, err := ParseRules("claude=blog,architecture,linkedin,x-thread;ollama=seo,tiktok")
	if err != nil {
		t.Fatalf("ParseRules compact: %v", err)
	}
	if rules["blog"] != "claude" || rules["x-thread"] != "claude" {
		t.Errorf("claude compact rules = %+v", rules)
	}
	if rules["seo-metadata"] != "ollama" || rules["tiktok"] != "ollama" {
		t.Errorf("ollama compact rules = %+v", rules)
	}
}

func TestParseRulesCompactInvalid(t *testing.T) {
	if _, err := ParseRules("claude"); err == nil {
		t.Fatal("expected error for compact group with no '='")
	}
}
