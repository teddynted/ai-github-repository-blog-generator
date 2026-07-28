#!/usr/bin/env bash
# generate-content.sh — two-stage local content generation for a release.
#
# Stage 1: Claude (claude-code) builds the foundation blog.md — once.
# Stage 2: every downstream artifact derives from that blog via --from-blog,
#          with --hybrid auto-routing each kind (premium → claude-code,
#          transforms → Ollama).
#
# Defaults to v0.14.0; override with env vars:
#   RELEASE=v0.14.0                       release tag (folder + fixture name)
#   CONTEXT=fixtures/designing-<rel>.json Release Context fixture
#   OLLAMA_MODEL=llama3.2:1b              Ollama transform model
#   FORCE_BLOG=1                          regenerate blog even if it exists
#   FAST=1                                one `--artifact all` run instead of
#                                         per-artifact steps (storyboard once;
#                                         faster overall, less granular logging)
#
# Usage:
#   scripts/generate-content.sh
#   RELEASE=v0.11.0 scripts/generate-content.sh
#   FAST=1 scripts/generate-content.sh
set -euo pipefail

RELEASE="${RELEASE:-v0.14.0}"
CTX="${CONTEXT:-fixtures/designing-${RELEASE}.json}"
BLOG="output/releases/${RELEASE}/blog.md"
FORCE_BLOG="${FORCE_BLOG:-0}"
FAST="${FAST:-0}"
MODEL="${OLLAMA_MODEL:-llama3.2:1b}"

cd "$(git rev-parse --show-toplevel)"
[ -f "$CTX" ] || { printf '\033[1;31m✗ context not found: %s\033[0m\n' "$CTX" >&2; exit 1; }

say() { printf '\033[1;34m▶ %s\033[0m\n' "$*"; }
run() { say "cmd/content $*"; go run ./cmd/content "$@"; }

# --- Stage 1: foundation blog (Claude), once ---------------------------------
if [ "$FORCE_BLOG" = 1 ] || [ ! -s "$BLOG" ]; then
  say "Stage 1: blog (claude-code)"
  run --artifact blog --provider claude-code --no-history --context "$CTX"
else
  say "Stage 1: reusing existing $BLOG"
fi
[ -s "$BLOG" ] || { printf '\033[1;31m✗ blog not produced: %s\033[0m\n' "$BLOG" >&2; exit 1; }

# --- Stage 2: derive downstream from blog.md ---------------------------------
if [ "$FAST" = 1 ]; then
  say "Stage 2: all downstream in one run (storyboard generated once)"
  run --artifact all --from-blog "$BLOG" --hybrid --no-history --no-cache --context "$CTX"
else
  say "Stage 2a: premium artifacts (→ claude-code)"
  for a in architecture architecture-diagram-spec linkedin x-thread; do
    run --artifact "$a" --from-blog "$BLOG" --hybrid --no-history --context "$CTX"
  done
  # NOTE: the storyboard-derived transforms (voiceover, youtube, youtube-shorts,
  # tiktok, visual-assets) internally regenerate storyboard each per-artifact
  # run. Set FAST=1 to generate them together and avoid the repeat.
  say "Stage 2b: transforms (→ ollama:${MODEL}, --no-cache)"
  for a in seo-metadata storyboard voiceover youtube youtube-shorts tiktok visual-assets; do
    run --artifact "$a" --from-blog "$BLOG" --hybrid --no-history --no-cache --context "$CTX"
  done
fi

say "done → output/releases/${RELEASE}/"
ls -la "output/releases/${RELEASE}/" 2>/dev/null || true
