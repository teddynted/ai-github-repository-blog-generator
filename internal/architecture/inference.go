package architecture

import "strings"

// inference is the grounded AI-inference detection for a release: which local and
// which cloud inference providers the Release Context actually names. It is the
// signal that lets the generator use hybrid-AI framing ONLY when both are present,
// never fabricating it for a non-AI repository.
type inference struct {
	Local []string // e.g. ["Ollama"]
	Cloud []string // e.g. ["Amazon Bedrock", "Anthropic Claude"]
}

// hybrid reports whether the release runs both local and cloud inference — the
// precondition for the "Hybrid AI" sections and terminology.
func (i inference) hybrid() bool { return len(i.Local) > 0 && len(i.Cloud) > 0 }

// kw is an ordered keyword → canonical-name mapping. A slice (not a map) keeps
// detection order deterministic, so the emitted lists are stable across runs.
type kw struct{ key, canonical string }

// localInferenceKeywords are runtimes that serve models on self-hosted hardware.
var localInferenceKeywords = []kw{
	{"ollama", "Ollama"},
	{"llama.cpp", "llama.cpp"},
	{"llamacpp", "llama.cpp"},
	{"vllm", "vLLM"},
	{"gpt4all", "GPT4All"},
	{"localai", "LocalAI"},
	{"lm studio", "LM Studio"},
}

// cloudInferenceKeywords are managed/hosted inference providers.
var cloudInferenceKeywords = []kw{
	{"bedrock", "Amazon Bedrock"},
	{"sagemaker", "Amazon SageMaker"},
	{"anthropic", "Anthropic Claude"},
	{"claude", "Anthropic Claude"},
	{"openai", "OpenAI"},
	{"gpt-4", "OpenAI GPT"},
	{"vertex ai", "Google Vertex AI"},
	{"gemini", "Google Gemini"},
}

// detectInference finds grounded inference providers from the release context:
// the resolved AWS services (e.g. Amazon Bedrock) and the detected technologies
// (e.g. Ollama, Anthropic Claude). It never adds a provider not named there.
func detectInference(pkg ReleasePackage, a analysis) inference {
	var inf inference
	seen := map[string]bool{}
	add := func(dst *[]string, tag, name string) {
		if k := tag + name; !seen[k] {
			seen[k] = true
			*dst = append(*dst, name)
		}
	}
	scan := func(text string) {
		lt := strings.ToLower(text)
		for _, k := range localInferenceKeywords {
			if strings.Contains(lt, k.key) {
				add(&inf.Local, "L:", k.canonical)
			}
		}
		for _, k := range cloudInferenceKeywords {
			if strings.Contains(lt, k.key) {
				add(&inf.Cloud, "C:", k.canonical)
			}
		}
	}

	// Resolved AWS services (deterministic order) — catches Bedrock/SageMaker.
	for _, s := range a.serviceLabels() {
		scan(s)
	}
	if pkg.Context != nil {
		// Detected technologies — catches Ollama, Anthropic Claude, etc.
		for _, t := range pkg.Context.Technologies {
			scan(t.Name)
		}
		// Architecture-scoped prose (grounded descriptions of THIS system).
		scan(pkg.Context.Architecture.Overview)
		scan(pkg.Context.Architecture.DeploymentTopology)
	}
	// Parsed Mermaid diagram nodes/edges — real architecture nodes the repository
	// authored (e.g. an "Amazon Bedrock" or "Ollama" node), the strongest signal.
	for _, d := range a.Mermaid {
		for _, n := range d.Nodes {
			scan(n)
		}
		for _, e := range d.Edges {
			scan(e.From)
			scan(e.To)
			scan(e.Label)
		}
	}
	// Grounded event-driven flow descriptions.
	for _, f := range a.Flows {
		scan(f)
	}
	return inf
}
