package seo

import (
	"context"
	"fmt"
)

// planMetaDescription returns the blog meta description, bounded to the platform
// limit. It prefers the blog's existing meta description, optionally polished by
// the Model, and always falls back to a grounded summary.
func (g *Generator) planMetaDescription(ctx context.Context, pkg ReleasePackage) string {
	base := firstNonEmpty(pkg.Blog.MetaDescription, summary(pkg),
		firstSentences(featureName(pkg), 1))
	desc := base
	if g.Model != nil {
		if out, err := g.Model.Generate(ctx, metaDescPrompt(base)); err == nil {
			desc = sanitizeModelText(out, desc)
		}
	}
	if len(desc) > BlogDescMax {
		desc = truncateChars(desc, BlogDescMax)
	}
	return desc
}

func metaDescPrompt(base string) string {
	return fmt.Sprintf(
		"Write an SEO meta description (strictly 150–160 characters) for a technical blog post, based on the text "+
			"below. Accurate, specific, compelling. Use ONLY the facts below — invent nothing."+evergreenRule+" Output only the description.\n\n%s",
		base)
}

// ogDescription returns the Open Graph description (bounded to OGDescMax).
func ogDescription(pkg ReleasePackage, metaDesc string) string {
	d := firstNonEmpty(metaDesc, summary(pkg))
	if len(d) > OGDescMax {
		d = truncateChars(d, OGDescMax)
	}
	return d
}
