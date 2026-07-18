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

## 3. API (`POST /process`)

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
  "contextId": "b6c1…",
  "repository": "ai-github-repository-blog-generator",
  "releaseTag": "v0.2.0"
}
```

Validation errors return `400` with `{ "error": "invalid request: releaseTag is required" }`.
The route is API-key protected, consistent with the existing registration and
manual-trigger endpoints.

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

**Milestone 2 — Phase 1 (this change):** the Content Intelligence engine —
domain schema, all analyzers, the orchestrating `Builder`, request validation,
and the content-intelligence layer — implemented as pure, unit-tested Go
(85%+ statement coverage on core logic).

**Phase 2 (next):** the I/O layer — a GitHub REST + git adapter implementing
`Sources`, the `/process` Lambda handler, the API Gateway route and
CloudFormation, context persistence (S3/DynamoDB), and integration tests with
mocked GitHub responses.
