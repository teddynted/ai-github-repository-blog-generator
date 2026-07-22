# Test fixtures — sample release + goldens

Reusable, deterministic fixtures for exercising the full content pipeline without
a model or network. They drive the end-to-end test in
[`internal/e2e`](../internal/e2e/e2e_test.go).

## Contents

| File | Role | Kind |
|------|------|------|
| `sample-release-context.json` | Canonical **Release Context** (M2 output) for a fictional `acme/widget` v1.0.0 release — repo, release, notes, changelog, commits, architecture (AWS + CloudFormation), Mermaid, technologies. | Input |
| `sample-release-notes.md` | Raw GitHub release notes for the sample release. | Input |
| `sample-changelog.md` | Raw `CHANGELOG.md` for the sample release. | Input |
| `golden-blog.md` | Hand-authored technical blog (the blog stage needs a model, so it is supplied as a fixed input for the offline chain). | Input |
| `golden-storyboard.md` | Storyboard produced by the pipeline. | Golden output |
| `golden-youtube.md` | YouTube script produced by the pipeline. | Golden output |
| `golden-linkedin.md` | LinkedIn post produced by the pipeline. | Golden output |
| `golden-x-thread.md` | X thread produced by the pipeline. | Golden output |
| `golden-seo.json` | SEO metadata (structured). | Golden output |
| `golden-thumbnail.json` | Visual-asset / thumbnail metadata (structured). | Golden output |

The Markdown goldens are the orchestrator's **own** canonical output, so the
regression test compares byte-for-byte. The JSON goldens carry a normalized
`generatedAt` sentinel (`0001-01-01T00:00:00Z`) so they stay stable in git.

## How the fixtures are used

- **Smoke** — `TestSampleReleaseNotesAndChangelogPresent`: the raw inputs exist and describe the release.
- **Integration** — `TestEndToEndFromSampleRelease`: the whole suite runs on the sample context and produces all 11 stages in canonical order with no failures; each downstream stage consumes its upstream inputs.
- **Acceptance** — grounding checks: every artifact references the actual release (`Widget`, `v1.0.0`, `EventBridge`, `SQS`); the JSON goldens are valid and grounded.
- **Regression** — the Markdown artifacts must byte-match the committed goldens; drift fails the test.

Run them with:

```bash
go test ./internal/e2e/
```

## Regenerating the goldens

If a generator's output changes intentionally, regenerate the goldens and review
the diff before committing:

```bash
# Markdown goldens (from the orchestrator itself — guarantees a byte-match):
go run ./cmd/generate-all --context testdata/sample-release-context.json \
  --blog testdata/golden-blog.md --offline --out /tmp/gold
cp /tmp/gold/02-storyboard.md     testdata/golden-storyboard.md
cp /tmp/gold/04-youtube-script.md testdata/golden-youtube.md
cp /tmp/gold/10-linkedin.md       testdata/golden-linkedin.md
cp /tmp/gold/11-x-thread.md       testdata/golden-x-thread.md

# JSON goldens (structured), then pin the volatile timestamp:
go run ./cmd/seo          --context testdata/sample-release-context.json --blog testdata/golden-blog.md --offline --format json --out testdata/golden-seo.json
go run ./cmd/visualassets --context testdata/sample-release-context.json --blog testdata/golden-blog.md --offline --format json --out testdata/golden-thumbnail.json
sed -i '' -E 's/"generatedAt": *"[^"]*"/"generatedAt": "0001-01-01T00:00:00Z"/' testdata/golden-*.json
```

Only regenerate when a change is intended — an unexpected diff is the regression
test doing its job.
