# Release Context (Milestone 2)

The **Release Context** is the platform's Content Intelligence artifact: a single,
structured, versioned JSON document that describes a repository *at one release* —
**what changed, how it was implemented, why it matters, and how it fits the
architecture**. It is the canonical input for every downstream AI content
milestone (blog posts, release summaries, docs, LinkedIn/Shorts/TikTok, SEO).

Related: [Architecture](./architecture.md) · [Requirements](./requirements.md) · [Workflows](./workflows.md).

---

## 1. Design

The engine lives in [`internal/releasecontext`](../internal/releasecontext) and is
**pure domain logic** — no AWS, no network, no filesystem. It depends on one
port, `Sources`, which supplies already-fetched raw material (repository
metadata, the release, commits, changed files, and the file inventory). This
keeps every analyzer independently unit-testable and the whole engine reusable
from a Lambda, a CLI, or a test.

```
Request (owner, repository, releaseTag)
        │
        ▼
Builder ──uses──► Sources (port; GitHub REST + git adapter in the I/O layer)
        │
        ├─ Repository / Release          (metadata + summaries)
        ├─ Documentation                 (README, docs, ADRs → purpose, patterns)
        ├─ RepositoryStructure           (directory responsibilities, layout)
        ├─ Commits + CommitStats         (categorized; merges/bumps/format ignored)
        ├─ ChangedFiles + FileStats      (categorized by area)
        ├─ Technologies                  (languages, AWS services, tools)
        ├─ CloudFormation                (resources, params, outputs, services)
        ├─ Mermaid                       (type, nodes, edges)
        ├─ Changelog                     (features / fixes / breaking / …)
        ├─ Architecture                  (components, flows, topology, security)
        ├─ Implementation                (what shipped, why it matters)
        └─ ContentIntelligence           (summary, SEO, blog ideas, audience, …)
        │
        ▼
ReleaseContext (JSON, schemaVersion 2.0.0)
```

### Extensibility & versioning

- `SchemaVersion` follows SemVer and is **additive-only**. New milestones add
  fields; consumers pinned to a major version keep working.
- Each analyzer is a small, independent function over the raw inputs — add a new
  one (or a new content-intelligence field) without touching the others.
- The `Sources` port decouples analysis from I/O: the GitHub/git adapter,
  persistence, and the Lambda are swappable without changing the engine.

---

## 2. Top-level schema

| Field | Type | Description |
| --- | --- | --- |
| `schemaVersion` | string | Schema version (`2.0.0`). |
| `contextId` | string | UUIDv4 for this build. |
| `generatedAt` | string | RFC 3339 timestamp. |
| `repository` | object | Metadata + derived summary. |
| `release` | object | The selected GitHub Release + summary. |
| `documentation` | object | Purpose, patterns, goals, decisions, per-doc refs. |
| `repositoryStructure` | object | Directory responsibilities, layout, file count. |
| `architecture` | object | Components, AWS services, flows, topology, security. |
| `commits` | array | Categorized commits (previous → selected release). |
| `commitStats` | object | Totals, by-category counts, contributors. |
| `changedFiles` | array | Files changed for the release, categorized. |
| `fileStats` | object | Additions/deletions and by-category rollup. |
| `technologies` | array | Detected tech inventory (name, category, confidence). |
| `cloudformation` | object | Resources, parameters, outputs, service counts. |
| `mermaid` | array | Parsed diagrams (type, nodes, edges). |
| `changelog` | object | Parsed CHANGELOG section for the release. |
| `implementation` | object | What changed, how it works, why it matters. |
| `contentIntelligence` | object | AI-ready metadata (see below). |
| `warnings` | array | Non-fatal analyzer notices (e.g. missing CHANGELOG). |

### `contentIntelligence`

`summary`, `technicalHighlights`, `infrastructureHighlights`,
`architectureHighlights`, `implementationComplexity` (`low`/`medium`/`high`),
`developerValue`, `businessValue`, `targetAudience`, `seoKeywords`,
`blogTitles`, `articleOutline`, `linkedInPost`, `youTubeShortsTopic`,
`tikTokTopic`, `documentationUpdates`, `futureEnhancements`.

The content-intelligence layer is **deterministic and heuristic** — a strong,
structured substrate a later LLM milestone (Amazon Bedrock) refines, not a
replacement for one.

---

## 3. API (`POST /release-context`)

**This endpoint is a standalone, on-demand "context API": it builds the Release
Context and persists it to S3, and does not generate content.** Nothing in the
platform calls it automatically — it is invoked by an operator, a script, CI, or
external tooling that wants the structured context for a repo + release.

> The **automatic** pipeline does **not** go through this endpoint. A published
> release (or `POST /process` with a `releaseTag`) builds the Release Context
> **in-process in the worker** — the endpoint and the worker share the same
> `releasecontext.Builder`, but neither calls the other. Generation runs on the
> instance where the local model lives, which a Lambda cannot reach; hence the
> split (endpoint = context only; worker = context + generation + publish).

> **Route naming:** the milestone brief shows `POST /process`, but that route is
> already the platform's manual pipeline trigger. To keep both entry points, the
> Content Intelligence endpoint is **`POST /release-context`**. It is API-key
> protected (`x-api-key`) on the same usage plan as `/repositories` and
> `/process`.

Request:

```json
{
  "owner": "teddynted",
  "repository": "ai-github-repository-blog-generator",
  "releaseTag": "v0.2.0"
}
```

Accepted response (`202`):

```json
{
  "status": "accepted",
  "contextId": "b6c1a2f0-…",
  "repository": "ai-github-repository-blog-generator",
  "releaseTag": "v0.2.0",
  "location": "s3://blog-gen-artifacts-…/release-contexts/2026/07/20/b6c1….json",
  "warnings": 1
}
```

The built context is persisted to S3 (`release-contexts/<date>/<id>.json`) when
`CONTEXT_BUCKET` is set. Error mapping: `400` invalid request, `404`
repository/release not found, `403` not authorized, `502` GitHub unavailable,
`500` otherwise.

### Handler → engine wiring

```
API Gateway (POST /release-context, x-api-key)
      │
      ▼
Lambda: release-context  ──►  releasesource.GitHubSource (implements Sources)
      │                              │  GitHub REST: repo, release, compare, tree, contents
      ▼                              ▼
releasecontext.Builder.Build ──► ReleaseContext (JSON) ──► S3 (optional)
```

---

## 4. Example `ReleaseContext` (abridged)

```json
{
  "schemaVersion": "2.0.0",
  "contextId": "b6c1a2f0-…",
  "generatedAt": "2026-07-20T12:00:00Z",
  "repository": { "fullName": "teddynted/ai-github-repository-blog-generator", "language": "Go", "summary": "…" },
  "release": { "tag": "v0.2.0", "previousTag": "v0.1.0", "summary": "…" },
  "commitStats": { "analyzed": 4, "byCategory": { "Features": 2, "Bug Fixes": 1, "Documentation": 1 } },
  "cloudformation": { "counts": { "resources": 4, "serverless": 1, "iam": 1, "messaging": 2 }, "services": ["Events","IAM","Lambda","SQS"] },
  "mermaid": [ { "type": "flowchart", "nodeCount": 3, "edgeCount": 2 } ],
  "architecture": { "awsServices": ["AWS Lambda","Amazon SQS","Amazon EventBridge"], "eventDrivenFlows": ["API Gateway/Webhook → EventBridge → SQS → worker"] },
  "contentIntelligence": {
    "summary": "…",
    "seoKeywords": ["go","aws","serverless","event-driven architecture"],
    "blogTitles": ["Inside … v0.2.0: What Changed and Why It Matters"],
    "targetAudience": "Software engineers, cloud/DevOps engineers, …"
  }
}
```

---

## 5. Status

**Milestone 2 — implemented:**

- **Content Intelligence engine** ([`internal/releasecontext`](../internal/releasecontext)) — versioned schema, all analyzers, the orchestrating `Builder`, request validation, and the content-intelligence layer. Pure, unit-tested Go (85%+ coverage).
- **GitHub adapter** ([`internal/releasesource`](../internal/releasesource)) — implements `Sources` over the GitHub REST client (repository, release, previous-tag, compare, tree, contents), with a memoized compare and curated content fetching.
- **GitHub client** ([`internal/github`](../internal/github)) — `GetRepositoryDetail`, `GetReleaseByTag`, `ListReleases`, `CompareCommits`, `GetTree`, `GetFileContent` (integration-tested with mocked responses).
- **Lambda + API Gateway + CloudFormation** — `POST /release-context` (API-key protected), the `release-context` Lambda, least-privilege IAM, a CloudWatch log group, and optional S3 persistence (all in [`infrastructure/serverless.yaml`](../infrastructure/serverless.yaml)).

**Future milestones:** generate content (blog/social/SEO) from the context via
Amazon Bedrock; diff successive contexts; a webhook path that builds a context
automatically on release publication.
