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
	"unicode/utf8"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/apperror"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

// DefaultMaxPromptBytes bounds prompt size so a large repository cannot overflow
// the local model's context window. ~24 KB is a conservative budget (roughly a
// few thousand tokens).
const DefaultMaxPromptBytes = 24000

const truncationMarker = "\n\n[context truncated to fit the model budget]\n"

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
	Model Model
	// MaxPromptBytes bounds each prompt; <= 0 uses DefaultMaxPromptBytes.
	MaxPromptBytes int
	Logger         *slog.Logger
}

// promptSpec holds the varying parts of a prompt for a content kind.
type promptSpec struct {
	role        string
	instruction string
	closing     string
}

var promptSpecs = map[Kind]promptSpec{
	KindBlog: {
		role:        "You are a senior software engineer writing a technical blog post.",
		instruction: "Write a clear, accurate, publication-ready blog post in GitHub-flavoured Markdown about the repository below. Use only the provided material; do not invent facts.",
		closing:     "Produce only the Markdown blog post, starting with a top-level title.",
	},
	KindReadme: {
		role:        "You are an experienced open-source maintainer improving a project's README.",
		instruction: "Suggest concrete improvements to the repository's README as GitHub-flavoured Markdown. Base every suggestion on the provided material.",
		closing:     "Produce a Markdown document of prioritised, actionable README improvements.",
	},
	KindDocs: {
		role:        "You are a technical writer producing project documentation.",
		instruction: "Write clear project documentation in GitHub-flavoured Markdown from the provided material. Do not invent features.",
		closing:     "Produce a Markdown documentation page.",
	},
	KindArchitecture: {
		role:        "You are a software architect summarising a system.",
		instruction: "Write a concise architecture summary in GitHub-flavoured Markdown from the provided material, covering components and how they fit together.",
		closing:     "Produce a Markdown architecture summary.",
	},
	KindReleaseNotes: {
		role:        "You are preparing release notes for a software project.",
		instruction: "Write release notes in GitHub-flavoured Markdown based on the recent commits and repository material below. Group related changes.",
		closing:     "Produce Markdown release notes.",
	},
}

// Generate produces a single content asset of the given kind.
func (g *Generator) Generate(ctx context.Context, kind Kind, snap processing.Snapshot) (Content, error) {
	spec, ok := promptSpecs[kind]
	if !ok {
		return Content{}, apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("unknown content kind %q", kind))
	}
	if snap.RepoFullName == "" {
		return Content{}, apperror.New(apperror.CodeInvalidInput, "snapshot is missing a repository")
	}

	out, err := g.Model.Generate(ctx, g.buildPrompt(spec, snap))
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

// buildPrompt assembles a deterministic prompt (role, instruction, repository
// context, closing) and enforces the prompt-size budget by truncating only the
// repository context — the instructions and closing directive are preserved.
func (g *Generator) buildPrompt(spec promptSpec, snap processing.Snapshot) string {
	max := g.MaxPromptBytes
	if max <= 0 {
		max = DefaultMaxPromptBytes
	}
	head := spec.role + "\n" + spec.instruction + "\n\n"
	tail := "\n" + spec.closing + "\n"

	body := repoContext(snap)
	budget := max - len(head) - len(tail)
	if budget < len(truncationMarker) {
		budget = len(truncationMarker) // never negative; degrade gracefully
	}
	if len(body) > budget {
		body = safeTruncate(body, budget-len(truncationMarker)) + truncationMarker
	}
	return head + body + tail
}

// safeTruncate cuts s to at most max bytes without splitting a UTF-8 rune.
func safeTruncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
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
