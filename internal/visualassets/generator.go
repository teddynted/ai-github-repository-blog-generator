package visualassets

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
)

// Generator converts a ReleasePackage into a collection of provider-neutral AI
// image prompts. It reuses the shared releasegen.Model port; when Model is nil,
// generation is fully deterministic (prompts are assembled from grounded
// specs). It never regenerates the upstream artifacts and never invents
// architecture.
type Generator struct {
	Model releasegen.Model
	// MaxAssets caps how many assets to produce; <= 0 uses the default (14).
	MaxAssets int
	Now       func() time.Time
	Logger    *slog.Logger
}

func (g *Generator) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now().UTC()
}

// VisualAssets converts a ReleasePackage into a VisualAssetCollection of image
// prompts for thumbnails, social graphics, blog headers, banners, and technical
// illustrations. Discovery, branding, styles, negative prompts, metadata, and
// validation are deterministic; the Model only polishes the creative prompt
// prose. Every prompt is grounded in the Release Context.
func (g *Generator) VisualAssets(ctx context.Context, pkg ReleasePackage) (VisualAssetCollection, error) {
	if pkg.Context == nil {
		return VisualAssetCollection{}, fmt.Errorf("visualassets: release context is required")
	}

	candidates := discover(pkg, g.MaxAssets)
	if len(candidates) == 0 {
		return VisualAssetCollection{}, fmt.Errorf("visualassets: no assets discovered for the release")
	}

	branding := planBranding(pkg)
	collection := VisualAssetCollection{
		SchemaVersion: SchemaVersion,
		Branding:      branding,
		Metadata: Metadata{
			Repository:      repoName(pkg),
			Release:         releaseTag(pkg),
			SourceBlogTitle: pkg.Blog.Title,
			GeneratedAt:     g.now().Format(time.RFC3339),
			SourceSchemas:   sourceSchemas(pkg),
		},
	}

	assets := make([]Asset, 0, len(candidates))
	for i, c := range candidates {
		assets = append(assets, g.buildAsset(ctx, pkg, c, branding, i))
	}

	collection.Assets = assets
	collection.Metadata.AssetCount = len(assets)
	collection.ContentIntelligence = g.planIntelligence(pkg, assets)
	collection.Warnings = collectWarnings(pkg, assets)

	if g.Logger != nil {
		g.Logger.Info("visual assets generated",
			slog.String("repository", collection.Metadata.Repository),
			slog.String("release", collection.Metadata.Release),
			slog.Int("assets", len(assets)),
			slog.Int("warnings", len(collection.Warnings)),
		)
	}
	return collection, nil
}

// buildAsset assembles one image-prompt asset from a discovered candidate.
func (g *Generator) buildAsset(ctx context.Context, pkg ReleasePackage, c candidate, b Branding, index int) Asset {
	style := styleFor(c, b)
	placeholders := textPlaceholders(c)
	promptText := g.prompt(ctx, c, style, b, placeholders)

	// SDXL tuning: parameters keyed to the asset category at its own dimensions,
	// and a composition archetype rotated per release so releases stay varied.
	w, h := parseDimensions(c.Dimensions, c.AspectRatio)

	return Asset{
		ID:                   index + 1,
		Type:                 c.Type,
		Platform:             c.Platform,
		AspectRatio:          c.AspectRatio,
		Dimensions:           c.Dimensions,
		Title:                c.Type,
		Purpose:              c.Purpose,
		Prompt:               promptText,
		NegativePrompt:       planNegative(c, pkg),
		Style:                style,
		Branding:             b,
		TextPlaceholders:     placeholders,
		References:           dedupe(c.References),
		Metadata:             planAssetMeta(c, pkg),
		SDXL:                 sdxlParamsFor(c.Type, w, h),
		CompositionArchetype: chooseArchetype(releaseTag(pkg), index),
	}
}

func sourceSchemas(pkg ReleasePackage) map[string]string {
	m := map[string]string{"releaseContext": pkg.Context.SchemaVersion}
	if pkg.Storyboard.SchemaVersion != "" {
		m["storyboard"] = pkg.Storyboard.SchemaVersion
	}
	if pkg.YouTube.SchemaVersion != "" {
		m["youTube"] = pkg.YouTube.SchemaVersion
	}
	if pkg.Shorts.SchemaVersion != "" {
		m["shorts"] = pkg.Shorts.SchemaVersion
	}
	if pkg.TikTok.SchemaVersion != "" {
		m["tikTok"] = pkg.TikTok.SchemaVersion
	}
	return m
}

// collectWarnings surfaces non-fatal concerns (e.g. no architecture to
// illustrate, so those assets were skipped).
func collectWarnings(pkg ReleasePackage, assets []Asset) []string {
	var w []string
	if archSubject(pkg) == "" {
		w = append(w, "no architecture overview or diagram in the release context; architecture illustrations were skipped")
	}
	if len(assets) == 0 {
		w = append(w, "no assets were produced")
	}
	return w
}
