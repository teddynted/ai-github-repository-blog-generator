package publish

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/generation"
)

func TestLogPublisherLogsAssets(t *testing.T) {
	var buf bytes.Buffer
	p := &LogPublisher{Logger: slog.New(slog.NewJSONHandler(&buf, nil))}

	err := p.Publish(context.Background(), "acme/widget", []generation.Content{
		{Kind: generation.KindBlog, Markdown: "# post"},
		{Kind: generation.KindReadme, Markdown: "readme"},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "acme/widget") || !strings.Contains(out, "blog") || !strings.Contains(out, "readme-improvements") {
		t.Errorf("log output missing expected fields: %s", out)
	}
	// Placeholder must not dump full content bodies at info level.
	if strings.Contains(out, "# post") {
		t.Error("publisher should not log full markdown content")
	}
}

func TestLogPublisherNilLoggerIsSafe(t *testing.T) {
	if err := (&LogPublisher{}).Publish(context.Background(), "a/b", nil); err != nil {
		t.Errorf("nil logger should be safe: %v", err)
	}
}
