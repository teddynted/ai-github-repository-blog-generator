// Package generation turns a repository processing.Snapshot into technical
// content using a local LLM. It produces multiple documented output types (blog
// post, README improvements, documentation, architecture summary, release
// notes) via the Model port, which keeps the LLM swappable. Repository Memory,
// quality review, optional approval, and publishing remain future work.
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

// Kind identifies a content output type.
type Kind string

const (
	KindBlog         Kind = "blog"
	KindReadme       Kind = "readme-improvements"
	KindDocs         Kind = "documentation"
	KindArchitecture Kind = "architecture-summary"
	KindReleaseNotes Kind = "release-notes"
)

// Content is a generated asset (Markdown).
type Content struct {
	Kind     Kind   `json:"kind"`
	Markdown string `json:"markdown"`
}

// Generator produces content from a Snapshot via the local model.
type Generator struct {
	Model  Model
	Logger *slog.Logger
}

// promptBuilders maps each output kind to its (pure) prompt builder.
var promptBuilders = map[Kind]func(processing.Snapshot) string{
	KindBlog:         buildBlogPrompt,
	KindReadme:       buildReadmePrompt,
	KindDocs:         buildDocsPrompt,
	KindArchitecture: buildArchitecturePrompt,
	KindReleaseNotes: buildReleaseNotesPrompt,
}

// Generate produces a single content asset of the given kind.
func (g *Generator) Generate(ctx context.Context, kind Kind, snap processing.Snapshot) (Content, error) {
	build, ok := promptBuilders[kind]
	if !ok {
		return Content{}, apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("unknown content kind %q", kind))
	}
	if snap.RepoFullName == "" {
		return Content{}, apperror.New(apperror.CodeInvalidInput, "snapshot is missing a repository")
	}

	out, err := g.Model.Generate(ctx, build(snap))
	if err != nil {
		return Content{}, fmt.Errorf("generate %s: %w", kind, err)
	}
	if strings.TrimSpace(out) == "" {
		return Content{}, apperror.New(apperror.CodeUpstream, fmt.Sprintf("model returned empty content for %s", kind))
	}
	if g.Logger != nil {
		g.Logger.Info("generated content", slog.String("kind", string(kind)), slog.String("repo", snap.RepoFullName))
	}
	return Content{Kind: kind, Markdown: strings.TrimSpace(out)}, nil
}

// BlogPost is a convenience wrapper for the most common output.
func (g *Generator) BlogPost(ctx context.Context, snap processing.Snapshot) (Content, error) {
	return g.Generate(ctx, KindBlog, snap)
}

// GenerateAll produces every requested kind. A failure on one kind does not
// abort the others: successful assets are returned, and a non-nil error
// aggregates the failures (so a partial package is never corrupted).
func (g *Generator) GenerateAll(ctx context.Context, snap processing.Snapshot, kinds ...Kind) ([]Content, error) {
	var out []Content
	var failed []string
	for _, k := range kinds {
		c, err := g.Generate(ctx, k, snap)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", k, err))
			continue
		}
		out = append(out, c)
	}
	if len(failed) > 0 {
		return out, fmt.Errorf("generation failed for %d kind(s): %s", len(failed), strings.Join(failed, "; "))
	}
	return out, nil
}

// --- prompt builders (pure, unit-tested) ---

func buildBlogPrompt(snap processing.Snapshot) string {
	return promptWith(
		"You are a senior software engineer writing a technical blog post.",
		"Write a clear, accurate, publication-ready blog post in GitHub-flavoured Markdown about the repository below. Use only the provided material; do not invent facts.",
		snap,
		"Produce only the Markdown blog post, starting with a top-level title.",
	)
}

func buildReadmePrompt(snap processing.Snapshot) string {
	return promptWith(
		"You are an experienced open-source maintainer improving a project's README.",
		"Suggest concrete improvements to the repository's README as GitHub-flavoured Markdown. Base every suggestion on the provided material.",
		snap,
		"Produce a Markdown document of prioritised, actionable README improvements.",
	)
}

func buildDocsPrompt(snap processing.Snapshot) string {
	return promptWith(
		"You are a technical writer producing project documentation.",
		"Write clear project documentation in GitHub-flavoured Markdown from the provided material. Do not invent features.",
		snap,
		"Produce a Markdown documentation page.",
	)
}

func buildArchitecturePrompt(snap processing.Snapshot) string {
	return promptWith(
		"You are a software architect summarising a system.",
		"Write a concise architecture summary in GitHub-flavoured Markdown from the provided material, covering components and how they fit together.",
		snap,
		"Produce a Markdown architecture summary.",
	)
}

func buildReleaseNotesPrompt(snap processing.Snapshot) string {
	return promptWith(
		"You are preparing release notes for a software project.",
		"Write release notes in GitHub-flavoured Markdown based on the recent commits and repository material below. Group related changes.",
		snap,
		"Produce Markdown release notes.",
	)
}

// promptWith assembles a deterministic prompt: a role, an instruction, the
// shared repository context, and a closing directive.
func promptWith(role, instruction string, snap processing.Snapshot, closing string) string {
	var b strings.Builder
	b.WriteString(role)
	b.WriteString("\n")
	b.WriteString(instruction)
	b.WriteString("\n\n")
	b.WriteString(repoContext(snap))
	b.WriteString("\n")
	b.WriteString(closing)
	b.WriteString("\n")
	return b.String()
}

// repoContext renders the snapshot material shared by every prompt.
func repoContext(snap processing.Snapshot) string {
	var b strings.Builder
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
