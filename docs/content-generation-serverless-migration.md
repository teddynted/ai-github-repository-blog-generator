# Migration Plan: Move Content Generation off EC2 onto Step Functions + Fargate

**Status:** Phase A implemented (behind flags, not yet cut over) · **Date:** 2026-08-03

## Implementation status

Phase A is built and wired so the cutover is a **config flip**, not a code change:

- `internal/releaseapp` — shared release-pipeline builder (single source of truth).
- `cmd/content-runner` + `containers/content-runner/Dockerfile` — the one-shot Fargate task.
- `infrastructure/content.yaml` — the `GenerateContent` state machine (`ecs:runTask.sync` →
  best-effort `startExecution` of `blog-gen-video`), ECR repo, Fargate task (2 vCPU / 4 GB, arm64),
  IAM. It reads the same `{"detail": <event>}` envelope the manual trigger already emits.
- `manual-trigger` — starts `GenerateContent` when `CONTENT_STATE_MACHINE_ARN` is set, else the
  EC2 orchestration machine (default). `serverless.yaml` exposes it as a parameter; the manual
  trigger role can start both machines.
- `deploy.yml` — deploys the content stack + pushes the runner image behind `ENABLE_SERVERLESS_CONTENT`;
  flips `/process` to the serverless path behind `CUTOVER_CONTENT`.

**Remaining (post-verification):** deploy behind `ENABLE_SERVERLESS_CONTENT=true`, run the v0.6.0
parity check, set `CUTOVER_CONTENT=true`, then retire the worker release path + orchestration/SQS
and downsize the EC2 to `t4g.medium`.

## Goal

Move the heavy work — the 13-artifact content suite **and** video rendering — off the
long-running EC2 worker and onto **Step Functions + ECS Fargate**, so the EC2 box shrinks
to a lean, always-cheap **orchestration/publishing plane** running only **n8n + PostgreSQL +
Redis**. Generation becomes fully on-demand, pay-per-use, and serverless — no instance to
size, power-window, idle-stop, or boot-debug.

Video rendering already works this way (`blog-gen-video` state machine → Fargate). This plan
brings **text/artifact generation** onto the same pattern.

## Current architecture (what runs where today)

```
POST /process
  → manual-trigger (λ) → blog-gen-orchestration (Step Functions):
        FindInstance → StartInstance → WaitForBoot ⇄ CheckSSM → EnqueueJob   (~30s, wake+enqueue only)
  → SQS: blog-gen-events
  → blog-gen-worker (Go systemd service ON THE EC2 BOX)  ◄── ALL generation happens here (~10 min):
        releasepipeline.Pipeline.Run(owner, repo, tag):
          1. Build ReleaseContext (clone/inspect repo, changed files)
          2. Analyze (engineering context)
          3. contentsuite.Orchestrator  → the 13-artifact DAG (below)
          4. Publish → S3 (internal/artifactstore, internal/publish)
          5. [decorator] StartExecution → blog-gen-video (Fargate render)
```

The EC2 box therefore runs **both** the generation pipeline **and** the n8n/PostgreSQL/Redis
Docker stack — which is why it needs real RAM (currently `t4g.large`, 8 GB).

### The content-suite dependency DAG (the crux)

`internal/contentsuite/suite.go` (`stageDeps`) — the suite is **mostly one long linear chain**,
not 13 independent jobs:

```
blog (root — needs ReleaseContext + EngineeringContext)
 ├─ storyboard → voiceover → youtube → youtube-shorts → tiktok → visual-assets → seo-metadata
 └─ architecture
        ├─ linkedin                    (also needs seo-metadata)
        ├─ x-thread                    (also needs seo-metadata)
        └─ architecture-diagram-spec   (also needs blog)
```

Critical path ≈ **10 sequential model calls**: context → blog → storyboard → voiceover →
youtube → youtube-shorts → tiktok → visual-assets → seo-metadata → {linkedin, x-thread}.
The only real parallelism is `architecture` running alongside the storyboard→…→seo chain.

**Implication:** a naive "Map/fan-out over 13 types" does **not** fit — the stages are chained.
This is the single most important input to the strategy below.

### Assets that make this migration cheap

`contentsuite.Orchestrator` is already built for this:
- **Pure** — `Run` returns a `Suite`; the caller writes artifacts (no file I/O baked in).
- **Fault-tolerant per stage** — a failed stage is recorded in the manifest and the run
  continues; one weak input never aborts the pipeline.
- **`Only` + transitive deps** — restrict a run to named stages plus their dependencies.
- **`Store` + `Reuse`** — satisfy a *dependency* stage from a previously persisted artifact
  instead of regenerating it (already used by the local `--artifact` workflow).

Those last two are exactly the primitives needed to run stages as separate tasks with S3
handoffs — but, given the linear DAG, we don't need them for Phase A.

## Target architecture

```
POST /process → manual-trigger (λ) → GenerateContent (NEW Step Functions):
      BuildAndGenerate   ecs:runTask.sync   (ONE Fargate task: context → analyze → suite → publish-to-S3)
      → StartExecution: blog-gen-video      (existing video SM; Fargate render)
      → PublishManifest (λ)  → SNS/n8n notify

EC2 box  ──►  n8n + PostgreSQL + Redis ONLY  (t4g.medium, always-on-cheap or scheduled)
```

- **No SQS, no worker service, no instance wake-gate** for generation — Step Functions calls
  Fargate directly and waits (`runTask.sync`).
- The EC2 box no longer participates in generation at all; it only hosts the publishing plane.

## Strategy — two phases

### Phase A — Lift-and-shift the release run to a single Fargate task  ✅ recommended first

Containerize the **existing** release pipeline and run it as one Fargate task. **No DAG
decomposition.** This reuses all tested generator code and removes EC2 from generation with
minimal change — the same lift that `cmd/video-renderer` represents for video.

Why one task (not per-stage): the DAG is a linear chain, so decomposing into ~13 Step
Functions states buys little parallelism while adding S3 round-trips, per-transition cost, and
significant refactoring. Keep the in-process orchestrator; just move the process to Fargate.

**Work items:**

1. **`cmd/content-runner` (new one-shot binary)** — thin `main` that reads env
   `OWNER, REPO, RELEASE_TAG, CONTENT_BUCKET` and invokes the existing
   `releasepipeline.Pipeline.Run(...)` with the S3 `ArtifactStore`/`Publisher` already wired in
   `cmd/worker/main.go`. Essentially the worker's `handleReleaseMessage` body, minus SQS.
   Exit non-zero on hard failure; the per-stage fault tolerance stays as-is.
2. **`containers/content-runner/Dockerfile`** — multi-stage Go build → slim arm64 runtime.
   Include whatever the ReleaseContext build needs (git, or rely on the GitHub API path).
3. **`infrastructure/content.yaml` (new stack)** — mirrors `video.yaml`:
   - ECR repo `blog-gen-content-runner`; ECS cluster (reuse the video cluster if present);
     Fargate task def (arm64, size below); task role (S3 content RW, `bedrock:InvokeModel`,
     Secrets Manager read for the GitHub + Anthropic keys), execution role (ECR pull + logs).
   - `GenerateContentStateMachine` (Standard): `BuildAndGenerate` (`ecs:runTask.sync`) →
     `StartExecution` `blog-gen-video` → `PublishManifest` (λ). Per-branch `Catch`.
4. **Repoint the trigger** — `manual-trigger` (λ) starts `GenerateContent` instead of
   `blog-gen-orchestration`. `blog-gen-orchestration`, SQS `blog-gen-events`, and the
   `sfn`-trigger decorator in `cmd/worker` are retired for the release path.
5. **`deploy.yml`** — build/push the content-runner image (buildx arm64) + deploy
   `content.yaml`, gated behind e.g. `ENABLE_SERVERLESS_CONTENT=true` during rollout.
6. **Fargate sizing** — start **2 vCPU / 4 GB** (`bedrock`/Anthropic calls are I/O-bound; RAM
   holds the in-flight suite). Bump to 8 GB only if a large repo's ReleaseContext needs it.
   No Lambda 15-min ceiling to worry about — that's the point of Fargate.

### Phase B — Optional per-stage decomposition (later, only if warranted)

If per-stage retry/observability or the `architecture ∥ chain` parallelism becomes valuable,
decompose using the **existing** `Only` + `Store`/`Reuse` primitives: each Step Functions state
runs `content-runner --only <stage>`, reads its dependencies from S3, writes its artifact to S3.
Model the DAG in ASL (chain + the two parallel branches). This is **strictly optional** and much
larger; Phase A already achieves the EC2-offload goal.

## What the EC2 box becomes

- Runs **only** the `docker-compose` publishing plane (n8n + PostgreSQL + Redis). The
  `blog-gen-worker` service and its release path are removed from the userdata.
- **Downsize `INSTANCE_TYPE` → `t4g.medium`** (2 vCPU / 4 GB) — comfortable for n8n+PG+Redis,
  well clear of OOM. `t4g.small` (2 GB) is defensible only if n8n usage is genuinely light.
  **Do this only AFTER Phase A ships** (until then the worker still needs `t4g.large`).
- Keep the `/data` EBS volume (n8n execution history + PG grow over time); current 100 GB is
  ample. Power schedules/idle-stop can stay or relax (the box is now cheap either way).

## Artifact contracts (unchanged)

Reuse the existing S3 layout so nothing downstream changes:
`generated-content/<owner>/<name>/releases/<tag>/{.artifacts/<stage>.json, <stage>.md}` plus
`metadata.json` and `architecture-diagram.svg`. The video SM already reads these keys; Fargate
just writes them from a different host. `internal/artifactstore` + `internal/publish` are reused
verbatim.

## Risks & tradeoffs

| Risk | Mitigation |
|---|---|
| ReleaseContext build needs git/tools/creds in the container | Package git in the image (or use the API path); mount GitHub creds from Secrets Manager (task role), same as the worker uses today. |
| Fargate cold start per run (~30–60s image pull) | Acceptable for a minutes-long batch job; keep the image slim; reuse across runs. |
| Per-transition / task cost vs. one warm process | One Fargate task per release is cheap and only runs on demand — cheaper overall than a box sized for generation sitting powered-on. |
| Behavioural drift vs. the worker | Phase A runs the **same** `releasepipeline`/`contentsuite` code — no generation logic changes; add an e2e test that diffs a Fargate run against a known-good local run. |
| Two things now trigger video (worker decorator vs. new SM) | Remove the worker decorator; the new `GenerateContent` SM owns the `StartExecution` to `blog-gen-video`. |

## Rollout sequence

1. Land `cmd/content-runner` + Dockerfile + `content.yaml` behind `ENABLE_SERVERLESS_CONTENT`
   (stacks + image build, no traffic yet). cfn-lint + `go build/vet/test` clean.
2. Deploy the stack; run a **manual** `GenerateContent` execution for a known release; diff the
   S3 output against the current worker output for the same tag (e.g. v0.6.0).
3. Repoint `manual-trigger` to `GenerateContent`; run `/process` end-to-end; confirm artifacts +
   video + manifest.
4. Retire `blog-gen-orchestration`, SQS `blog-gen-events`, the worker release path + decorator.
5. Strip the worker from compute userdata; **switch `INSTANCE_TYPE` → `t4g.medium`**; redeploy.
6. Delete dead code/config (worker release wiring, `VIDEO_STATE_MACHINE_ARN` on the box, etc.).

## Verification

- `cfn-lint` on `content.yaml`; `go build ./... && go vet ./... && go test ./...`.
- `docker build containers/content-runner` succeeds; a Fargate run produces the full 24-object
  suite for a test release and matches the worker's output.
- End-to-end `/process` → `GenerateContent` → video → manifest, with artifacts in S3.

## Out of scope (for this migration)

- Rehoming n8n/PostgreSQL/Redis off EC2 (managed services) — separate decision; EC2 stays as
  the publishing plane per the current direction.
- Phase B per-stage decomposition (optional future optimization).
- Changing any generator's prompts/output — this is a **compute relocation**, not a content change.
```
