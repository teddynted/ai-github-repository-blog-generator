# Generated-Content Storage Architecture

How AI-generated artifacts are stored in S3 for long-term lifecycle management:
immutable history, provenance, rollback, comparison, and future artifact types.

## Bucket

`s3://<project>-content-<account>-<region>/` (created in `infrastructure/bootstrap.yaml`,
`DeletionPolicy: Retain`). The worker publishes under the `generated-content/` prefix.

Security posture:

- **Versioning: enabled** — every generation is immutable; re-running a release tag
  never destroys prior output, it creates a new object version.
- **SSE-KMS** with a customer-managed key (`alias/<project>-content`) and S3 Bucket
  Keys enabled. The worker is granted `kms:GenerateDataKey` scoped to the S3 service
  (`kms:ViaService`), not the key ARN, so there is no cross-stack coupling.
- **TLS-only** bucket policy (`DenyInsecureTransport`).
- **Public access fully blocked.**
- **Lifecycle:** abort incomplete multipart uploads after 7 days; expire noncurrent
  versions after 365 days while always retaining the 20 most recent generations.

## Layout

Shallow and release-centric. Multiple generations of an artifact are S3 object
versions of one stable key — not `vN.md` files (which would sprawl keys and
duplicate what versioning does natively and atomically).

```
generated-content/{owner}/{repo}/
├── releases/{tag}/
│   ├── blog.md                      # current generation (S3-versioned)
│   ├── architecture.md … x-thread.md
│   ├── storyboard.md … seo-metadata.md
│   ├── architecture-diagram-spec.md
│   ├── architecture-diagram.svg
│   ├── metadata.json                # generation manifest (Phase 2)
│   └── experiments/{experimentId}/  # optional A/B (prompt/provider) siblings
├── latest/                          # newest APPROVED release (Phase 3)
│   ├── blog.md …
│   └── latest.json
└── index.json                       # optional releases index
```

New artifact types (video/mp4, thumbnail/png, narration/wav, slides/pptx,
eval-report/json) are just new files in the same folder — no layout change.

## Versioning strategy — hybrid

- **S3 Versioning** is the immutable backbone (history, atomic writes, restore).
- **`metadata.json`** gives opaque version IDs *meaning* (which prompt/provider/model
  produced each generation).
- **`experiments/{experimentId}/`** is the one place application-level naming is used,
  for deliberate side-by-side A/B runs that linear versioning can't express.

## Metadata (`metadata.json`, Phase 2)

Only semantics S3 does not already store (omit size/ETag/Last-Modified/Content-Type):

```json
{
  "schemaVersion": "1.0.0",
  "generationId": "<ULID>",
  "owner": "…", "repository": "…", "releaseTag": "…",
  "gitCommit": "<release target SHA>",
  "generatedAt": "…", "workflowRunId": "…",
  "generatorVersion": "<worker commit SHA>",
  "artifacts": {
    "blog.md": { "provider":"claude","model":"…","promptVersion":"blog@6","s3VersionId":"…","sha256":"…","status":"ok" }
  }
}
```

## Rollback strategy

- **Single artifact:** `CopyObject` a prior `versionId` back onto the current key.
- **Entire release:** copy each artifact's recorded `versionId` back, or re-point `latest/`.
- **Compare / history:** `ListObjectVersions` (or read a historical `metadata.json`) → `GetObject` by version.
- Exposed via the `content-admin` CLI (`cmd/content-admin`). The **worker stays
  write-only**; reads and rollback run under the separate `content-operator` IAM
  role (defense in depth):

```bash
content-admin history        --repo owner/name --release v0.3.0 --artifact blog.md
content-admin compare        --repo owner/name --release v0.3.0 --artifact blog.md --a <ver> --b <ver>
content-admin rollback       --repo owner/name --release v0.3.0 --artifact blog.md --to <ver>
content-admin promote-latest --repo owner/name --release v0.3.0
```

Rollback is non-destructive: it server-side-copies a prior version back onto the
live key, creating a new current version equal to the old one.

## Roadmap

| Phase | Scope | Status |
| --- | --- | --- |
| 1 | Infra: versioning, SSE-KMS, lifecycle, TLS-only policy, worker KMS IAM | **done** |
| 2 | Publish `metadata.json` with provenance (provider/model/promptVersion/sha256/versionId) | **done** |
| 3 | `latest/` promotion of approved releases (content + metadata + `latest.json`) | **done** |
| 4 | Operator CLI (`content-admin`) + `content-operator` IAM role | **done** |
| 5 | `experiments/` namespace + prompt-version registry | planned |

Existing `releases/{tag}/{kind}.{ext}` keys are unchanged throughout, so downstream
consumers keep working and only gain new capabilities.
