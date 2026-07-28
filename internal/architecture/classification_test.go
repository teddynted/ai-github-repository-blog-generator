package architecture

import (
	"testing"

	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
)

func TestArchitectureStyleIsEvidenceDriven(t *testing.T) {
	cases := []struct {
		name string
		a    analysis
		want string
	}{
		{"event+hybrid", analysis{EventDriven: true, Inference: inference{Local: []string{"Ollama"}, Cloud: []string{"Amazon Bedrock"}}}, "Event-driven hybrid AI platform"},
		{"hybrid-no-event", analysis{Inference: inference{Local: []string{"Ollama"}, Cloud: []string{"Amazon Bedrock"}}}, "Hybrid AI platform"},
		{"ai-no-hybrid-no-event", analysis{Inference: inference{Cloud: []string{"Amazon Bedrock"}}}, "AI platform"},
		{"event-no-ai", analysis{EventDriven: true}, "Event-driven platform"},
		{"infra-automation", analysis{Templates: []string{"infra.yaml"}, Services: []serviceNode{{Label: "Amazon S3"}}}, "Cloud automation platform"},
		{"dev-tooling", analysis{Dirs: []rc.DirectoryInfo{{Path: "cmd"}}, Inference: inference{Cloud: []string{"Amazon Bedrock"}}}, "AI-enabled developer tooling platform"},
		{"plain", analysis{}, "Application"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := architectureStyle(tc.a); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
