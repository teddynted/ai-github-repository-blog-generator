# Brand assets

Logo family for **AI GitHub Repository Blog Generator**. The mark is a Git branch
peeling off the commit trunk and blooming into an AI spark — software engineering
meeting automated content generation, in one flat, scalable symbol.

All assets are pure, hand-editable SVG on a transparent ground: no raster, no
gradients, no drop shadows. They work in monochrome and in light/dark themes.

## Files

| File | Use |
| --- | --- |
| `logo-horizontal.svg` | Primary lockup — light backgrounds |
| `logo-horizontal-dark.svg` | Primary lockup — dark backgrounds |
| `logo-horizontal-mono.svg` | Single-color lockup (`currentColor`) |
| `icon.svg` | Icon only — color, transparent |
| `icon-mono.svg` | Icon only — single color (`currentColor`) |
| `favicon.svg` | Favicon — 32×32 rounded tile |
| `social-square.svg` | Avatar / social — 512×512 rounded tile |

## Palette

| Token | Hex | Role |
| --- | --- | --- |
| GitHub Black | `#24292F` | Structure, ink, commit trunk |
| AI Blue | `#2563EB` | Primary — the spark |
| Purple | `#7C3AED` | Accent — alternate spark |
| Cyan | `#06B6D4` | Accent — secondary spark |

Dark-theme variants brighten the accents to `#3B82F6` / `#22D3EE` and the
structure to `#E6EDF3` for contrast on `#0D1117`.

## Usage in the README

The `-mono` builds use `fill="currentColor"`, so they inherit the surrounding
text color. For a logo that follows GitHub's light/dark automatically, use a
`<picture>` element:

```html
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/brand/logo-horizontal-dark.svg">
  <img alt="AI GitHub Repository Blog Generator" src="docs/assets/brand/logo-horizontal.svg" width="320">
</picture>
```

## Guidelines

- Keep clear space around the mark equal to the diameter of one commit node.
- Below ~120 px wide, use the icon alone rather than the full lockup.
- The wordmark uses a geometric sans stack (Poppins/Montserrat → system fallback).
  For pixel-perfect distribution, outline the text in Figma/Illustrator/Inkscape.
- Don't recolor the spark to a non-accent hue, add gradients, or apply shadows.
