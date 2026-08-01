package contentoptimizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// DeterministicReasoner produces grounded structured reasoning directly from the
// detected patterns — no model, no fabrication. It is always available and is the
// fallback whenever the AI provider fails.
type DeterministicReasoner struct{}

// NewDeterministicReasoner constructs the deterministic reasoner.
func NewDeterministicReasoner() DeterministicReasoner { return DeterministicReasoner{} }

// Reason derives why content performed well/poorly straight from the patterns.
func (DeterministicReasoner) Reason(_ context.Context, input AnalyticsInput, winning, losing []Pattern) (Reasoning, error) {
	r := Reasoning{Reviewer: "deterministic"}
	var conf []float64
	for _, p := range winning {
		r.WhyWell = append(r.WhyWell, p.Name+": "+join(p.SignalChain))
		r.Evidence = append(r.Evidence, p.Evidence...)
		conf = append(conf, p.Confidence)
	}
	for _, p := range losing {
		r.WhyUnder = append(r.WhyUnder, p.Name+": "+join(p.SignalChain))
		r.Evidence = append(r.Evidence, p.Evidence...)
		r.Improvements = append(r.Improvements, "Address: "+p.Description)
		conf = append(conf, p.Confidence)
	}
	if len(winning) > 0 {
		r.Improvements = append(r.Improvements, "Replicate the winning signature in upcoming content.")
	}
	if len(r.WhyWell) == 0 && len(r.WhyUnder) == 0 {
		r.WhyWell = append(r.WhyWell, "Performance is broadly consistent; no dominant pattern separated top from bottom content.")
	}
	r.TradeOffs = append(r.TradeOffs, "Recommendations are grounded in historical data and may lag emerging shifts; validate with a small test batch.")
	r.Confidence = round2(mean(conf))
	r.ExpectedImpact = expectedImpact(r.Confidence)
	r.Evidence = dedupeStr(r.Evidence)
	return r, nil
}

// ModelReasoner is a ReasoningProvider backed by releasegen.Model (Bedrock/Anthropic).
// The prompt supplies the collected analytics + detected patterns and forbids
// inventing statistics. On any error or unparseable output it returns an error so
// the optimizer falls back to deterministic reasoning — it never fabricates.
type ModelReasoner struct {
	Model releasegen.Model
	Name  string
}

// NewModelReasoner wraps a Model as a ReasoningProvider.
func NewModelReasoner(m releasegen.Model, name string) *ModelReasoner {
	if name == "" {
		name = "model"
	}
	return &ModelReasoner{Model: m, Name: name}
}

// Reason asks the model for grounded reasoning and parses it.
func (r *ModelReasoner) Reason(ctx context.Context, input AnalyticsInput, winning, losing []Pattern) (Reasoning, error) {
	if r.Model == nil {
		return Reasoning{}, ErrReasoningUnavailable
	}
	out, err := r.Model.Generate(ctx, reasoningPrompt(input, winning, losing))
	if err != nil {
		return Reasoning{}, errReasoning("model generate failed", err)
	}
	reasoning, err := parseReasoning(out)
	if err != nil {
		return Reasoning{}, err
	}
	reasoning.Reviewer = r.Name
	if reasoning.ExpectedImpact == "" {
		reasoning.ExpectedImpact = expectedImpact(reasoning.Confidence)
	}
	return reasoning, nil
}

func reasoningPrompt(input AnalyticsInput, winning, losing []Pattern) string {
	payload, _ := json.Marshal(struct {
		AsOf      Date           `json:"asOf"`
		Winning   []Pattern      `json:"winningPatterns"`
		Losing    []Pattern      `json:"losingPatterns"`
		Platforms []PlatformStat `json:"platforms"`
	}{input.AsOf, winning, losing, input.Platforms})

	return fmt.Sprintf(
		"You are a content-performance analyst. Using ONLY the analytics JSON below (do NOT invent any numbers "+
			"or statistics not present), explain performance.\n\n"+
			"Respond ONLY with minified JSON: {\"whyPerformedWell\":[\"...\"],\"whyUnderperformed\":[\"...\"],"+
			"\"supportingEvidence\":[\"...\"],\"tradeOffs\":[\"...\"],\"suggestedImprovements\":[\"...\"],"+
			"\"confidence\":0.0,\"expectedImpact\":\"low|medium|high\"}. Ground every statement in the data.\n\nANALYTICS:\n%s",
		string(payload))
}

func parseReasoning(out string) (Reasoning, error) {
	s := strings.TrimSpace(out)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return Reasoning{}, errParse("no JSON object in model output")
	}
	var r Reasoning
	if err := json.Unmarshal([]byte(s[start:end+1]), &r); err != nil {
		return Reasoning{}, errParse("unmarshal model output: " + err.Error())
	}
	r.Confidence = clampFloat(round2(r.Confidence), 0, 1)
	return r, nil
}
