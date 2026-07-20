package seo

import (
	"context"
	"strings"
)

// planExcerpts builds 50/100/200-word excerpts from the blog prose (grounded).
// When a Model is configured, the ~50-word excerpt is polished for readability;
// the others are deterministic truncations. Falls back to truncation.
func (g *Generator) planExcerpts(ctx context.Context, pkg ReleasePackage) Excerpts {
	prose := blogProse(pkg)
	if strings.TrimSpace(prose) == "" {
		prose = summary(pkg)
	}

	short := truncateWords(prose, 50)
	if g.Model != nil && wordCount(prose) > 0 {
		if out, err := g.Model.Generate(ctx, excerptPrompt(prose)); err == nil {
			if r := collapse(strings.TrimSpace(out)); r != "" {
				short = truncateWords(r, 55)
			}
		}
	}

	return Excerpts{
		Short:  short,
		Medium: truncateWords(prose, 100),
		Long:   truncateWords(prose, 200),
	}
}

func excerptPrompt(prose string) string {
	return "Write a compelling ~50-word excerpt for a technical blog post, summarizing the content below for search " +
		"and social previews. Be accurate and specific; use ONLY the facts below — invent nothing. Output only the excerpt.\n\n" +
		truncateWords(prose, 220)
}
