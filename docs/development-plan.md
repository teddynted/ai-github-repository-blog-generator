# Development Plan

This document tracks the incremental implementation of the MVP against the
[Requirements](./requirements.md) and [Roadmap](./roadmap.md). The documentation
is the single source of truth; this plan records **how** the MVP is being built
and **what is implemented so far**.

Principles: documentation-first, AWS-first, event-driven, serverless where
appropriate, cost-optimised, secure by design, Clean Architecture / SOLID,
small PR-sized changes, Infrastructure as Code.

---

## Implementation Decisions

These resolve ambiguities in the docs; they are deliberate and documented here
so behaviour is never undocumented.

| Decision | Choice | Rationale |
| --- | --- | --- |
| **IaC tool** | **AWS CloudFormation** (no Terraform) | Mandated throughout the docs; the platform is AWS-first. |
| **Go module layout** | **Single monorepo module** | One `go.mod` at the root; shared code in `internal/`; each Lambda is `lambdas/<fn>/` built with `go build ./lambdas/<fn>`. Maximises reuse (config, logging, signature, GitHub client) and suits Clean Architecture. |
| **Lambda runtime** | `provided.al2023`, **Linux/arm64** (Graviton) | Documented; best price/performance. |
| **Registration auth (MVP)** | **API Gateway API key** (`x-api-key`) | Simplest way to satisfy REG-9 "access-controlled"; revisited if GitHub App onboarding lands. |
| **n8n invocation** | EventBridge → SQS buffer + `instance-starter`; n8n invoked once healthy | Preserves cold-start durability (hybrid model). |

---

## Milestones

Each milestone compiles, is independently testable, and ships as a small PR.

| # | Milestone | Status |
| --- | --- | --- |
| 1 | **Project foundation** — module, config, logging, errors, bootstrap, build tooling | ✅ Implemented |
| 2 | **Infrastructure as Code** — CloudFormation stacks (network, serverless, compute, observability) | ✅ Implemented |
| 3 | **Repository registration** — validate repo + PAT, create webhook, store metadata + secret | ✅ Implemented |
| 4 | **Webhook receiver** — signature verification + commit-message trigger validation | ✅ Implemented |
| 5 | **Event processing** — matched event → EventBridge → SQS + Spot start (n8n stubbed) | ✅ Implemented |
| 6 | **Infrastructure lifecycle** — Spot start, health/readiness, n8n invoke, idle shutdown, retries | ✅ Implemented |
| 7 | **Repository processing** — placeholder clone / README / docs / commit retrieval | ✅ Implemented |

Out of scope for the MVP (see [Roadmap](./roadmap.md)): GitHub Apps, multi-user,
SaaS dashboard, billing, analytics, advanced AI agents, multi-platform
publishing, enterprise functionality.

---

## Milestone 1 — Project Foundation ✅

A single Go module with the shared building blocks every Lambda depends on.

| Package | Responsibility |
| --- | --- |
| `internal/config` | Environment-driven configuration with defaults, validation, and a `Require` check. Injectable `Getenv` for testability. |
| `internal/logging` | Structured JSON logging (`log/slog`), run-ID correlation, and secret redaction. |
| `internal/apperror` | Typed errors with stable codes and HTTP-status mapping; composes with `errors.Is/As`. |
| `internal/app` | Bootstrap container wiring config + logger for a Lambda entry point. |

Tooling: `Makefile` (`fmt`, `vet`, `test`, `check`, `build`, `tidy`), `.gitignore`,
`.env.example`. Build/test:

```bash
make check          # gofmt check + go vet + go test -race -cover
make build          # compile each implemented Lambda to dist/<fn>/bootstrap (Linux/arm64)
```

No external dependencies yet — the foundation is standard-library only. AWS SDK
and `aws-lambda-go` arrive with Milestones 3–4.

---

## Milestone 2 — Infrastructure as Code ✅

Four modular, `cfn-lint`-clean CloudFormation templates under `infrastructure/`.
Deploy order: **network → serverless → compute → observability**.

| Stack | Provisions |
| --- | --- |
| `network.yaml` | VPC, public subnet, Internet Gateway, route table, instance security group (SSH from operator CIDR only) |
| `serverless.yaml` | REST API Gateway (`/webhook` open, `/repositories` API-key), 4 Lambdas, per-function IAM roles, EventBridge bus + matched-event rule + idle timer, SQS events queue + DLQ, DynamoDB metadata table |
| `compute.yaml` | EC2 Spot launch template + instance, persistent gp3 EBS volume (retained), instance IAM role/profile, base-host user data |
| `observability.yaml` | CloudWatch log groups (bounded retention), SNS alarm topic, failure alarms, ops dashboard |

Validate: `make lint-cfn` (runs `cfn-lint infrastructure/*.yaml`).

**Implementation notes**

- **Spot via Launch Template.** CloudFormation's `AWS::EC2::Instance` does not
  accept `InstanceMarketOptions`; Spot options are declared on an
  `AWS::EC2::LaunchTemplate` (`SpotInstanceType: persistent`,
  `InstanceInterruptionBehavior: stop`) that the instance references. This
  matches the start/stop cost model — a persistent Spot instance can be stopped
  by `idle-shutdown` and restarted by `instance-starter`.
- **Tag-scoped start/stop.** The starter/idle Lambdas are created before the
  instance, so their IAM grants `ec2:Start/StopInstances` conditioned on
  `aws:ResourceTag/Project`, and they resolve the instance by tag rather than a
  hard-coded ID. This avoids a stack dependency cycle (serverless ⇄ compute).
- **Lambda code.** The templates reference deployment packages in an artifacts
  S3 bucket (`ArtifactsBucket` + `*CodeKey` parameters). The function code is
  implemented in Milestones 3–6; until then the stacks validate but the Lambdas
  are not yet deployable with real behaviour.

---

## Milestone 3 — Repository Registration ✅

The `registration` Lambda, built with Clean Architecture: a pure use case with
small ports, plus thin AWS/GitHub adapters. Introduces `aws-lambda-go` and the
AWS SDK v2 (which raise the module's minimum Go to 1.25; the build toolchain is
pinned in `go.mod` to a patched release).

| Package | Responsibility |
| --- | --- |
| `internal/repo` | Repository domain model + `ParseRepositoryURL` (HTTPS + SSH forms). |
| `internal/github` | Minimal GitHub REST client (`GetRepository`, `CreateWebhook`) over `net/http`, status→typed-error mapping. |
| `internal/registration` | The onboarding use case + ports (`GitHub`, `SecretStore`, `MetadataStore`) and the JSON handler. |
| `internal/secrets` | Secrets Manager adapter — stores PAT + webhook secret, returns only a reference. |
| `internal/metadata` | DynamoDB adapter — persists repository metadata (never the PAT). |
| `lambdas/registration` | API Gateway entry point wiring the collaborators. |

Onboarding order guarantees no half-registered repo is visible: validate access
→ create webhook → store credentials → write metadata. The PAT is never logged
and never written to DynamoDB (guarded by a test). All logic is unit-tested with
fakes/`httptest`; the Lambda cross-compiles to `linux/arm64`.

**Decision:** the registration endpoint is protected by an **API Gateway API
key** (`x-api-key`); the key ID is a stack output and its value is retrieved
from API Gateway after deploy.

---

## Milestone 4 — Webhook Receiver ✅

The `webhook-handler` Lambda — deliberately lightweight, no cloning/analysis/AI,
and it never reads the PAT.

| Package | Responsibility |
| --- | --- |
| `internal/trigger` | Commit-message trigger matching (prefix, default `blog:`). 100% covered. |
| `internal/githubsig` | HMAC-SHA256 signature verify/sign, constant-time; GitHub known-answer test. |
| `internal/webhook` | The handler: parse payload → resolve metadata → verify signature → evaluate trigger → publish (via port). |
| `internal/eventbus` | `LogPublisher` — a placeholder Publisher for M4 (real EventBridge adapter in M5). |
| `lambdas/webhook-handler` | API Gateway entry point (handles base64 bodies). |
| `internal/metadata`, `internal/secrets` | Extended with `Get` and `WebhookSecret` read paths. |

Request flow and outcomes: invalid signature → `401`; unregistered repo → `404`;
non-`push`/disabled/no-match → `200 ignored`; matched `blog:` commit → publish
+ `200 accepted`. The per-repository **Trigger Pattern** from metadata is used
(falling back to the platform default). Every branch is unit-tested with fakes.

**Placeholder:** on a match the handler calls the `Publisher` port, which is
wired to `LogPublisher` for now. Milestone 5 swaps in the EventBridge adapter
that publishes `blog.publish.requested` and starts the Spot instance.

---

## Milestone 5 — Event Processing ✅

The matched event now flows all the way to compute:
**webhook-handler → EventBridge → (SQS buffer + instance-starter → EC2 Spot start)**.

| Package | Responsibility |
| --- | --- |
| `internal/eventbus` | `EventBridgePublisher` — `PutEvents` of `blog.publish.requested`; swapped into the webhook handler in place of `LogPublisher`. |
| `internal/lifecycle` | `Starter.EnsureRunning` — start the instance unless already running (idempotent), located by Project tag. |
| `internal/awsec2` | EC2 adapter (`DescribeInstances` by tag, `StartInstances`). |
| `lambdas/instance-starter` | EventBridge-invoked Lambda that ensures the Spot host is running. |

Config gains `PROJECT_NAME` and `EVENT_SOURCE`; the serverless template passes
`EVENT_SOURCE=<project>.webhook` to the handler (matching the rule pattern).

**Stub:** the instance-starter ensures the host is running but does **not** yet
invoke the n8n workflow — that (and readiness detection + idle shutdown) is
Milestone 6. The event payload is available to the starter for that step.

---

## Milestone 6 — Infrastructure Lifecycle ✅

Completes the cost-optimised compute loop with automatic shutdown.

| Package | Responsibility |
| --- | --- |
| `internal/lifecycle` | `Shutdowner.StopIfIdle` — stop the instance once it has been up beyond the idle timeout **and** the queue is drained (visible + in-flight == 0). Idempotent. `EC2` port extended with `StopInstance` and an `Instance` value (id/state/launch time). |
| `internal/awssqs` | SQS adapter reporting queue depth (visible + not-visible). |
| `internal/awsec2` | Extended with `StopInstances` and launch-time. |
| `lambdas/idle-shutdown` | Scheduled Lambda (EventBridge idle timer) that runs `StopIfIdle`. |

**Readiness & n8n invocation (design decision).** In the hybrid model, n8n
**pulls** work from SQS (its SQS-trigger workflow) rather than being pushed an
HTTP call. So there is no Lambda-side n8n invoke or readiness probe: EventBridge
buffers the matched event in SQS, the instance-starter brings the host up, and
n8n drains SQS via long-polling once its container is healthy. This keeps n8n
unexposed (no inbound) and needs no Lambda-in-VPC networking. The n8n
SQS-trigger workflow + Docker Compose bring-up live on the instance and are part
of the instance configuration (`instance/`, Milestone 7).

**Retries.** Run retries come free from SQS visibility timeout + DLQ; the
scheduled idle check and instance start/stop are idempotent, so EventBridge/
Lambda retries are safe.

All four Lambdas now build; `make check` (`-race`) is green.

---

## Milestone 7 — Repository Processing ✅

The processing seam where Repository Intelligence will plug in — **placeholders
only**, no analysis or generation.

| Package / file | Responsibility |
| --- | --- |
| `internal/processing` | Ports (`Cloner`, `ReadmeRetriever`, `DocsRetriever`, `CommitRetriever`) + `Processor` that gathers a `Snapshot` (clone → README → docs → commits). |
| `internal/processing` (placeholder.go) | Canned implementations returning representative data; `NewPlaceholderProcessor` wires them. |
| `instance/docker-compose.yml` | Instance runtime: n8n + Ollama (state/models on `/data` = the EBS volume). OpenClaw is a documented placeholder service. |

The `Snapshot` is the fixed input contract for Repository Intelligence: swapping
the placeholders for real (e.g. OpenClaw-backed) retrievers requires **no change**
to the `Processor` or its ports. Every path is unit-tested.

**Out of scope (future roadmap):** Repository Intelligence (analysis), Repository
Memory population, AI generation, quality review, publishing, and multi-platform
output — all deliberately deferred, with the seams in place.

---

## MVP status

Milestones 1–7 are complete: an opt-in, event-driven, cost-optimised vertical
slice from repository registration through the commit-trigger gate, event
routing, on-demand Spot compute, automatic shutdown, and the processing seam —
all AWS-native, IaC-provisioned, and unit-tested. The next phase is Repository
Intelligence and content generation on top of the `processing.Snapshot`.

## Testing

Every package is unit-tested (ports exercised with fakes; HTTP clients with
`httptest`; git operations against real local repos via go-git). An **end-to-end
integration test** (`internal/pipeline`) wires the real adapters — filesystem
retrieval, analysis, quality review, file publish, and Repository Memory —
through the pipeline with only the LLM stubbed, and verifies the memory-based
skip on a repeat commit. Run with `make check` (`-race`).

## Continuous integration

GitHub Actions under `.github/workflows/` run on every push/PR and are green
without repository secrets:

- `go.yml` — gofmt, `go vet`, `go test -race -cover`, and cross-compiles all
  four Lambdas for `linux/arm64`.
- `cloudformation.yml` — `cfn-lint` on all templates.
- `security.yml` — `govulncheck` + `gosec` (SAST, high-severity gate) +
  `gitleaks` (secret scan with a placeholder allowlist); `checkov` runs
  informationally on the CloudFormation.
- `deploy.yml` — on merge to `main`, packages the Lambdas (SHA-versioned keys)
  and deploys network → serverless → compute → observability via OIDC. **Opt-in**:
  dormant until `DEPLOY_ENABLED=true` and the AWS role/vars are set
  (see [CI/CD → Enabling deploy.yml](./ci-cd.md#enabling-deployyml)).

CI/CD is now complete end to end; deploying just needs the AWS side configured.

### Deployment enablement

`infrastructure/bootstrap.yaml` + `scripts/bootstrap.sh` provision, once with
admin credentials, the pieces the deploy path needs: the **Lambda artifacts S3
bucket**, the **GitHub Actions OIDC provider** (optional), and the **deploy
role** the workflow assumes. The script prints the exact repository
secrets/variables to set (`AWS_DEPLOY_ROLE_ARN`, `ARTIFACTS_BUCKET`, …,
`DEPLOY_ENABLED=true`); after that, a merge to `main` deploys the app stacks.
Runbook: [Deployment → First-Time Bootstrap](./deployment.md#first-time-bootstrap-automated-deploy).

**Remaining to actually run in AWS (your side):** an account, a GPU Spot
instance type, and an EC2 key pair — then bootstrap → set the vars → deploy →
register a repo → push a `blog:` commit.

---

## Hardening (correctness vs. requirements)

- **Retry with backoff (FR-6.1).** `internal/retry` — bounded exponential
  backoff + jitter, retrying only transient (`upstream`/`unavailable`) errors and
  respecting context cancellation. Wired into the Ollama and GitHub clients
  (401/403/404 still fail fast); AWS SDK calls rely on the SDK's own retry.
- **Prompt/context budget (AI-6).** `generation.Generator.MaxPromptBytes`
  (default 24 KB) deterministically truncates the repository context so a large
  repo cannot overflow the model window; instructions and the closing directive
  are preserved, and a truncation marker is appended.
- **CloudWatch custom metrics (MON-2).** `internal/metrics` emits
  Embedded-Metric-Format (EMF) records to stdout — no `PutMetricData`, no extra
  IAM, no latency. The webhook handler emits `WebhookReceived` / `WebhookRejected`
  / `WebhookIgnored` / `TriggerMatched`; the worker emits `RunsStarted` /
  `RunsSucceeded` / `RunsFailed` / `RunsHeld` / `RunsSkipped` / `AssetsGenerated`.
  The `observability.yaml` dashboard gained two `BlogGenerator`-namespace widgets
  and a `RunsFailed` alarm.
- **Deeper analysis (FR-2.3–2.5).** `reposource.FSAnalyzer` detects the
  technical profile from the working copy — languages, dependency managers, IaC
  (Terraform, CloudFormation), containers (Docker/Compose), and CI/CD (GitHub
  Actions, GitLab CI, …). It is added to the `processing.Snapshot` and rendered
  into every generation prompt as a "Technical profile" section, so content is
  grounded in the real stack rather than just the README.

## Phase 2 (post-MVP) — Content Generation

Beyond the MVP slice, building on `processing.Snapshot`. In progress.

| Package | Responsibility | Status |
| --- | --- | --- |
| `internal/ollama` | Local LLM client (`/api/generate`, non-streaming) — realises "local inference via Ollama" with real, testable code. No external/paid API. | ✅ Implemented |
| `internal/generation` | `Generator.Generate`/`GenerateAll` — build deterministic prompts from a `Snapshot` and produce Markdown per kind via the `Model` port (Ollama satisfies it). Kinds: blog, README improvements, documentation, architecture summary, release notes. `GenerateAll` continues past a per-kind failure and aggregates errors. | ✅ Implemented |

**Scope note.** Five content outputs are implemented. Repository Memory
population, quality review, optional human approval, and publishing are still
future work — the ports keep them additive, and `GenerateAll` never lets one
failing kind corrupt the rest of the package.

| `internal/pipeline` | `Pipeline.Run` — composes process → generate → publish for a request. Resilient: publishes the assets that succeeded even on a partial generation failure. | ✅ Implemented |
| `internal/publish` | `FilePublisher` writes each asset to `<dir>/<owner>/<name>/<YYYY-MM-DD>/<kind>.md` (path-traversal guarded); `LogPublisher` remains for logging. The worker uses `FilePublisher` (`OUTPUT_DIR`, default `/data/generated-content` on the EBS volume). Remote destinations (Git / object store / CMS) can follow behind the same port. | ✅ Implemented |
| `internal/awssqs` | Extended with `Receive`/`Delete` (message consumption) alongside depth. | ✅ Implemented |
| `cmd/worker` | Instance worker: long-polls SQS → `Pipeline.Run` → deletes on success (leaves failures for SQS redelivery/DLQ). Message-handling logic is unit-tested. | ✅ Implemented |

**Runtime decision.** The MVP runtime is a **Go worker** (`cmd/worker`) draining
SQS and invoking `pipeline.Pipeline`. It was chosen over hand-authored n8n
workflow JSON because it is fully unit-testable and keeps the whole slice in one
verifiable codebase. The documented **n8n** SQS-trigger design remains a valid
alternative orchestration for the same seams (it would call the same
composition, e.g. via an exec node) and a future visual-orchestration option;
this worker does not remove that path. Build it with `make build-worker`
(Linux/amd64 for the g4dn host).

### Real repository processing ✅

`internal/reposource` replaces the placeholder processor with real
implementations of the processing ports:

| Type | Responsibility |
| --- | --- |
| `GitCloner` | Shallow, single-branch clone via **go-git** (no external git binary); token passed via `Auth`, never in the URL; per-repo work dir replaced each run. |
| `FSReadme` / `FSDocs` | Read the README and `docs/*.md` from the working copy (size- and count-capped). |
| `GitCommits` | Read recent commits (SHA, subject, author) from the clone. |
| `MetaTokenSource` | Resolve the repo's PAT: metadata `Get` → `secret_ref` → Secrets Manager `PAT`. |

The worker now wires this real processor (metadata + secrets clients,
`WORK_DIR` default `/data/work`), so a run clones the real repository, reads its
content, generates the package via Ollama, and writes Markdown files. Tested
with go-git against a real local repo (clone, README/docs retrieval, commit log)
and fakes for the token source.

### Repository Memory ✅

`internal/memory` is a filesystem-backed, per-repository record of published
commits (persisted under `MEMORY_DIR`, default `/data/memory` on the EBS volume
— not a managed DB, per the requirements; never stores secrets). The pipeline
now consults it: a matched event whose commit was **already published is skipped**
(`Result.Skipped`), and a completed run **records** the commit + kinds. Memory is
an optional port and a read failure never blocks a run. Tested (record → dedup,
per-repo isolation, idempotent per commit) plus pipeline behaviour (skip, record,
proceed-on-error).

### Quality review & optional human approval ✅

Two more pipeline stages, both optional ports:

| Package | Responsibility |
| --- | --- |
| `internal/review` | Rule-based `Reviewer`: each asset must be non-empty, meet a minimum length, and contain a Markdown heading. Only passing assets are published; failures are reported. A second local-model pass is a future enhancement behind the same shape. |
| `internal/approval` | `HoldForReview`: when `REQUIRE_HUMAN_APPROVAL=true`, generated content is stashed under `PENDING_DIR` (default `/data/pending`) and the gate returns not-approved, so the pipeline **holds** (`Result.Held`) without publishing or recording memory — a later approved run proceeds. |

Pipeline order is now **process → generate → review → approve → publish → record memory**. The worker always runs review and enables the approval gate from config. Tested: review pass/fail split, held-not-published-nor-recorded, approved-publishes.

### Notifications ✅

`internal/notify` delivers run outcomes (`published` / `held` / `failed`). The
worker notifies after each run: failure (message retained for SQS retry), held
(approval pending), or published. Channels behind the `Notifier` port:
`LogNotifier` (always on) and `WebhookNotifier` — a Slack-compatible (`text` +
structured fields) HTTP POST, retried on transient failure, enabled by setting
`NOTIFY_WEBHOOK_URL`. `Multi` fans out to both. Email is future.

**Functional MVP complete.** Every documented functional requirement and
pipeline stage now has real, tested code.

Publishing destinations behind the `Publisher` port: `FilePublisher` (default,
to `/data` on the EBS volume) and `S3Publisher` — set `OUTPUT_S3_BUCKET` to
publish to `s3://<bucket>/<prefix>/<owner>/<name>/<date>/<kind>.md` instead. The
compute stack grants the instance `s3:PutObject` on that bucket only when
`OutputS3Bucket` is set.

**Approvals dashboard.** When `REQUIRE_HUMAN_APPROVAL` holds content under
`PENDING_DIR`, the `approve` CLI (`cmd/approve`, built with the worker) is the
review queue: `approve` lists pending packages, `approve -all` auto-approves
(publishes) everything to the live destination and clears the queue, and
`approve -reject-all` discards. `internal/approval.PendingStore` is unit-tested.

**Still future (roadmap, not blocking the MVP).** OpenClaw-based deeper
analysis; an email notification channel; extra trigger sources (releases, tags,
PR labels); GitHub Apps auth; a **web** approvals UI; and the n8n workflow as an
alternative orchestration. Real deployment + end-to-end validation require an
AWS account and a GPU instance.
