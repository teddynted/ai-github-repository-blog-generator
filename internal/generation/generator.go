// Package generation turns a repository processing.Snapshot into technical
// content using a local LLM. This is the first slice of the post-MVP content
// generation phase: a single technical blog post. Additional outputs (README
// improvements, docs, architecture summaries, ...) and Repository Memory /
// quality review are future work; the Model port keeps the LLM swappable.
package generation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

// Model is the local-inference port. *ollama.Client satisfies it.
type Model interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Content is a generated asset (Markdown).
type Content struct {
	Kind     string `json:"kind"` // e.g. "blog"
	Markdown string `json:"markdown"`
}

// Generator produces content from a Snapshot via the local model.
type Generator struct {
	Model  Model
	Logger *slog.Logger
}

// BlogPost generates a technical blog post from the snapshot.
func (g *Generator) BlogPost(ctx context.Context, snap processing.Snapshot) (Content, error) {
	if snap.RepoFullName == "" {
		return Content{}, apperror.New(apperror.CodeInvalidInput, "snapshot is missing a repository")
	}
	prompt := buildBlogPrompt(snap)

	out, err := g.Model.Generate(ctx, prompt)
	if err != nil {
		return Content{}, fmt.Errorf("generate blog post: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return Content{}, apperror.New(apperror.CodeUpstream, "model returned empty content")
	}
	if g.Logger != nil {
		g.Logger.Info("generated content", slog.String("kind", "blog"), slog.String("repo", snap.RepoFullName))
	}
	return Content{Kind: "blog", Markdown: strings.TrimSpace(out)}, nil
}

// buildBlogPrompt assembles a deterministic prompt from the snapshot. It is
// pure and unit-tested so prompt content is verifiable without a model.
func buildBlogPrompt(snap processing.Snapshot) string {
	var b strings.Builder
	b.WriteString("You are a senior software engineer writing a technical blog post.\n")
	b.WriteString("Write a clear, accurate, publication-ready blog post in GitHub-flavoured Markdown ")
	b.WriteString("about the repository below. Use only the provided material; do not invent facts.\n\n")

	fmt.Fprintf(&b, "Repository: %s\n", snap.RepoFullName)
	if snap.Ref != "" {
		fmt.Fprintf(&b, "Ref: %s\n", snap.Ref)
	}

	if strings.TrimSpace(snap.Readme) != "" {
		b.WriteString("\n## README\n")
		b.WriteString(snap.Readme)
		b.WriteString("\n")
	}

	if len(snap.Docs) > 0 {
		b.WriteString("\n## Documentation excerpts\n")
		for _, d := range snap.Docs {
			fmt.Fprintf(&b, "\n### %s\n%s\n", d.Path, d.Content)
		}
	}

	if len(snap.Commits) > 0 {
		b.WriteString("\n## Recent commits\n")
		for _, c := range snap.Commits {
			fmt.Fprintf(&b, "- %s %s\n", shortSHA(c.SHA), firstLine(c.Message))
		}
	}

	b.WriteString("\nProduce only the Markdown blog post, starting with a top-level title.\n")
	return b.String()
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
